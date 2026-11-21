package handoff

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

func useLongImageHistory(b *testBackend) {
	b.body = "early constraint\n" + strings.Repeat("archive_line(x <= 7); ", 950) +
		"MIDDLE_ONLY_SENTINEL " + strings.Repeat("second_half(x != 9); ", 950) + "\nlatest correction\n  code()"
	target := b.records["target"]
	target.Config = map[string]string{"model": "openai-codex/gpt-6-astra"}
	b.records["target"] = target
}

func TestDefaultImagePreviewSendsFrozenPages(t *testing.T) {
	s, b := newTestService(t)
	useLongImageHistory(b)
	task := "当前任务\n\tkeep  spacing\n"
	p, err := s.Preview(context.Background(), "source", Request{TargetSessionID: "target", UserText: task})
	if err != nil {
		t.Fatal(err)
	}
	if p.Format != "image" || p.Request.Format != "image" || p.Capability != "supported" || p.BlockedReason != "" || len(p.Pages) == 0 || p.ImageBudget.MaxPages != maxImagePages {
		t.Fatalf("preview format=%s capability=%s blocked=%q pages=%d budget=%+v", p.Format, p.Capability, p.BlockedReason, len(p.Pages), p.ImageBudget)
	}
	if strings.Contains(p.Text, "early constraint") || strings.Contains(p.Text, "latest correction") || strings.Contains(p.Text, "MIDDLE_ONLY_SENTINEL") || !strings.Contains(p.HistoryText, "MIDDLE_ONLY_SENTINEL") || !strings.Contains(p.Text, "snapshotId="+p.SnapshotID) || !strings.HasSuffix(p.Text, task) || p.imageTextOffset <= 0 {
		t.Fatalf("text=%q history=%q", p.Text, p.HistoryText)
	}
	var total int
	for i, page := range p.Pages {
		data, err := base64.StdEncoding.DecodeString(page.Data)
		if err != nil {
			t.Fatal(err)
		}
		dimensions, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if page.Index != i || page.Bytes != len(data) || page.MimeType != "image/png" || page.SHA256 != hex.EncodeToString(sum[:]) || page.Width != dimensions.Width || page.Height != dimensions.Height {
			t.Fatalf("invalid page metadata at %d", i)
		}
		total += len(data)
	}
	if p.ImageBudget.Bytes != total || p.ImageBudget.Pages != len(p.Pages) {
		t.Fatalf("image budget %+v", p.ImageBudget)
	}
	original := p.Pages[0]
	p.Pages[0].Data = "caller mutation"
	frozen, err := s.Get(context.Background(), p.ID)
	if err != nil || frozen.Pages[0] != original {
		t.Fatalf("preview mutation changed frozen data: %v", err)
	}
	b.mu.Lock()
	b.body += "\n新增历史不会改变冻结图片"
	b.mu.Unlock()
	if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
		t.Fatal(err)
	}
	if op := awaitOperation(t, s, p.OperationID); op.State != "succeeded" {
		t.Fatalf("operation %+v", op)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.text != frozen.Text || len(b.images) != len(frozen.Pages) {
		t.Fatal("delivery differs from preview")
	}
	for i, image := range b.images {
		if image.Data != frozen.Pages[i].Data || image.MimeType != frozen.Pages[i].MimeType {
			t.Fatalf("sent page %d differs from preview", i)
		}
	}
}

func TestImagePageBudgetReportsOmissions(t *testing.T) {
	s, b := newTestService(t)
	useLongImageHistory(b)
	for i := range 6 {
		b.history = append(b.history, historysource.Record{ID: fmt.Sprintf("record-%d", i), Role: "user", Kind: "structured", Text: fmt.Sprintf("start_marker_%d\n", i) + strings.Repeat("  code_line()\n", 800) + fmt.Sprintf("end_marker_%d\n", i)})
	}
	p, err := s.Preview(context.Background(), "source", Request{TargetSessionID: "target", MaxPages: 1})
	if err != nil {
		t.Fatal(err)
	}
	if p.BlockedReason != "" || len(p.Pages) != 1 || p.OmittedCount == 0 || strings.Contains(p.Text, "marker_") || !strings.Contains(p.HistoryText, "end_marker_5") || p.ImageBudget.RetainedRunes >= p.ImageBudget.ProjectedRunes {
		t.Fatalf("selection=%d omitted=%d pages=%d blocked=%q budget=%+v", p.SelectedCount, p.OmittedCount, len(p.Pages), p.BlockedReason, p.ImageBudget)
	}
}

