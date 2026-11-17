package handoff

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historyprojection"
	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

type testBackend struct {
	usage       *hybrid.Usage
	mu          sync.Mutex
	records     map[string]hybrid.Record
	body        string
	promptErr   error
	text        string
	promptCalls int
	readWait    <-chan struct{}
}

func (b *testBackend) Record(id string) (hybrid.Record, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.records[id]
	if !ok {
		return r, hybrid.ErrNotFound
	}
	return r, nil
}
func (b *testBackend) Prompt(ctx context.Context, id string, in hybrid.PromptInput) (hybrid.PromptResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.promptCalls++
	b.text = in.Text
	return hybrid.PromptResult{TurnID: "turn-1"}, b.promptErr
}
func (b *testBackend) read(ctx context.Context, req historysource.Request) (historysource.Document, error) {
	b.mu.Lock()
	body, wait := b.body, b.readWait
	b.mu.Unlock()
	if wait != nil {
		select {
		case <-wait:
		case <-ctx.Done():
			return historysource.Document{}, ctx.Err()
		}
	}
	if req.Cursor != "" {
		var end int
		if _, err := fmt.Sscan(req.Cursor, &end); err != nil || end > len(body) {
			return historysource.Document{}, historysource.ErrChanged
		}
		body = body[:end]
	}
	return historysource.Document{Source: req.Source, Cursor: fmt.Sprint(len(body)), Generation: "1", Fingerprint: "inode-1", FormatVersion: "v1", ParserVersion: "v1", Records: []historysource.Record{{ID: "native-message-1", Kind: "prose", Role: "user", Text: body}}}, nil
}

type testConfig struct{}

