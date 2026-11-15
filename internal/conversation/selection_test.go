package conversation

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestCompletePreviewBulkSelectionPreservesFrozenScopeAndAtomicity(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	sources, source, _ := nativeFixture(t, s, "source", "first message", "second message")
	snapshots, err := sources.FreezeConversation(ctx, source.ConversationID, source.ConversationID, true)
	if err != nil || len(snapshots) != 1 || len(snapshots[0].Records) != 2 {
		t.Fatal(snapshots, err)
	}
	first, second := snapshots[0].Records[0].ID, snapshots[0].Records[1].ID
	p, err := s.ReservePreview(ctx, PreviewReservation{SourceConversationID: source.ConversationID, ClientRequestID: "preview-command", RequestHash: "preview-hash", Request: PreviewRequest{ConfigID: "config", AgentName: "omp", Fork: true}, Snapshots: snapshots})
	if err != nil {
		t.Fatal(err)
	}
	otherSources, other, _ := nativeFixture(t, s, "other", "another conversation")
	otherSnapshots, err := otherSources.FreezeConversation(ctx, other.ConversationID, other.ConversationID, true)
	if err != nil || len(otherSnapshots) != 1 || len(otherSnapshots[0].Records) != 1 {
		t.Fatal(otherSnapshots, err)
	}
	foreign := otherSnapshots[0].Records[0].ID
	for _, tc := range []struct {
		name, selected string
		want           error
	}{
		{"outside-frozen-range", "future-native-record", ErrConflict},
		{"foreign", foreign, ErrConflict},
		{"duplicate", first, ErrInvalid},
		{"empty", "", ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.CompletePreview(ctx, p.ID, PreviewResult{OmittedItemIDs: []string{first}, SelectedItemIDs: []string{tc.selected}})
			if !errors.Is(err, tc.want) {
				t.Fatalf("CompletePreview error=%v, want %v", err, tc.want)
			}
			saved, err := s.Preview(ctx, p.ID)
			if err != nil || saved.State != "preparing" || len(saved.SelectedItemIDs)+len(saved.OmittedItemIDs) != 0 {
				t.Fatalf("partial selection committed: %+v, %v", saved, err)
			}
			page, err := s.Selection(ctx, p.ID, 0, 100, "")
			if !errors.Is(err, ErrConflict) || page.SelectedCount != 0 || page.OmittedCount != 0 {
				t.Fatalf("partial selection counts committed: %+v, %v", page, err)
			}
		})
	}
	saved, err := s.CompletePreview(ctx, p.ID, PreviewResult{OmittedItemIDs: []string{first}, SelectedItemIDs: []string{second}, ProjectionPolicy: "native-test/v1", RepresentationHashes: map[string]string{second: snapshots[0].Records[1].Hash}})
	if err != nil || saved.State != "ready" || len(saved.SelectedItemIDs) != 1 || saved.SelectedItemIDs[0] != second || len(saved.OmittedItemIDs) != 1 || saved.OmittedItemIDs[0] != first {
		t.Fatalf("bulk selection lost classification: %+v, %v", saved, err)
	}
	saved, err = s.EditDraft(ctx, p.ID, saved.Revision, nil, new("task-only edit"), saved.Budget)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, "UPDATE conversation SET cwd='/moved' WHERE id=?", source.ConversationID); err != nil {
		t.Fatal(err)
	}
	selection, err := s.Selection(ctx, p.ID, 0, 100, "")
	if err != nil || selection.SelectedCount != 1 || selection.OmittedCount != 1 || selection.SourceLocation != saved.Request.SourceLocation {
		t.Fatalf("task edit changed selection or frozen source: %+v, %v", selection, err)
	}
	if _, err := s.EditDraft(ctx, p.ID, saved.Revision, new("manual context"), nil, saved.Budget); err != nil {
		t.Fatal(err)
	}
	page, err := s.Selection(ctx, p.ID, 0, 100, "")
	if err != nil || page.SelectedCount != 0 || page.OmittedCount != 2 || !page.Done {
		t.Fatalf("manual edit lost selection counts: %+v, %v", page, err)
	}
}

func TestVersionEightUpgradePreservesSelectionCounts(t *testing.T) {
	ctx := context.Background()
	legacy, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "v8.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	legacyFrozenExec(t, legacy, schema+activitySchema+previewSchema+budgetSchema+executionSchema+attachmentSchema+inlineReceiptSchema+dispatchSchema+injectionSchema)
	legacyFrozenExec(t, legacy, `
INSERT INTO conversation(id,cwd,history_origin,created_at,updated_at) VALUES('source','/source','complete',1,1);
INSERT INTO provider_thread(id,conversation_id,config_id,agent_name,status,created_at,updated_at) VALUES('thread','source','config','omp','active',1,1);
INSERT INTO context_handoff VALUES('preview','source','source','thread','full_thread_summary',1,1,1,1);
INSERT INTO handoff_preview(handoff_id,source_position,source_run_ordinal,history_origin,reconcile_required,request,budget,state) VALUES('preview',2,1,'complete',0,'{}','{}','ready');
INSERT INTO handoff_item(handoff_id,item_id,selected,position) VALUES('preview','first',0,1),('preview','second',1,2);
`)
	if err := migrate(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	var selected, omitted int
	if err := legacy.QueryRowContext(ctx, "SELECT selected_count,omitted_count FROM handoff_preview WHERE handoff_id='preview'").Scan(&selected, &omitted); err != nil || selected != 1 || omitted != 1 {
		t.Fatalf("selection migration lost counts: %d/%d, %v", selected, omitted, err)
	}
}
