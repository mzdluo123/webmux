package handoff

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/historysource"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

const operationLifetime = 15 * time.Minute
const maxPreviews = 32
const maxOperations = 256

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
	return out
}
func (s *Service) Preview(ctx context.Context, source string, in Request) (Preview, error) {
	var empty Preview
	if s.options.Hybrid == nil || s.options.NativeHistory == nil || s.options.HistoryProjection == nil || s.options.Launch == nil {
		return empty, ErrUnavailable
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
	header := fmt.Sprintf("Historical reference. Source agent=%q session=%q branch=%q cwd=%q. Frozen scope: %s. Source gaps: %d. Exact text: thread_read with snapshotId=%s.\n", doc.Source.Agent, doc.Source.NativeID, doc.Source.Branch, rec.Cwd, scope.ID, len(doc.Gaps), scope.ID)
	available := max(int64(0), budget.History-int64(len(header))-512)
	var selected []string
	omitted := len(projection.Projection.Omissions)
	segments := projection.Projection.Segments
	// Recent complete segments are selected within the configured history budget.
	for i := len(segments) - 1; i >= 0; i-- {
		item := fmt.Sprintf("[role=%s id=%s tool=%s]\n%s\n", segments[i].Role, segments[i].SourceID, segments[i].ToolName, segments[i].Text)
		cost := jsonBytes(item)
		if cost > available {
			omitted++
			continue
		}
		available -= cost
		selected = append(selected, item)
	}
	var history strings.Builder
	history.WriteString(header)
	fmt.Fprintf(&history, "Included: %d; omissions: %d.\n", len(selected), omitted)
	for i := len(selected) - 1; i >= 0; i-- {
		history.WriteString(selected[i])
	}
	text := history.String() + "\nCurrent user task:\n" + in.UserText
	if err := hybrid.ValidatePromptText(text, 0); err != nil {
		return empty, err
	}
	budget.Used = jsonBytes(history.String())
	budget.OverBudget = budget.OverBudget || budget.Used > budget.History
	if budget.OverBudget {
		return empty, ErrBudget
	}
	if err := s.options.NativeHistory.ReserveScopeBytes(scope.ID, len(text)+len(in.UserText)+metadata+2048); err != nil {
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
	p := &Preview{ID: id, InstanceID: s.instance, OperationID: s.instance + "_" + rand.Text(), SourceSessionID: source, SnapshotID: scope.ID, Request: in, Text: text, UserText: in.UserText, CreatedAt: now, ExpiresAt: scope.ExpiresAt, SelectedCount: len(selected), OmittedCount: omitted, GapCount: len(doc.Gaps), Budget: budget, sourceIdentity: runtimeIdentity(rec), targetIdentity: targetIdentity, configHash: fingerprint}
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
func (s *Service) send(ctx context.Context, e *operationEntry) {
	s.mu.Lock()
	p := e.preview
	s.mu.Unlock()
	if p == nil {
		return
	}
	source, sourceErr := s.options.Hybrid.Record(p.SourceSessionID)
	if sourceErr != nil || runtimeIdentity(source) != p.sourceIdentity {
		s.finish(e, "failed", ErrConflict)
		return
	}
	if _, err := s.options.NativeHistory.ScopeDocument(ctx, p.SnapshotID); err != nil {
		s.finish(e, "failed", err)
		return
	}
	config, err := s.options.Launch.Get(p.Request.ConfigID)
	if err != nil {
		s.finish(e, "failed", err)
		return
	}
	fingerprint, err := configFingerprint(config)
	if err != nil || fingerprint != p.configHash {
		s.finish(e, "failed", ErrConflict)
		return
	}
	if p.Request.TargetSessionID != "" {
		budget, err := s.targetBudget(ctx, p.Request.TargetSessionID, p.UserText)
		if err != nil || budget.NativeUsed != p.Budget.NativeUsed || budget.Window != p.Budget.Window || budget.Source != p.Budget.Source {
			s.finish(e, "failed", ErrConflict)
			return
		}
	}
	if p.Request.TargetSessionID != "" {
		target, err := s.options.Hybrid.Record(p.Request.TargetSessionID)
		if err != nil || hybrid.NativeIdentity(target) != p.targetIdentity {
			s.finish(e, "failed", ErrConflict)
			return
		}
	}
	s.mu.Lock()
	if e.operation.FinishedAt != nil {
		s.mu.Unlock()
		return
	}
	if err := ctx.Err(); err != nil {
		s.finishLocked(e, "failed", err.Error())
		s.mu.Unlock()
		return
	}
	e.operation.State = "sending"
	s.mu.Unlock()
	target := p.Request.TargetSessionID
	if target == "" {
		if s.options.Create == nil {
			s.finish(e, "failed", ErrUnavailable)
			return
		}
		in := sessioncreate.Input{ClientRequestID: rand.Text(), ConfigID: &p.Request.ConfigID, Side: string(hybrid.SideACP), Cwd: p.Request.Location.Cwd, AgentConfig: p.Request.AgentConfig}
		location := p.Request.Location
		if location.NewWorktreeBranch != "" {
			in.NewWorktree = &sessioncreate.NewWorktree{RepositoryID: location.RepositoryID, Mode: "new", Branch: location.NewWorktreeBranch, Base: location.NewWorktreeBase}
		} else if location.RepositoryID != "" {
			in.Workspace = &sessioncreate.Reference{RepositoryID: location.RepositoryID, WorktreeID: location.WorktreeID}
		}
		result, err := s.options.Create(ctx, in)
		s.mu.Lock()
		e.operation.Location = result.Location
		if result.Session != nil {
			target = result.Session.Info().ID
			e.operation.TargetSessionID = target
		}
		s.mu.Unlock()
		if err != nil {
			var step *sessioncreate.Error
			if errors.As(err, &step) {
				s.mu.Lock()
				if step.SessionID != "" {
					e.operation.TargetSessionID = step.SessionID
				}
				if step.Location != nil {
					e.operation.Location = step.Location
				}
				s.mu.Unlock()
			}
			if ctx.Err() != nil {
				s.finish(e, "unknown", errors.New("可能已发送，请检查目标会话"))
			} else {
				s.finish(e, "failed", err)
			}
			return
		}
	}
	if err := ctx.Err(); err != nil {
		s.finish(e, "failed", err)
		return
	}
	targetRecord, recordErr := s.options.Hybrid.Record(target)
	if recordErr != nil {
		s.finish(e, "failed", recordErr)
		return
	}
	s.mu.Lock()
	s.history[target+"_"+p.SnapshotID] = historyBinding{scopeID: p.SnapshotID, identity: runtimeIdentity(targetRecord)}
	s.mu.Unlock()
	var expectedUsage *hybrid.Usage
	if p.Request.TargetSessionID != "" && p.Budget.Source == "provider" {
		expectedUsage = &hybrid.Usage{Used: p.Budget.NativeUsed, Size: p.Budget.Window}
	}
	expectedIdentity := hybrid.NativeIdentity(targetRecord)
	if p.Request.TargetSessionID != "" {
		expectedIdentity = p.targetIdentity
	}
	_, err = s.options.Hybrid.Prompt(ctx, target, hybrid.PromptInput{ClientRequestID: rand.Text(), Text: p.Text, TitleSource: p.UserText, Mode: hybrid.ModeRefuse, ExpectedIdentity: expectedIdentity, AwaitAcknowledgement: true, ExpectedUsage: expectedUsage})
	if err != nil {
		if errors.Is(err, hybrid.ErrBusy) || errors.Is(err, hybrid.ErrWrongSide) || errors.Is(err, hybrid.ErrSwitching) || errors.Is(err, hybrid.ErrInvalid) || errors.Is(err, hybrid.ErrConflict) {
			s.finish(e, "failed", err)
		} else {
			s.finish(e, "unknown", errors.New("可能已发送，请检查目标会话"))
		}
		return
	}
	s.finish(e, "succeeded", nil)
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
