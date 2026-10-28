// Package hybrid runs hybrid sessions: one agent conversation that moves
// between the agent's terminal UI (a PTY run of a session shell) and an
// ACP chat (a sidecar process of the same shell), under one WebMux session
// ID. Each session is owned by an actor goroutine that drives the shell
// (session.Driver), translates ACP updates into normalized chat events,
// and keeps the session's record in hybrid-sessions.json.
//
// Content safety: nothing here logs prompts, messages, tool output,
// permission options, form content or agent stderr. Agent stderr only
// reaches the user who requested a failed start, in the returned error.
package hybrid

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/session"
)

const (
	// defaultScrollbackBytes matches the session default when no limit is
	// configured.
	defaultScrollbackBytes = 2 << 20
	// shutdownTimeout bounds Close: actors stop their ACP processes in
	// parallel within it.
	shutdownTimeout = 10 * time.Second
)

// Options configure a Service. Only cmd/ builds them.
type Options struct {
	// StateDir holds hybrid-sessions.json.
	StateDir string
	Manager  *session.Manager
	// ScrollbackBytes bounds each session's chat event log, like a
	// terminal's scrollback (config scrollback_bytes).
	ScrollbackBytes int
	// Version is the WebMux version sent as ACP clientInfo.
	Version string
	// Logger receives lifecycle events, method names and error codes only.
	Logger *slog.Logger
}

// Service owns the hybrid sessions of one server.
type Service struct {
	manager  *session.Manager
	records  *records
	capacity int
	version  string
	log      *slog.Logger

	mu     sync.Mutex
	actors map[string]*actor
	closed bool
}

// New opens the record store, marks sessions left active by an earlier
// server interrupted (ending their leftover processes), and reserves the
// recorded IDs in the manager. A store that cannot be opened leaves the
// service unavailable for hybrid creation; ordinary sessions keep working.
func New(o Options) *Service {
	log := o.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	capacity := o.ScrollbackBytes
	if capacity <= 0 {
		capacity = defaultScrollbackBytes
	}
	if o.Version == "" {
		o.Version = "dev"
	}
	s := &Service{manager: o.Manager, records: openRecords(o.StateDir), capacity: capacity, version: o.Version, log: log, actors: make(map[string]*actor)}
	if err := s.records.recoverAfterRestart(time.Now()); err != nil {
		log.Warn("hybrid sessions unavailable", "error", err)
	}
	o.Manager.SetIDReserver(s.records.reserved)
	return s
}

// Unavailable reports why hybrid sessions cannot be created, or nil.
func (s *Service) Unavailable() error { return s.records.unavailable() }

// Close stops every actor and its ACP process without changing records,
// so a restart finds the sessions active and marks them interrupted, then
// releases the store. Terminal runs are left to Manager.Shutdown.
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	actors := make([]*actor, 0, len(s.actors))
	for _, a := range s.actors {
		actors = append(actors, a)
	}
	s.mu.Unlock()
	deadline := time.NewTimer(shutdownTimeout)
	defer deadline.Stop()
	for _, a := range actors {
		a.post(a.shutdown)
	}
	for _, a := range actors {
		select {
		case <-a.quit:
		case <-deadline.C:
			s.log.Warn("hybrid session did not stop in time", "id", a.id)
		}
	}
	s.records.close()
}

// CreateInput starts a hybrid session from a saved launch config. Cwd is
// resolved again through symlinks, as ACP cwds must be. Env reaches the
// processes only; it is never stored.
type CreateInput struct {
	Cwd       string
	Name      string
	ConfigID  string
	Cmd       []string
	Env       map[string]string
	StartSide Side
}

