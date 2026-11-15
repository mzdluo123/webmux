package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/historysource"
)

func TestSourceForgetRetainsTargetUnknownGateAndInlineProof(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := openTest(t, dir)
	b := bindTest(t, s, "source")
	_, e := executionTest(t, s, b)
	if err := s.BindRuntime(ctx, e.RuntimeID, e.TargetProviderThreadID, "config", "omp"); err != nil {
		t.Fatal(err)
	}
	text := "historical packet\nUser message:\noriginal"
	e.Phase = "sending"
	e.NativeThreadID = "native"
	e.PromptHash = TextHash(text)
	if err := s.CheckpointExecution(ctx, e); err != nil {
		t.Fatal(err)
	}
	if err := s.ClaimExecutionText(ctx, e.ID, e.PromptHash, e.PromptHash); err != nil {
		t.Fatal(err)
	}
	if err := s.ForgetRuntime(ctx, "source"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execution(ctx, e.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("source operation survived", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openTest(t, dir)
	gate, held, err := s.RuntimeDispatch(ctx, e.RuntimeID)
	if err != nil || !held || gate.ID != e.ID || gate.Status != "unknown" {
		t.Fatal("unknown gate lost", gate, held, err)
	}
	if _, matched, err := s.NormalizeInlineUser(ctx, e.TargetProviderThreadID, "other-native", text); err != nil || matched {
		t.Fatal("wrong identity admitted", err)
	}
	p, err := s.ReservePreview(ctx, PreviewReservation{Snapshots: controlSnapshots(t, s, e.TargetConversationID), SourceConversationID: e.TargetConversationID, ClientRequestID: "new-target-preview", RequestHash: "preview", Request: PreviewRequest{ConfigID: "config", AgentName: "omp", TargetProviderThreadID: e.TargetProviderThreadID}})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.CompletePreview(ctx, p.ID, PreviewResult{Budget: PreviewBudget{History: 16000}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.BeginExecution(ctx, ExecutionInput{SourceConversationID: e.TargetConversationID, HandoffID: p.ID, ClientRequestID: "bypass-unknown", RequestHash: "new", Revision: p.Revision}); !errors.Is(err, ErrConflict) {
		t.Fatal("new command bypassed forgotten-source unknown", err)
	}
	if _, matched, err := s.NormalizeInlineUser(ctx, e.TargetProviderThreadID, "native", text, "stable-user"); err != nil || !matched {
		t.Fatal("exact target proof failed", err)
	}
	if gate, held, err = s.RuntimeDispatch(ctx, e.RuntimeID); err != nil || held {
		t.Fatal("proof did not release gate", gate, held, err)
	}
}

func TestForgottenSourceNativeUnknownNeedsWholeProofButNeverSendsUserTask(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	_, _, e, items := injectionTest(t, s)
	if err := s.BindRuntime(ctx, e.RuntimeID, e.TargetProviderThreadID, "config", "omp"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReserveInjection(ctx, e.ID, "native", items); err != nil {
		t.Fatal(err)
	}
	if err := s.ClaimInjection(ctx, e.DeliveryID); err != nil {
		t.Fatal(err)
	}
	if err := s.ForgetRuntime(ctx, "source"); err != nil {
		t.Fatal(err)
	}
	if _, held, err := s.RuntimeDispatch(ctx, e.RuntimeID); err != nil || !held {
		t.Fatal("history unknown lost", held, err)
	}
	if err := s.ReconcileInjection(ctx, e.DeliveryID, "native", items[:1]); !errors.Is(err, ErrConflict) {
		t.Fatal("partial proof", err)
	}
	if err := s.ReconcileInjection(ctx, e.DeliveryID, "native", items); err != nil {
		t.Fatal(err)
	}
	gate, held, err := s.RuntimeDispatch(ctx, e.RuntimeID)
	if err != nil || held || gate.CallStarted {
		t.Fatal("proof did not cancel unsent deleted operation", gate, held, err)
	}
	if _, err := s.Execution(ctx, e.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted operation recreated", err)
	}
}

func TestPublishedManagedRuntimePrunesButReservationSurvives(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	b := bindTest(t, s, "source")
	_, e := executionTest(t, s, b)
	if err := s.BindRuntime(ctx, e.RuntimeID, e.TargetProviderThreadID, "config", "omp"); err != nil {
		t.Fatal(err)
	}
	if err := s.PruneRuntimes(ctx, []string{"source"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Binding(ctx, e.RuntimeID); err != nil {
		t.Fatal("unpublished reservation pruned", err)
	}
	if err := s.SyncRuntime(ctx, RuntimeInput{RuntimeID: e.RuntimeID, Cwd: "/workspace", ConfigID: "config", AgentName: "omp", Status: "closed"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PruneRuntimes(ctx, []string{"source"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Binding(ctx, e.RuntimeID); !errors.Is(err, ErrNotFound) {
		t.Fatal("published managed orphan retained", err)
	}
}

func TestForgettingUnpublishedRuntimeLeavesRecoverableDeletionIntent(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	b := bindTest(t, s, "source")
	_, e := executionTest(t, s, b)
	if err := s.BindRuntime(ctx, e.RuntimeID, e.TargetProviderThreadID, "config", "omp"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkRuntimeForgetting(ctx, e.RuntimeID); err != nil {
		t.Fatal(err)
	}
	// Simulate successful JSON removal and a crash before SQL deletion.
	if err := s.PruneRuntimes(ctx, []string{"source"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Binding(ctx, e.RuntimeID); !errors.Is(err, ErrNotFound) {
		t.Fatal("forget intent was confused with unpublished creation", err)
	}
}

func TestForgottenSourceRemovesUnexecutedForkDraftMetadata(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	b := bindTest(t, s, "source")
	p, _ := executionTest(t, s, b)
	if err := s.ForgetRuntime(ctx, "source"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Conversation(ctx, p.TargetConversationID); !errors.Is(err, ErrNotFound) {
		t.Fatal("abandoned fork metadata survived", err)
	}
}

func TestSQLiteReadOnlyAndFullAbortBeforeFrozenPublication(t *testing.T) {
	for _, fault := range []string{"read-only", "full"} {
		t.Run(fault, func(t *testing.T) {
			ctx := context.Background()
			s := openTest(t, t.TempDir())
			sources, _, source := nativeFixture(t, s, "source", strings.Repeat("private not logged ", 10000))
			statement := "PRAGMA query_only=ON"
			if fault == "full" {
				var pages int
				if err := s.db.QueryRow("PRAGMA page_count").Scan(&pages); err != nil {
					t.Fatal(err)
				}
				statement = fmt.Sprintf("PRAGMA max_page_count=%d", pages)
			}
			if _, err := s.db.Exec(statement); err != nil {
				t.Fatal(err)
			}
			_, err := sources.Freeze(ctx, source, "pending", true)
			if !errors.Is(err, ErrUnavailable) {
				t.Fatal("storage failure did not fail closed", err)
			}
			var count int
			if err := s.db.QueryRow("SELECT count(*) FROM frozen_snapshot").Scan(&count); err != nil || count != 0 {
				t.Fatal("partial frozen manifest published", count, err)
			}
		})
	}
}

func TestProcessCrashAfterClaimBeforeConsumptionCommitStaysUnknown(t *testing.T) {
	if dir := os.Getenv("WEBMUX_CONSUMPTION_COMMIT_CRASH"); dir != "" {
		s := Open(Options{StateDir: dir})
		if s.Unavailable() != nil {
			os.Exit(40)
		}
		b, err := s.Binding(context.Background(), "source")
		if err != nil {
			os.Exit(41)
		}
		r, err := s.Receipt(context.Background(), b.ConversationID, "message-command")
		if err != nil {
			os.Exit(41)
		}
		s.beforeCommit = func() error { os.Exit(42); return nil }
		_ = s.FinishExecution(context.Background(), r.ExecutionID, "inline", "")
		os.Exit(43)
	}
	ctx := context.Background()
	dir := t.TempDir()
	s := openTest(t, dir)
	b := bindTest(t, s, "source")
	_, e := executionTest(t, s, b)
	e.Phase = "sending"
	e.NativeThreadID = "native"
	e.PromptHash = TextHash("packet")
	if err := s.CheckpointExecution(ctx, e); err != nil {
		t.Fatal(err)
	}
	if err := s.ClaimExecutionText(ctx, e.ID, e.PromptHash, e.PromptHash); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestProcessCrashAfterClaimBeforeConsumptionCommitStaysUnknown$")
	cmd.Env = append(os.Environ(), "WEBMUX_CONSUMPTION_COMMIT_CRASH="+dir)
	var exit *exec.ExitError
	if err := cmd.Run(); !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatal("unexpected child crash", err)
	}
	s = openTest(t, dir)
	got, err := s.Execution(ctx, e.ID)
	if err != nil || got.Status != "unknown" {
		t.Fatal("uncommitted success became known", got, err)
	}
	if err := s.ClaimExecutionText(ctx, e.ID, e.PromptHash, e.PromptHash); !errors.Is(err, ErrConflict) {
		t.Fatal("claimed call repeated after crash", err)
	}
	receipt, err := s.Receipt(ctx, b.ConversationID, "message-command")
	if err != nil || receipt.ExecutionID != e.ID {
		t.Fatal("new attempt allocated", receipt, err)
	}
}

func TestForgottenSourceDispatchMigration(t *testing.T) {
	for _, version := range []int{10, 11} {
		for _, replaced := range []bool{false, true} {
			for _, state := range []string{"confirmed", "prompt-unknown", "native-pending", "native-unknown", "native-confirmed", "unsent", "failed"} {
				t.Run(fmt.Sprintf("v%d/replaced=%t/%s", version, replaced, state), func(t *testing.T) {
					ctx := context.Background()
					dir := t.TempDir()
					db, err := sql.Open("sqlite", filepath.Join(dir, "conversations.sqlite"))
					if err != nil {
						t.Fatal(err)
					}
					// Explicit pre-native layout. Source rows have already cascaded
					// away; target-owned receipts are the only surviving proof.
					legacyFrozenExec(t, db, schema+activitySchema+previewSchema+budgetSchema+executionSchema+attachmentSchema+inlineReceiptSchema+dispatchSchema+injectionSchema+selectionSchema+`
CREATE INDEX handoff_source_created ON context_handoff(source_conversation_id,created_at DESC);
CREATE INDEX receipt_execution ON command_receipt(execution_id);
PRAGMA user_version=10;
INSERT INTO conversation(id,cwd,history_origin,active_provider_thread_id,dispatch_execution_id,created_at,updated_at) VALUES('target','/workspace','unknown','historical','execution',1,1);
INSERT INTO provider_thread(id,conversation_id,config_id,agent_name,agent_session_id,status,created_at,updated_at) VALUES('historical','target','config','omp','native','active',1,1);
INSERT INTO runtime_binding VALUES('runtime','target','historical');`)
					current := Binding{ConversationID: "target", ProviderThreadID: "historical"}
					if replaced {
						legacyFrozenExec(t, db, `INSERT INTO provider_thread(id,conversation_id,config_id,agent_name,agent_session_id,status,created_at,updated_at) VALUES('replacement','target','config','omp','replacement-native','active',2,2);
UPDATE conversation SET active_provider_thread_id='replacement' WHERE id='target';
UPDATE runtime_binding SET provider_thread_id='replacement' WHERE runtime_id='runtime';`)
						current.ProviderThreadID = "replacement"
					}
					packet := "legacy envelope\nUser message:\noriginal"
					items := []InjectionItem{{ID: "legacy-item", Role: "user", TextHash: TextHash("legacy history")}}
					if state == "confirmed" || state == "prompt-unknown" {
						providerID := ""
						if state == "confirmed" {
							providerID = "confirmed-item"
						}
						legacyFrozenExec(t, db, `INSERT INTO inline_receipt VALUES('execution','historical','native',?,'original','[]',?)`, TextHash(packet), providerID)
					}
					if strings.HasPrefix(state, "native-") {
						injectionState := strings.TrimPrefix(state, "native-")
						if injectionState == "confirmed" {
							injectionState = "injected"
						}
						legacyFrozenExec(t, db, `INSERT INTO native_injection VALUES('delivery','historical','native',?,100,1,1)`, injectionState)
						legacyFrozenExec(t, db, `INSERT INTO native_injection_item VALUES('delivery',0,'legacy-item','user',?,'')`, items[0].TextHash)
					}
					if version == 11 {
						legacyFrozenExec(t, db, dispatchReceiptSchema)
						if replaced {
							// Shipped v11 could attach an unsent fact to the replacement.
							legacyFrozenExec(t, db, `INSERT INTO dispatch_receipt VALUES('execution','replacement','runtime','replacement-native','execution','cancelled','ready',0,0)`)
						}
					}
					if err := db.Close(); err != nil {
						t.Fatal(err)
					}
					s := openTest(t, dir)
					if err := s.Close(); err != nil {
						t.Fatal(err)
					}
					s = openTest(t, dir)
					gate, held, err := s.RuntimeDispatch(ctx, "runtime")
					wantHeld := state == "prompt-unknown" || state == "native-pending" || state == "native-unknown"
					historical := state == "confirmed" || wantHeld
					wantThread := current.ProviderThreadID
					if historical {
						wantThread = "historical"
					}
					if err != nil || held != wantHeld || gate.RuntimeID != "runtime" || gate.TargetProviderThreadID != wantThread {
						t.Fatal("legacy owner cannot resume exact target", gate, held, err)
					}
					if historical && gate.NativeThreadID != "native" {
						t.Fatal("receipt adopted replacement native identity", gate)
					}
					if state == "confirmed" && gate.Status != "inline" {
						t.Fatal("confirmed target became unknown", gate)
					}
					switch state {
					case "prompt-unknown":
						if _, matched, err := s.NormalizeInlineUser(ctx, "historical", "wrong-native", packet); err != nil || matched {
							t.Fatal("wrong proof", matched, err)
						}
						if replaced {
							if _, matched, err := s.NormalizeInlineUser(ctx, "replacement", "replacement-native", packet); err != nil || matched {
								t.Fatal("replacement writer reconciled historical prompt", matched, err)
							}
						}
						if _, matched, err := s.NormalizeInlineUser(ctx, "historical", "native", packet); err != nil || !matched {
							t.Fatal("legacy exact packet cannot reconcile", matched, err)
						}
					case "native-pending", "native-unknown":
						if err := s.ReconcileInjection(ctx, "delivery", "replacement-native", items); !errors.Is(err, ErrConflict) {
							t.Fatal("replacement writer reconciled historical injection", err)
						}
						if err := s.ReconcileInjection(ctx, "delivery", "native", items); err != nil {
							t.Fatal("legacy native proof cannot reconcile", err)
						}
					}
					if _, held, err := s.RuntimeDispatch(ctx, "runtime"); err != nil || held {
						t.Fatal("legacy proof did not release gate", held, err)
					}
					if _, err := s.Execution(ctx, "execution"); !errors.Is(err, ErrNotFound) {
						t.Fatal("migration recreated deleted source operation", err)
					}
					if binding, err := s.Binding(ctx, "runtime"); err != nil || binding != current {
						t.Fatal("repair rebound current writer to historical target", binding, err)
					}
				})
			}
		}
	}
}

func TestCaptureOptOutFollowsReplacementNativeIdentityAtomically(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := Open(Options{StateDir: dir, DisableCapture: true})
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Unavailable(); err != nil {
		t.Fatal(err)
	}
	first := bindTest(t, s, "runtime")
	s.beforeCommit = func() error { return errors.New("replacement commit failed") }
	if err := s.SyncRuntime(ctx, RuntimeInput{RuntimeID: "runtime", Cwd: "/workspace", AgentSessionID: "private-replacement"}); err == nil {
		t.Fatal("replacement published without its privacy marker")
	}
	s.beforeCommit = nil
	if binding, err := s.Binding(ctx, "runtime"); err != nil || binding != first {
		t.Fatal("failed replacement changed binding", binding, err)
	}
	if err := s.SyncRuntime(ctx, RuntimeInput{RuntimeID: "runtime", Cwd: "/workspace", AgentSessionID: "private-replacement"}); err != nil {
		t.Fatal(err)
	}
	replacement, err := s.Binding(ctx, "runtime")
	if err != nil || replacement.ProviderThreadID == first.ProviderThreadID {
		t.Fatal("native replacement did not allocate identity", replacement, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = Open(Options{StateDir: dir})
	if err := s.Unavailable(); err != nil {
		t.Fatal(err)
	}
	if err := s.SourceAllowed(ctx, historysource.Source{Agent: "codex", NativeID: "private-replacement"}); !errors.Is(err, ErrUnavailable) {
		t.Fatal("replacement opted-out history would be imported", err)
	}
	// Metadata reconciliation after re-enable must not erase the marker, even
	// when the replacement itself was first observed after the opt-out ended.
	if err := s.SyncRuntime(ctx, RuntimeInput{RuntimeID: "runtime", Cwd: "/workspace", AgentSessionID: "late-replacement"}); err != nil {
		t.Fatal(err)
	}
	late, err := s.Binding(ctx, "runtime")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SourceAllowed(ctx, historysource.Source{Agent: "codex", NativeID: "late-replacement"}); !errors.Is(err, ErrUnavailable) {
		t.Fatal("late replacement lost opt-out lineage", err)
	}
	if _, err := s.StartRun(ctx, late.ProviderThreadID, "new-public-turn"); err != nil {
		t.Fatal("re-enable blocked control lifecycle", err)
	}
}
