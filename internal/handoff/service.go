package handoff

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
)

const operationLifetime = 15 * time.Minute
const maxPreviews = 32
const maxOperations = 256
const maxImagePages = 8

type historyBinding struct{ scopeID, identity string }

type operationEntry struct {
	operation Operation
	preview   *Preview
	cancel    context.CancelFunc
}
type Service struct {
	history    map[string]historyBinding
	options    Options
	mu         sync.Mutex
	previews   map[string]*Preview
	operations map[string]*operationEntry
	instance   string
	now        func() time.Time
	ctx        context.Context
	cancel     context.CancelFunc
	workers    sync.WaitGroup
	closed     bool
}

func New(o Options) *Service {
	if o.TokenCap == 0 {
		o.TokenCap = DefaultTokenCap
	}
	if o.MaxPromptBytes == 0 {
		o.MaxPromptBytes = hybrid.MaxPromptBytes
	}
	if o.MaxPromptWireBytes == 0 {
		o.MaxPromptWireBytes = hybrid.MaxPromptWireBytes
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{options: o, history: map[string]historyBinding{}, previews: map[string]*Preview{}, operations: map[string]*operationEntry{}, instance: rand.Text(), now: time.Now, ctx: ctx, cancel: cancel}
}
func (s *Service) Close() { s.mu.Lock(); s.closed = true; s.cancel(); s.mu.Unlock(); s.workers.Wait() }
func (s *Service) cleanLocked() {
	now := s.now()
	for key, binding := range s.history {
		if _, err := s.options.NativeHistory.Scope(binding.scopeID); err != nil {
			delete(s.history, key)
		}
	}
	for id, p := range s.previews {
		active := false
		for _, op := range s.operations {
			if op.preview == p && op.operation.FinishedAt == nil {
				active = true
				break
			}
		}
		if !now.Before(p.ExpiresAt) && !active {
			delete(s.previews, id)
			s.options.NativeHistory.ReleaseScope(p.SnapshotID)
		}
	}
	for id, e := range s.operations {
		if e.operation.FinishedAt != nil && !now.Before(e.operation.FinishedAt.Add(operationLifetime)) {
			delete(s.operations, id)
		}
	}
}
func configFingerprint(c launch.Config) (string, error) {
	b, err := json.Marshal(struct {
		Cmd    []string
		Env    map[string]string
		Hybrid bool
	}{c.Cmd, c.Env, c.Hybrid})
	if err != nil {
		return "", ErrInvalid
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
func runtimeIdentity(r hybrid.Record) string {
	return r.ID + "\x00" + r.Agent + "\x00" + r.AgentSessionID + "\x00" + r.HistoryRoot + "\x00" + r.Cwd
}
func clonePreview(p *Preview) Preview {
	out := *p
	out.Request.AgentConfig = maps.Clone(p.Request.AgentConfig)
	out.Pages = slices.Clone(p.Pages)
	return out
}
func (s *Service) Preview(ctx context.Context, source string, in Request) (Preview, error) {
	var empty Preview
	if s.options.Hybrid == nil || s.options.NativeHistory == nil || s.options.HistoryProjection == nil || s.options.Launch == nil {
		return empty, ErrUnavailable
	}
	if in.Format == "" {
		in.Format = "image"
	}
	if in.MaxPages == 0 {
		in.MaxPages = maxImagePages
	}
	if (in.Format != "image" && in.Format != "text") || in.MaxPages < 1 || in.MaxPages > maxImagePages {
		return empty, ErrInvalid
	}
	if !utf8.ValidString(in.UserText) || len(in.UserText) > int(s.options.MaxPromptBytes) || len(in.AgentConfig) > 64 {
		return empty, ErrInvalid
	}
	if in.TargetSessionID == "" && in.ConfigID == "" {
		return empty, ErrInvalid
	}
	if in.Location.NewWorktreeBranch != "" && (in.Location.RepositoryID == "" || in.Location.WorktreeID != "" || in.Location.Cwd != "") {
		return empty, ErrInvalid
	}
	metadata := len(in.ConfigID) + len(in.TargetSessionID) + len(in.Location.Cwd) + len(in.Location.RepositoryID) + len(in.Location.WorktreeID) + len(in.Location.NewWorktreeBranch) + len(in.Location.NewWorktreeBase)
	for key, value := range in.AgentConfig {
		metadata += len(key) + len(value)
	}
	if metadata > 8192 {
		return empty, ErrInvalid
	}
	rec, err := s.options.Hybrid.Record(source)
	if err != nil {
		return empty, err
	}
	if rec.AgentSessionID == "" {
		return empty, historysource.ErrIdentity
	}
	var targetIdentity string
	if in.TargetSessionID != "" {
		if len(in.AgentConfig) > 0 {
			return empty, ErrInvalid
		}
		target, err := s.options.Hybrid.Record(in.TargetSessionID)
		if err != nil {
			return empty, err
		}
		if target.State != hybrid.StateActive || target.Side != hybrid.SideACP {
			return empty, hybrid.ErrWrongSide
		}
		if in.ConfigID != "" && in.ConfigID != target.ConfigID {
			return empty, ErrConflict
		}
		in.ConfigID = target.ConfigID
		targetIdentity = hybrid.NativeIdentity(target)
	}
	config, err := s.options.Launch.Get(in.ConfigID)
	if err != nil {
		return empty, err
	}
	if !config.Hybrid || !config.HybridSupport.Supported {
		return empty, ErrInvalid
	}
	fingerprint, err := configFingerprint(config)
	if err != nil {
		return empty, err
	}
	scope, err := s.options.NativeHistory.FreezeRuntime(ctx, source)
	if err != nil {
		return empty, err
	}
	saved := false
	defer func() {
		if !saved {
			s.options.NativeHistory.ReleaseScope(scope.ID)
		}
	}()
	doc, err := s.options.NativeHistory.ScopeDocument(ctx, scope.ID)
	if err != nil {
		return empty, err
	}
	projection, err := s.options.HistoryProjection.ProjectDocument(ctx, doc, in.UserText, s.options.HistoryProjection.Config(), false)
	if err != nil {
		return empty, err
	}
	budget, err := s.targetBudget(ctx, in.TargetSessionID, in.UserText)
	if err != nil {
		return empty, err
	}
	header := fmt.Sprintf("Historical reference. Source agent=%q session=%q branch=%q cwd=%q. Frozen scope: %s. Source gaps: %d. Exact text: thread_read with view=activity snapshotId=%s.\n", doc.Source.Agent, doc.Source.NativeID, doc.Source.Branch, rec.Cwd, scope.ID, len(doc.Gaps), scope.ID)
	p, err := s.preparePreview(ctx, in, projection.Projection, header, budget)
	if err != nil {
		return empty, err
	}
	storage := len(p.Text) + len(p.HistoryText) + len(in.UserText) + metadata + 2048
	for _, page := range p.Pages {
		storage += len(page.Data) + len(page.SHA256) + len(page.MimeType) + 64
	}
	if err := s.options.NativeHistory.ReserveScopeBytes(scope.ID, storage); err != nil {
		return empty, err
	}
	in.AgentConfig = maps.Clone(in.AgentConfig)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanLocked()
	if s.closed {
		return empty, ErrUnavailable
	}
	if len(s.previews) >= maxPreviews {
		return empty, ErrLimit
	}
	now := s.now()
	id := s.instance + "_" + rand.Text()
	p.ID, p.InstanceID, p.OperationID = id, s.instance, s.instance+"_"+rand.Text()
	p.SourceSessionID, p.SnapshotID, p.Request = source, scope.ID, in
	p.CreatedAt, p.ExpiresAt, p.GapCount = now, scope.ExpiresAt, len(doc.Gaps)
	p.sourceIdentity, p.targetIdentity, p.configHash = runtimeIdentity(rec), targetIdentity, fingerprint
	s.previews[id] = p
	saved = true
	return clonePreview(p), nil
}
func (s *Service) Get(ctx context.Context, id string) (Preview, error) {
	if err := ctx.Err(); err != nil {
		return Preview{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanLocked()
	p := s.previews[id]
	if p == nil || !s.now().Before(p.ExpiresAt) {
		return Preview{}, ErrExpired
	}
	return clonePreview(p), nil
}
func (s *Service) Deliver(ctx context.Context, previewID, operationID string) (Operation, error) {
	if err := ctx.Err(); err != nil {
		return Operation{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanLocked()
	if s.closed {
		return Operation{}, ErrUnavailable
	}
	if existing := s.operations[operationID]; existing != nil {
		if existing.operation.PreviewID != previewID {
			return Operation{}, ErrConflict
		}
		return existing.operation, nil
	}
	p := s.previews[previewID]
	if p == nil || !s.now().Before(p.ExpiresAt) {
		return Operation{}, ErrExpired
	}
	if p.consumed || operationID != p.OperationID {
		return Operation{}, ErrConflict
	}
	if p.BlockedReason != "" {
		return Operation{}, fmt.Errorf("%w: %s", ErrInvalid, p.BlockedReason)
	}
	if len(s.operations) >= maxOperations {
		return Operation{}, ErrLimit
	}
	if err := s.options.NativeHistory.PinScope(p.SnapshotID); err != nil {
		return Operation{}, ErrExpired
	}
	p.consumed = true
	ctx, cancel := context.WithCancel(s.ctx)
	e := &operationEntry{operation: Operation{ID: operationID, PreviewID: p.ID, InstanceID: s.instance, State: "preparing", TargetSessionID: p.Request.TargetSessionID, StartedAt: s.now()}, preview: p, cancel: cancel}
	s.operations[operationID] = e
	s.workers.Add(1)
	go func() { defer s.workers.Done(); defer cancel(); s.send(ctx, e) }()
	return e.operation, nil
}
func (s *Service) Delivery(ctx context.Context, id string) (Operation, error) {
	if err := ctx.Err(); err != nil {
		return Operation{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanLocked()
	e := s.operations[id]
	if e == nil {
		return Operation{}, ErrExpired
	}
	return e.operation, nil
}
func (s *Service) Cancel(ctx context.Context, id string) (Operation, error) {
	if err := ctx.Err(); err != nil {
		return Operation{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.operations[id]
	if e == nil {
		return Operation{}, ErrExpired
	}
	if e.operation.FinishedAt != nil {
		return e.operation, nil
	}
	e.cancel()
	if e.operation.State == "preparing" {
		s.finishLocked(e, "failed", "已取消")
	}
	return e.operation, nil
}
func (s *Service) finishLocked(e *operationEntry, state, message string) {
	if e.operation.FinishedAt != nil {
		return
	}
	if e.preview != nil {
		s.options.NativeHistory.UnpinScope(e.preview.SnapshotID)
	}
	now := s.now()
	e.operation.State = state
	e.operation.Error = message
	e.operation.FinishedAt = &now
	e.preview = nil
}
func (s *Service) finish(e *operationEntry, state string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	message := ""
	if err != nil {
		message = err.Error()
	}
	s.finishLocked(e, state, message)
}

func (s *Service) HistoryAllowed(ctx context.Context, runtime, scope string) error {
	if _, err := s.options.NativeHistory.Scope(scope); err != nil {
		return ErrExpired
	}
	rec, err := s.options.Hybrid.Record(runtime)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	binding, ok := s.history[runtime+"_"+scope]
	if !ok || binding.identity != runtimeIdentity(rec) {
		return ErrConflict
	}
	return ctx.Err()
}

func (s *Service) targetBudget(ctx context.Context, target, task string) (Budget, error) {
	input := BudgetInput{TokenCap: s.options.TokenCap, UserText: task, Inline: true, MaxPromptBytes: s.options.MaxPromptBytes, MaxPromptWireBytes: s.options.MaxPromptWireBytes}
	if target != "" {
		if source, ok := s.options.Hybrid.(interface {
			ContextUsage(context.Context, string) (*hybrid.Usage, error)
		}); ok {
			usage, err := source.ContextUsage(ctx, target)
			if err != nil {
				return Budget{}, err
			}
			if usage != nil && usage.Size > 0 {
				input.Used = usage.Used
				input.Window = usage.Size
				input.Measured = true
			}
		}
	}
	return ComputeBudget(input), nil
}