func (testConfig) Get(string) (launch.Config, error) {
	return launch.Config{ID: "config", Cmd: []string{"test-agent"}, Hybrid: true, HybridSupport: agentprofile.Support{Supported: true, Agent: "test"}}, nil
}
func newTestService(t *testing.T) (*Service, *testBackend) {
	t.Helper()
	b := &testBackend{records: map[string]hybrid.Record{}, body: "历史内容\n  code()"}
	for _, id := range []string{"source", "target"} {
		b.records[id] = hybrid.Record{ID: id, ConfigID: "config", Agent: "test", AgentSessionID: id + "-native", HistoryRoot: "/native", Cwd: "/work", State: hybrid.StateActive, Side: hybrid.SideACP}
	}
	native := historyimport.New(historyimport.Options{Readers: map[string]historysource.ReadFunc{"test": b.read}, Discoverers: map[string]historyimport.DiscoverFunc{"test": func(context.Context, string) ([]historysource.Source, error) {
		return []historysource.Source{{Agent: "test", NativeID: "source-native", Path: "/native/source"}}, nil
	}}, Resolve: func(ctx context.Context, id string) (historyimport.Identity, error) {
		r, err := b.Record(id)
		return historyimport.Identity{Agent: r.Agent, NativeID: r.AgentSessionID, Root: r.HistoryRoot}, err
	}})
	projection := historyprojection.New(historyprojection.Options{Source: native, Preprocess: historyprojection.DefaultPreprocessConfig()})
	s := New(Options{Hybrid: b, Launch: testConfig{}, NativeHistory: native, HistoryProjection: projection})
	t.Cleanup(s.Close)
	return s, b
}
func previewTarget(t *testing.T, s *Service) Preview {
	t.Helper()
	p, err := s.Preview(context.Background(), "source", Request{TargetSessionID: "target", UserText: "当前任务\n\tkeep  spacing"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func awaitOperation(t *testing.T, s *Service, id string) Operation {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		o, err := s.Delivery(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if o.FinishedAt != nil {
			return o
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("operation did not finish")
	return Operation{}
}
func TestConcurrentPreviewConsumptionCallsPromptOnce(t *testing.T) {
	s, b := newTestService(t)
	p := previewTarget(t, s)
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
				t.Errorf("deliver %v", err)
			}
		})
	}
	wg.Wait()
	o := awaitOperation(t, s, p.OperationID)
	if o.State != "succeeded" {
		t.Fatalf("operation %+v", o)
	}
	if _, err := s.Deliver(context.Background(), p.ID, "different-operation"); !errors.Is(err, ErrConflict) {
		t.Fatalf("second owner: %v", err)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.promptCalls != 1 || b.text != p.Text || !strings.HasSuffix(b.text, "当前任务\n\tkeep  spacing") {
		t.Fatalf("calls=%d text=%q", b.promptCalls, b.text)
	}
}
func TestOperationPayloadConflict(t *testing.T) {
	s, _ := newTestService(t)
	first := previewTarget(t, s)
	second := previewTarget(t, s)
	if _, err := s.Deliver(context.Background(), first.ID, first.OperationID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Deliver(context.Background(), second.ID, first.OperationID); !errors.Is(err, ErrConflict) {
		t.Fatalf("payload conflict: %v", err)
	}
}
func TestUnknownPromptResultIsCached(t *testing.T) {
	s, b := newTestService(t)
	b.promptErr = context.DeadlineExceeded
	p := previewTarget(t, s)
	if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
		t.Fatal(err)
	}
	o := awaitOperation(t, s, p.OperationID)
	if o.State != "unknown" || o.Error != "可能已发送，请检查目标会话" {
		t.Fatalf("operation %+v", o)
	}
	for range 10 {
		if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
			t.Fatal(err)
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.promptCalls != 1 {
		t.Fatalf("prompt calls %d", b.promptCalls)
	}
}
func TestChangedSourceOrTargetPreventsSend(t *testing.T) {
	for _, change := range []string{"source-body", "source-identity", "target-identity"} {
		t.Run(change, func(t *testing.T) {
			s, b := newTestService(t)
			p := previewTarget(t, s)
			b.mu.Lock()
			switch change {
			case "source-body":
				b.body = "替换内容"
			case "source-identity":
				r := b.records["source"]
				r.AgentSessionID = "replacement"
				b.records["source"] = r
			case "target-identity":
				r := b.records["target"]
				r.AgentSessionID = "replacement"
				b.records["target"] = r
			}
			b.mu.Unlock()
			if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
				t.Fatal(err)
			}
			o := awaitOperation(t, s, p.OperationID)
			if o.State != "failed" {
				t.Fatalf("operation %+v", o)
			}
			b.mu.Lock()
			defer b.mu.Unlock()
			if b.promptCalls != 0 {
				t.Fatal("changed preview was sent")
			}
		})
	}
}
func TestAppendingNativeHistoryPreservesPreview(t *testing.T) {
	s, b := newTestService(t)
	p := previewTarget(t, s)
	b.mu.Lock()
	b.body += "\nnew suffix"
	b.mu.Unlock()
	if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
		t.Fatal(err)
	}
	o := awaitOperation(t, s, p.OperationID)
	if o.State != "succeeded" {
		t.Fatalf("operation %+v", o)
	}
}
func TestCancellationBeforeExternalCall(t *testing.T) {
	s, b := newTestService(t)
	p := previewTarget(t, s)
	b.mu.Lock()
	b.readWait = make(chan struct{})
	b.mu.Unlock()
	if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Cancel(context.Background(), p.OperationID); err != nil {
		t.Fatal(err)
	}
	o := awaitOperation(t, s, p.OperationID)
	if o.State != "failed" {
		t.Fatalf("operation %+v", o)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.promptCalls != 0 {
		t.Fatal("cancelled preparation sent a prompt")
	}
}
func TestExpiryAndOldInstanceHandles(t *testing.T) {
	s, _ := newTestService(t)
	p := previewTarget(t, s)
	other, _ := newTestService(t)
	if _, err := other.Deliver(context.Background(), p.ID, p.OperationID); !errors.Is(err, ErrExpired) {
		t.Fatalf("old instance: %v", err)
	}
	s.mu.Lock()
	s.now = func() time.Time { return p.ExpiresAt }
	s.mu.Unlock()
	if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); !errors.Is(err, ErrExpired) {
		t.Fatalf("expiry: %v", err)
	}
}
func TestOperationAndPreviewCapacity(t *testing.T) {
	s, _ := newTestService(t)
	for range maxPreviews {
		previewTarget(t, s)
	}
	if _, err := s.Preview(context.Background(), "source", Request{TargetSessionID: "target"}); !errors.Is(err, historysource.ErrLimit) && !errors.Is(err, ErrLimit) {
		t.Fatalf("full preview cache: %v", err)
	}
	s2, _ := newTestService(t)
	p := previewTarget(t, s2)
	s2.mu.Lock()
	for i := range maxOperations {
		s2.operations[fmt.Sprint(i)] = &operationEntry{operation: Operation{ID: fmt.Sprint(i), State: "sending"}}
	}
	s2.mu.Unlock()
	if _, err := s2.Deliver(context.Background(), p.ID, p.OperationID); !errors.Is(err, ErrLimit) {
		t.Fatalf("full result cache: %v", err)
	}
}
func TestCreationErrorPreservesResourcesAndSingleCall(t *testing.T) {
	s, _ := newTestService(t)
	var calls atomic.Int32
	s.options.Create = func(context.Context, sessioncreate.Input) (sessioncreate.Result, error) {
		calls.Add(1)
		return sessioncreate.Result{}, &sessioncreate.Error{Step: sessioncreate.StepSession, Location: &sessioncreate.Location{WorktreeID: "new-tree", Path: "/work/new-tree"}, Err: errors.New("agent startup failed")}
	}
	p, err := s.Preview(context.Background(), "source", Request{ConfigID: "config", Location: Location{Cwd: "/work"}, UserText: "task"})
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
			t.Fatal(err)
		}
	}
	o := awaitOperation(t, s, p.OperationID)
	if o.State != "failed" || o.Location == nil || o.Location.WorktreeID != "new-tree" || calls.Load() != 1 {
		t.Fatalf("operation %+v calls=%d", o, calls.Load())
	}
}

func (b *testBackend) ContextUsage(context.Context, string) (*hybrid.Usage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.usage == nil {
		return nil, nil
	}
	v := *b.usage
	return &v, nil
}
func TestProviderBudgetChangeRequiresPreview(t *testing.T) {
	s, b := newTestService(t)
	b.usage = &hybrid.Usage{Used: 100, Size: 200000}
	p := previewTarget(t, s)
	if p.Budget.Source != "provider" || p.Budget.NativeUsed != 100 || p.Budget.Window != 200000 {
		t.Fatalf("budget %+v", p.Budget)
	}
	b.mu.Lock()
	b.usage = &hybrid.Usage{Used: 500, Size: 200000}
	b.mu.Unlock()
	if _, err := s.Deliver(context.Background(), p.ID, p.OperationID); err != nil {
		t.Fatal(err)
	}
	o := awaitOperation(t, s, p.OperationID)
	if o.State != "failed" {
		t.Fatalf("operation %+v", o)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.promptCalls != 0 {
		t.Fatalf("calls=%d", b.promptCalls)
	}
}