// Create registers a hybrid session shell, records it, and starts its first
// side: the terminal UI (default) or the ACP chat (initialize, then
// session/new). A failed start removes the shell and the record; an ACP
// failure carries the agent's stderr tail in its message.
func (s *Service) Create(ctx context.Context, in CreateInput) (*session.Session, error) {
	if err := s.records.unavailable(); err != nil {
		return nil, err
	}
	side := in.StartSide
	if side == "" {
		side = SideTUI
	}
	if side != SideTUI && side != SideACP {
		return nil, fmt.Errorf("%w: startSide 只能是 tui 或 acp", ErrInvalid)
	}
	cmd, err := agentprofile.Parse(in.Cmd)
	if err != nil {
		var unsupported *agentprofile.UnsupportedError
		if errors.As(err, &unsupported) {
			return nil, fmt.Errorf("%w: 该启动配置不能用于混合会话: %s", ErrInvalid, unsupported.Reason.Message())
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	cwd, err := filepath.EvalSymlinks(in.Cwd)
	if err == nil {
		cwd, err = filepath.Abs(cwd)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: 目录: %v", ErrInvalid, err)
	}
	if s.records.count() >= MaxRecords {
		return nil, ErrLimit
	}
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return nil, fmt.Errorf("%w: 服务已关闭", ErrUnavailable)
	}
	a := newActor(s, cmd, cwd, in.ConfigID, side)
	// The shell shows the terminal command, with the options the profile
	// injects (OpenCode's --standalone); the record keeps the config's argv.
	shell, err := s.manager.CreateShell(session.ShellInput{
		CreateInput: session.CreateInput{Cwd: cwd, Cmd: cmd.TUIArgv(), Name: in.Name, Env: in.Env, ConfigID: in.ConfigID},
		Overlay:     a.overlay(),
	}, a)
	if err != nil {
		return nil, shellError(err)
	}
	info := shell.Info()
	a.id, a.shell = info.ID, shell
	now := time.Now().UTC()
	rec := Record{
		ID: info.ID, Name: in.Name, ConfigID: in.ConfigID, Agent: cmd.Profile.Name, Cwd: cwd,
		Cmd: in.Cmd, Side: side, PreviousIDs: []string{}, Config: map[string]string{},
		State: StateActive, CreatedAt: now, UpdatedAt: now,
	}
	if rec.Name == "" {
		rec.Name = info.Title
	}
	if err := s.records.add(rec); err != nil {
		// The shell has no process yet: removal cannot be refused.
		_ = s.manager.Remove(info.ID)
		return nil, err
	}
	s.mu.Lock()
	s.actors[a.id] = a
	s.mu.Unlock()
	go a.run()
	go a.watchShell()
	started := make(chan error, 1)
	a.post(func() { a.start(started) })
	// The start runs to completion even if ctx ends, so a cancelled request
	// never leaves a half-started session behind.
	if err := <-started; err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		s.log.Info("hybrid session created after its request ended", "id", a.id)
	}
	return shell, nil
}

// shellError keeps the session start error kinds as hybrid kinds.
func shellError(err error) error {
	switch {
	case errors.Is(err, session.ErrInvalid):
		return fmt.Errorf("%w: 启动会话失败: %v", ErrInvalid, err)
	case errors.Is(err, session.ErrShuttingDown):
		return fmt.Errorf("%w: 启动会话失败: %v", ErrUnavailable, err)
	}
	return fmt.Errorf("启动会话失败: %w", err)
}

func (s *Service) unregister(a *actor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.actors[a.id] == a {
		delete(s.actors, a.id)
	}
}

func (s *Service) actor(id string) (*actor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.actors[id]
	if !ok {
		return nil, ErrNotFound
	}
	return a, nil
}

// Subscribe opens a gap-free view of session id's chat: a snapshot, the
// events after resume (or all cached events, with Reset when resume is no
// longer in the log), then live frames. A subscriber that falls behind is
// dropped with ErrSlow.
func (s *Service) Subscribe(id string, resume *ResumePoint) (*Subscription, error) {
	a, err := s.actor(id)
	if err != nil {
		return nil, err
	}
	return a.log.subscribe(resume)
}

// PromptResult identifies the turn a prompt started.
type PromptResult struct {
	TurnID   string
	PromptID string
}

// Prompt sends text to the agent as a new turn. It fails with ErrWrongSide
// outside the chat side and ErrConflict while a turn runs. The turn's
// events (turn.start, the user message, the agent's updates, turn.end)
// reach the event log as they happen.
func (s *Service) Prompt(ctx context.Context, id, text string) (PromptResult, error) {
	a, err := s.actor(id)
	if err != nil {
		return PromptResult{}, err
	}
	return call(ctx, a, func() (PromptResult, error) { return a.prompt(text) })
}

// Cancel stops the running turn: pending requests are answered cancelled
// and the agent gets session/cancel. Without a running turn it does
// nothing.
func (s *Service) Cancel(ctx context.Context, id string) error {
	a, err := s.actor(id)
	if err != nil {
		return err
	}
	_, err = call(ctx, a, func() (struct{}, error) { a.cancelTurn(); return struct{}{}, nil })
	return err
}

// Answer resolves pending request requestID. The first answer wins; later
// ones fail with ErrAnswered.
func (s *Service) Answer(ctx context.Context, id, requestID string, answer Answer) error {
	a, err := s.actor(id)
	if err != nil {
		return err
	}
	_, err = call(ctx, a, func() (struct{}, error) { return struct{}{}, a.answer(requestID, answer) })
	return err
}
