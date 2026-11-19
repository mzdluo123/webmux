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
	"os"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/acp"
	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/session"
)

const (
	// defaultScrollbackBytes matches the session default when no limit is
	// configured.
	defaultScrollbackBytes = 2 << 20
	// shutdownTimeout bounds stopping the ACP processes in Close: actors
	// stop them in parallel and kill those still running after it.
	shutdownTimeout = 10 * time.Second
	// shutdownJoin bounds Close as a whole: the kill, the wait for killed
	// processes to go (goneTimeout), and a margin for the actors to end.
	shutdownJoin = shutdownTimeout + goneTimeout + time.Second
)

// Options configure a Service. Only cmd/ builds them.
type Options struct {
	HistoryReader func(context.Context, string) []acp.MCPServer
	// Agents is the selected runtime registry. Nil enables no agents.
	Agents *agentprofile.Registry
	// NativeRoot resolves the native history directory from the actual launch environment.
	NativeRoot func(agent string, env map[string]string) string
	// MCPServers supplies instance tools on new/load/resume, independently of history capture.
	// Only cmd assembles descriptors; nil leaves library-created sessions unchanged.
	MCPServers func(context.Context) []acp.MCPServer
	// HistoryReader is assembled by cmd; nil disables the optional MCP tool.
	// DispatchCheckpoint records source-neutral continuity after acknowledgement.

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
	// Uploads validates that path is one of session id's uploaded files and
	// returns it cleaned (artifacts.Service.UploadPath). Nil refuses every
	// attachment.
	Uploads func(id, path string) (string, error)
}

// timing holds the service's waits; tests shorten them.
type timing struct {
	// interrupt is how long a switch waits for the terminal UI to go idle
	// after the interrupt key before it asks the user (D21).
	interrupt time.Duration
	// close bounds waiting for a side's processes to stop during a switch or close.
	close time.Duration
	// probeDelay debounces the eligibility probe after the terminal's
	// agent session or cwd changes.
	probeDelay time.Duration
	// observe is how often a terminal side's identity is read.
	observe time.Duration
	// connectorRecheck is how often a missing connector is looked for
	// again while the terminal side runs.
	connectorRecheck time.Duration
	// bgPoll is how often a finished turn with unfinished tool or background
	// work checks the agent's process tree before starting a status refresh.
	bgPoll time.Duration
}

var defaultTiming = timing{
	interrupt: 10 * time.Second, close: closeTimeout, probeDelay: time.Second, observe: 500 * time.Millisecond,
	connectorRecheck: connectorRecheck, bgPoll: bgPoll,
}

// Service owns the hybrid sessions of one server.
type Service struct {
	historyReader func(context.Context, string) []acp.MCPServer
	mcpServers    func(context.Context) []acp.MCPServer
	nativeRoot    func(agent string, env map[string]string) string
	manager       *session.Manager
	records       *records
	// options remembers each launch config's agent options for display
	// before a session starts (agent-options.json).
	options    *agentOptions
	capacity   int
	version    string
	log        *slog.Logger
	timing     timing
	uploads    func(id, path string) (string, error)
	workspace  func(id string) *WorkspaceRef
	metadataMu sync.RWMutex
	metadata   map[string]*Metadata
	// agents recognises launch commands and terminal signals.
	agents *agentprofile.Registry
	// userHome is the service user's home directory, where agents keep
	// their state unless the session environment says otherwise.
	userHome string
	// probes holds a token per running eligibility probe (maxProbes).
	probes chan struct{}
	// resumes dedupes resume requests by client request ID.
	resumes dedupe
	// onEvicted cleans up after an auto-evicted record: its artifacts folder
	// and release ref. Nil keeps them.
	onEvicted func(context.Context, Record)

	mu          sync.Mutex
	actors      map[string]*actor
	closed      bool
	closeCtx    context.Context
	closeCancel context.CancelFunc
	closeOnce   sync.Once
	closeDone   chan struct{}
	starting    sync.WaitGroup
	// recovering holds the record IDs a resume or removal is working on.
	recovering map[string]bool
	// writerClaims serializes lifecycle changes for each runtime identity.
	writerClaims map[string]writerClaim
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
	userHome, _ := os.UserHomeDir()
	closeCtx, closeCancel := context.WithCancel(context.Background())
	s := &Service{
		mcpServers: o.MCPServers, historyReader: o.HistoryReader, manager: o.Manager, records: openRecords(o.StateDir), options: openAgentOptions(o.StateDir), capacity: capacity, version: o.Version, log: log,
		uploads: o.Uploads, userHome: userHome, agents: o.Agents,
		nativeRoot: o.NativeRoot,
		metadata:   make(map[string]*Metadata),
		timing:     defaultTiming, probes: make(chan struct{}, maxProbes), actors: make(map[string]*actor),
		closeCtx:    closeCtx,
		closeCancel: closeCancel,
		closeDone:   make(chan struct{}),
		recovering:  make(map[string]bool),
	}
	if err := s.records.recoverAfterRestart(time.Now()); err != nil {
		log.Warn("hybrid sessions unavailable", "error", err)
	}

	s.records.setLogger(log)
	s.options.setLogger(log)
	logAgentOptions(log, s.options)
	o.Manager.SetIDReserver(s.records.reserved)
	return s
}

// Unavailable reports why hybrid sessions cannot be created, or nil.
func (s *Service) Unavailable() error {

	return s.records.unavailable()
}

// OnEvicted sets the cleanup of an auto-evicted record, called after its
// actor and shell ended. The server removes the record's artifacts folder
// and release ref there. Set it before serving.
func (s *Service) OnEvicted(fn func(context.Context, Record)) { s.onEvicted = fn }

// uploadCheck validates attachment paths against session id's uploads.
func (s *Service) uploadCheck(id string) func(string) (string, error) {
	return func(path string) (string, error) {
		if s.uploads == nil {
			return "", errors.New("上传不可用")
		}
		return s.uploads(id, path)
	}
}

func (s *Service) beginOperation() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("%w: 服务已关闭", ErrUnavailable)
	}
	s.starting.Add(1)
	return nil
}

// Close stops every actor and its ACP process, persists StateClosed for
// sessions already closing, leaves other active sessions active for restart
// recovery, waits within shutdownJoin for actors, probes, subtasks and
// startup operations to finish, and then closes the persistent stores.
// Terminal runs are left to Manager.Shutdown.
func (s *Service) Close() {
	s.closeOnce.Do(func() {
		defer close(s.closeDone)
		s.mu.Lock()
		s.closed = true
		s.closeCancel()
		actors := make([]*actor, 0, len(s.actors))
		for _, a := range s.actors {
			actors = append(actors, a)
		}
		s.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		join, cancelJoin := context.WithTimeout(context.Background(), shutdownJoin)
		defer cancelJoin()
		for _, a := range actors {
			a.post(func() { a.shutdown(ctx) })
		}
		for _, a := range actors {
			stopped := false
			select {
			case <-a.quit:
				stopped = true
			case <-join.Done():
				select {
				case <-a.quit:
					stopped = true
				default:
				}
			}
			if !stopped || !waitGroupWithin(join, &a.probes) || !waitGroupWithin(join, &a.subtasks) {
				s.log.Warn("hybrid session did not stop in time", "id", a.id)
			}
		}
		if !waitGroupWithin(join, &s.starting) {
			s.log.Warn("hybrid startup operations did not stop in time")
		}
		s.records.close()
		s.options.close()
	})
	<-s.closeDone
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

// recoveringNow reports whether id is being resumed or removed right now, so
// auto-eviction must not drop it.
func (s *Service) recoveringNow(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recovering[id]
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
