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

	snapcompact "github.com/mzdluo123/snapcompact-go"
	"github.com/rainchan/webmux/internal/historyprojection"
	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

func TestDefaultImagePreviewSendsFrozenPages(t *testing.T) {
	s, b := newTestService(t)
	task := "当前任务\n\tkeep  spacing\n"
	p, err := s.Preview(context.Background(), "source", Request{TargetSessionID: "target", UserText: task})
	if err != nil {
		t.Fatal(err)
	}
	if p.Format != "image" || p.Request.Format != "image" || p.Capability != "supported" || p.BlockedReason != "" || len(p.Pages) == 0 || p.ImageBudget.MaxPages != maxImagePages {
		t.Fatalf("preview format=%s capability=%s blocked=%q pages=%d budget=%+v", p.Format, p.Capability, p.BlockedReason, len(p.Pages), p.ImageBudget)
	}
	if strings.Contains(p.Text, "code()") || !strings.Contains(p.HistoryText, "code()") || !strings.Contains(p.Text, "snapshotId="+p.SnapshotID) || !strings.HasSuffix(p.Text, task) {
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

func TestImagePageBudgetReselectsCompleteSegments(t *testing.T) {
	s, b := newTestService(t)
	for i := range 4 {
		b.history = append(b.history, historysource.Record{ID: fmt.Sprintf("record-%d", i), Role: "user", Kind: "structured", Text: strings.Repeat("  code_line()\n", 20) + fmt.Sprintf("marker_%d\n", i)})
	}
	p, err := s.Preview(context.Background(), "source", Request{TargetSessionID: "target", MaxPages: 1})
	if err != nil {
		t.Fatal(err)
	}
	if p.BlockedReason != "" || len(p.Pages) != 1 || p.SelectedCount != 1 || p.OmittedCount != 3 || !strings.Contains(p.HistoryText, "marker_3") || strings.Count(p.HistoryText, "code_line()") != 20 {
		t.Fatalf("selection=%d omitted=%d pages=%d blocked=%q history=%q", p.SelectedCount, p.OmittedCount, len(p.Pages), p.BlockedReason, p.HistoryText)
	}
}

func TestImageFailureRequiresExplicitTextPreview(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Service, *testBackend)
	}{
		{"capability", func(_ *Service, b *testBackend) { b.image = false }},
		{"missing-glyph", func(_ *Service, b *testBackend) { b.body = "missing \U0010ffff" }},
		{"wire-limit", func(s *Service, _ *testBackend) { s.options.MaxPromptWireBytes = 2048 }},
		{"render-failure", func(s *Service, _ *testBackend) {
			cfg := snapcompact.DefaultRenderConfig()
			cfg.Width = -1
			s.options.HistoryProjection = historyprojection.New(historyprojection.Options{Preprocess: historyprojection.DefaultPreprocessConfig(), Render: cfg})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, b := newTestService(t)
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
			text, err := s.Preview(context.Background(), "source", Request{TargetSessionID: "target", Format: "text"})
			if err != nil || text.BlockedReason != "" || len(text.Pages) != 0 || text.Format != "text" {
				t.Fatalf("text preview blocked=%q err=%v", text.BlockedReason, err)
			}
			if _, err := s.Deliver(context.Background(), text.ID, text.OperationID); err != nil {
				t.Fatal(err)
			}
			if op := awaitOperation(t, s, text.OperationID); op.State != "succeeded" {
				t.Fatalf("operation %+v", op)
			}
			b.mu.Lock()
			defer b.mu.Unlock()
			if b.promptCalls != 1 || len(b.images) != 0 {
				t.Fatalf("calls=%d images=%d", b.promptCalls, len(b.images))
			}
		})
	}
}

func TestImageCapabilityRecheckedBeforeSend(t *testing.T) {
	s, b := newTestService(t)
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

func TestImageTextBudgetDoesNotCountHistoryBytes(t *testing.T) {
	s, b := newTestService(t)
	s.options.TokenCap = 1500
	b.body = strings.Repeat("history ", 500)
	p, err := s.Preview(context.Background(), "source", Request{TargetSessionID: "target"})
	if err != nil || p.BlockedReason != "" || p.Budget.Used >= int64(len(b.body)) || p.Budget.Used > p.Budget.History {
		t.Fatalf("budget=%+v blocked=%q err=%v", p.Budget, p.BlockedReason, err)
	}
}

func TestRenderCoverageUsesCompleteProjectionRanges(t *testing.T) {
	segments := []snapcompact.Segment{{SourceID: "s", Text: "中文x"}}
	page := func(ranges ...snapcompact.Range) snapcompact.RenderResult {
		return snapcompact.RenderResult{Pages: []snapcompact.Page{{Index: 0, Width: 1, Height: 1, PNG: []byte{1}, Ranges: ranges}}}
	}
	full := snapcompact.Range{SourceID: "s", Field: "projection", End: 3}
	if !renderCovered(segments, page(full)) {
		t.Fatal("complete rune coverage rejected")
	}
	tests := []snapcompact.RenderResult{
		{}, page(snapcompact.Range{SourceID: "s", Field: "projection"}),
		page(snapcompact.Range{SourceID: "s", Field: "text", End: 3}),
		page(snapcompact.Range{SourceID: "s", Field: "projection", Start: 1, End: 3}),
		page(snapcompact.Range{SourceID: "s", Field: "projection", End: 2}),
	}
	uncovered := page(full)
	uncovered.Uncovered = []snapcompact.Range{full}
	tests = append(tests, uncovered)
	for i, rendered := range tests {
		if renderCovered(segments, rendered) {
			t.Fatalf("incomplete coverage %d accepted", i)
		}
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