func TestImageFailureBlocksTextFallback(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Service, *testBackend)
	}{
		{"capability", func(_ *Service, b *testBackend) { b.image = false }},
		{"missing-glyph", func(_ *Service, b *testBackend) {
			b.body = strings.Replace(b.body, "MIDDLE_ONLY_SENTINEL", strings.Repeat("\U0010ffff", 5000), 1)
		}},
		{"wire-limit", func(s *Service, _ *testBackend) { s.options.MaxPromptWireBytes = 2048 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, b := newTestService(t)
			useLongImageHistory(b)
			tt.change(s, b)
			p, err := s.Preview(context.Background(), "source", Request{TargetSessionID: "target"})
			if err != nil {
				t.Fatal(err)
			}
			if p.BlockedReason == "" {
				t.Fatal("unsafe image preview was not blocked")
			}
			if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); !errors.Is(err, ErrInvalid) {
				t.Fatalf("blocked delivery: %v", err)
			}
			if _, err := s.Preview(context.Background(), "source", Request{TargetSessionID: "target", Format: "text"}); !errors.Is(err, ErrInvalid) {
				t.Fatalf("text format accepted: %v", err)
			}
			if b.promptCalls != 0 {
				t.Fatal("blocked preview sent a prompt")
			}
		})
	}
}

func TestImageCapabilityRecheckedBeforeSend(t *testing.T) {
	s, b := newTestService(t)
	useLongImageHistory(b)
	p, err := s.Preview(context.Background(), "source", Request{TargetSessionID: "target"})
	if err != nil || p.BlockedReason != "" {
		t.Fatalf("preview blocked=%q err=%v", p.BlockedReason, err)
	}
	b.mu.Lock()
	b.image = false
	b.mu.Unlock()
	if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
		t.Fatal(err)
	}
	if op := awaitOperation(t, s, p.OperationID); op.State != "failed" {
		t.Fatalf("operation %+v", op)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.promptCalls != 0 {
		t.Fatal("unsupported image was sent")
	}
}

func TestNewSessionImageCapabilityCheckedAfterCreation(t *testing.T) {
	for _, supported := range []bool{false, true} {
		t.Run(fmt.Sprint(supported), func(t *testing.T) {
			s, b := newTestService(t)
			useLongImageHistory(b)
			manager := session.NewManager(session.Options{})
			t.Cleanup(func() { manager.Shutdown(context.Background()) })
			created, err := manager.CreateShell(session.ShellInput{CreateInput: session.CreateInput{Cmd: []string{"test-agent"}, Cwd: t.TempDir()}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			id := created.Info().ID
			b.records[id] = hybrid.Record{ID: id, ConfigID: "config", Agent: "test", AgentSessionID: "new-native", State: hybrid.StateActive, Side: hybrid.SideACP}
			b.image = supported
			calls := 0
			s.options.Create = func(context.Context, sessioncreate.Input) (sessioncreate.Result, error) {
				calls++
				return sessioncreate.Result{Session: created}, nil
			}
			p, err := s.Preview(context.Background(), "source", Request{ConfigID: "config"})
			if err != nil || p.Capability != "pending" || p.BlockedReason != "" {
				t.Fatalf("capability=%q blocked=%q err=%v", p.Capability, p.BlockedReason, err)
			}
			if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
				t.Fatal(err)
			}
			op := awaitOperation(t, s, p.OperationID)
			b.mu.Lock()
			defer b.mu.Unlock()
			if calls != 1 || op.TargetSessionID != id || (supported && (op.State != "succeeded" || b.promptCalls != 1)) || (!supported && (op.State != "failed" || b.promptCalls != 0)) {
				t.Fatalf("operation=%+v creates=%d prompts=%d", op, calls, b.promptCalls)
			}
		})
	}
}

func TestImageTokensParticipateInHistoryBudget(t *testing.T) {
	s, b := newTestService(t)
	useLongImageHistory(b)
	s.options.TokenCap = 200
	p, err := s.Preview(context.Background(), "source", Request{TargetSessionID: "target", UserText: "preserve task"})
	if err != nil {
		t.Fatal(err)
	}
	if p.BlockedReason == "" || !strings.HasSuffix(p.Text, "preserve task") {
		t.Fatalf("budget=%+v blocked=%q", p.Budget, p.BlockedReason)
	}
	if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("over-budget archive accepted: %v", err)
	}
}

func TestShortArchiveSendsOnlyImageHistory(t *testing.T) {
	s, b := newTestService(t)
	b.body = "sentinel_short_history"
	p := previewTarget(t, s)
	if len(p.Pages) != 1 || strings.Contains(p.Text, b.body) || !strings.Contains(p.HistoryText, b.body) || p.ImageBudget.RetainedRunes != p.ImageBudget.ProjectedRunes {
		t.Fatalf("short history: pages=%d text=%q audit=%q", len(p.Pages), p.Text, p.HistoryText)
	}
	if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
		t.Fatal(err)
	}
	if op := awaitOperation(t, s, p.OperationID); op.State != "succeeded" || len(b.images) != 1 || b.images[0].Data != p.Pages[0].Data || b.text != p.Text {
		t.Fatalf("short archive operation: %+v", op)
	}
}

func TestImageRequestValidation(t *testing.T) {
	s, _ := newTestService(t)
	for _, request := range []Request{{Format: "unknown"}, {MaxPages: -1}, {MaxPages: maxImagePages + 1}} {
		request.TargetSessionID = "target"
		if _, err := s.Preview(context.Background(), "source", request); !errors.Is(err, ErrInvalid) {
			t.Fatalf("request=%+v err=%v", request, err)
		}
	}
}
