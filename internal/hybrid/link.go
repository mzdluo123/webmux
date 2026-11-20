package hybrid

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rainchan/webmux/internal/acp"
	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/session"
)

// goneTimeout bounds waiting for a stopped process tree to be observed gone
// after Stop already killed it.
const goneTimeout = 5 * time.Second

// errStopped: the actor ended while a subtask was opening a connection.
var errStopped = fmt.Errorf("%w: 服务正在关闭", ErrUnavailable)

// acpLink is one ACP agent process and its connection.
type acpLink struct {
	historyScope string
	epoch        int
	sidecar      *session.Sidecar
	conn         *acp.Conn
	sessionID    string
	caps         acp.SessionCapabilities
	prompt       acp.PromptCapabilities
	tools        acpTools
}

// stop closes the connection and ends the process, and reports whether its
// process tree was observed gone: the next writer may only start then.
func (l *acpLink) stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()
	return l.stopWithin(ctx)
}

// stopWithin is stop that kills the process tree when ctx ends first, then
// waits at most goneTimeout for it to go.
func (l *acpLink) stopWithin(ctx context.Context) error {
	_ = l.conn.Close()
	if err := l.sidecar.Stop(ctx); err == nil {
		return nil
	}
	gone := time.NewTimer(goneTimeout)
	defer gone.Stop()
	select {
	case <-l.sidecar.Done():
		return nil
	case <-gone.C:
		return errors.New("agent 进程未能结束")
	}
}

// releaseWithin closes the agent session within parent/releaseTimeout when the
// agent supports session/close, then stops the process.
func (l *acpLink) releaseWithin(parent context.Context) error {
	if l.caps.Close != nil && l.sessionID != "" {
		ctx, cancel := context.WithTimeout(parent, releaseTimeout)
		_ = l.conn.CloseSession(ctx, l.sessionID)
		cancel()
	}
	return l.stop()
}

func (a *actor) prepareTUIResume(sid string) error {
	changed, err := agentprofile.PrepareTUIResume(a.cmd.Agent, sid, a.shell.Getenv, a.svc.userHome)
	if err != nil {
		return fmt.Errorf("prepare terminal resume: %w", err)
	}
	if changed {
		a.svc.log.Info("hybrid agent session rebound for terminal", "id", a.id, "agent", a.cmd.Profile.Name)
	}
	return nil
}

// failure is a failed step: err for the requesting user (it may carry the
// agent's stderr, never log it), public for every device.
type failure struct {
	err    error
	public string
}

type agentCallError struct {
	text  string
	cause error
}

func (e *agentCallError) Error() string   { return e.text }
func (e *agentCallError) Unwrap() []error { return []error{ErrAgent, e.cause} }

// agentFailure describes a failed ACP call: the requester's error carries
// the agent's stderr tail, the public text does not.
func agentFailure(method string, err error, sidecar *session.Sidecar) failure {
	var rpc *acp.RPCError
	var detail string
	switch {
	case errors.As(err, &rpc):
		detail = fmt.Sprintf("错误码 %d: %s", rpc.Code, limitText(rpc.Message, errorTextBytes))
	case errors.Is(err, acp.ErrUnsupportedVersion):
		detail = "协议版本不受支持"
	case errors.Is(err, acp.ErrDisconnected), errors.Is(err, acp.ErrClosed):
		detail = "agent 已退出"
		if code := sidecar.ExitCode(); code >= 0 {
			detail += "（退出码 " + strconv.Itoa(code) + "）"
		}
	default:
		detail = err.Error()
	}
	public := method + ": " + detail
	full := detail
	if tail := stderrTail(sidecar.Stderr()); tail != "" {
		full += "\n" + tail
	}
	return failure{err: &agentCallError{text: method + ": " + full, cause: err}, public: public}
}

// connect checks the profile's connector, starts the ACP process under
// epoch and initializes it. The agent must support session/load and
// session/list. A failure stops the process before returning.
func (a *actor) connect(epoch int) (*acpLink, acp.InitializeResponse, *failure) {
	life := a.lifetime()
	tools, err := a.prepareACP(life)
	if err != nil {
		a.svc.log.Warn("hybrid acp check failed", "id", a.id, "missing", errors.Is(err, ErrConnectorMissing))
		public := "启动 agent 失败"
		if errors.Is(err, ErrConnectorMissing) {
			public = err.Error()
		}
		return nil, acp.InitializeResponse{}, &failure{err: err, public: public}
	}
	argv, _ := a.cmd.ACPArgv()
	sidecar, err := a.shell.StartSidecar(argv, a.cmd.ProtocolOnFD3(), tools.env)
	if err != nil {
		a.svc.log.Warn("hybrid acp start failed", "id", a.id, "error", err)
		return nil, acp.InitializeResponse{}, startFailure(a.cmd, err)
	}
	link := &acpLink{epoch: epoch, sidecar: sidecar, tools: tools}
	link.conn = acp.NewConn(sidecar.Stdout(), sidecar.Stdin(), acp.Options{Handler: &handler{a: a, epoch: epoch}, Logger: a.svc.log})
	go func() {
		<-link.conn.Done()
		a.post(func() { a.linkLost(link) })
	}()
	init, err := link.conn.Initialize(life, acp.Implementation{Name: "webmux", Title: "WebMux", Version: a.svc.version}, nil)
	if err == nil {
		caps := init.AgentCapabilities
		if !caps.LoadSession || caps.SessionCapabilities.List == nil {
			err = errMissingCapability
		}
	}
	if err != nil {
		f := a.callFailed(link, acp.MethodInitialize, err)
		if missing := exitedNotFound(a.cmd, link.sidecar); missing != nil {
			f = missing
		}
		return nil, init, f
	}
	link.caps = init.AgentCapabilities.SessionCapabilities
	link.prompt = init.AgentCapabilities.PromptCapabilities
	return link, init, nil
}

// callFailed logs a failed ACP call by method, stops the process and
// describes the failure.
func (a *actor) callFailed(link *acpLink, method string, err error) *failure {
	a.svc.log.Warn("hybrid acp call failed", "id", a.id, "method", method, "error", err)
	_ = link.stop()
	f := agentFailure(method, err, link.sidecar)
	return &f
}

// openResult is an ACP connection opened on an existing agent session.
type openResult struct {
	link    *acpLink
	init    acp.InitializeResponse
	config  []acp.ConfigOption
	resumed bool
	// notes are warnings about options that could not be restored or
	// derived, for notices.
	notes []string
	fail  *failure
}

// openACP starts the ACP process under epoch and opens agent session sid
// in cwd: with session/resume when allowed, the profile permits it and the
// agent declared it (no replay), else with session/load after beforeLoad
// (which starts a new log generation; false means the actor ended). Options
// the profile lists as not kept across loads get their recorded values
// back (D25), or on the first open the values derived from the launch
// arguments. It runs in a subtask; a failure stops the process.
func (a *actor) openACP(epoch int, sid, cwd string, resume bool, beforeLoad func() bool) openResult {
	return a.openACPWithRequirement(epoch, sid, cwd, resume, beforeLoad, false)
}

// openACPWithRequirement rechecks required resume against the new connection,
// since capabilities may differ from the writer that was just stopped.
func (a *actor) openACPWithRequirement(epoch int, sid, cwd string, resume bool, beforeLoad func() bool, requireResume bool) openResult {
	if a.externalWriterNow(sid) {
		return openResult{fail: &failure{err: ErrExternalWriter, public: ErrExternalWriter.Error()}}
	}
	link, init, fail := a.connect(epoch)
	if fail != nil {
		return openResult{fail: fail}
	}
	return a.openOnLink(link, init, sid, cwd, resume, beforeLoad, requireResume)
}

// openOnLink opens agent session sid in cwd on an already-initialized ACP
// connection link. On failure it stops link and clears replay mode if entered.
func (a *actor) openOnLink(link *acpLink, init acp.InitializeResponse, sid, cwd string, resume bool, beforeLoad func() bool, requireResume bool) openResult {
	want := a.reapplyValues()
	res := openResult{link: link, init: init}
	ctx := a.lifetime()
	servers, scope := a.sessionMCPServers(ctx)
	link.historyScope = scope
	method := acp.MethodSessionLoad
	var err error
	replaying := false
	if requireResume && (!agentprofile.ResumesSessions(a.cmd.Agent) || link.caps.Resume == nil) {
		_ = link.stop()
		return openResult{fail: &failure{err: ErrNativeResumeUnavailable, public: ErrNativeResumeUnavailable.Error()}}
	}
	if resume && agentprofile.ResumesSessions(a.cmd.Agent) && link.caps.Resume != nil {
		method = acp.MethodSessionResume
		var r acp.ResumeSessionResponse
		r, err = link.conn.ResumeSession(ctx, acp.ResumeSessionRequest{SessionID: sid, Cwd: cwd, MCPServers: servers})
		res.config, res.resumed = r.ConfigOptions, true
	} else {
		if beforeLoad == nil {
			beforeLoad = func() bool { return a.sync(a.beginControlReplay) }
		}
		if !beforeLoad() {
			_ = link.stop()
			return openResult{fail: &failure{err: errStopped, public: errStopped.Error()}}
		}
		replaying = true
		var r acp.LoadSessionResponse
		r, err = link.conn.LoadSession(ctx, acp.LoadSessionRequest{SessionID: sid, Cwd: cwd, MCPServers: servers})
		if err != nil {
			r, err = a.recoverSessionLock(ctx, link, sid, cwd, servers, err)
		}
		res.config = r.ConfigOptions
	}
	if err != nil {
		f := a.callFailed(link, method, err)
		if replaying {
			a.sync(a.finishControlReplay)
		}
		if externalWriter(err) || errors.Is(err, ErrExternalWriter) {
			f = &failure{err: ErrExternalWriter, public: ErrExternalWriter.Error()}
		}
		return openResult{fail: f}
	}
	link.sessionID = sid
	res.config, res.notes = a.reapply(link, want, res.config)
	return res
}

// recoverSessionLock answers a failed session/load: when the adapter reads
// it as a lock held by another process, a live holder is an external-writer
// conflict, while a dead one gets one force session/close and one retry, so
// a lock a crashed terminal left behind does not block switching.
func (a *actor) recoverSessionLock(ctx context.Context, link *acpLink, sid, cwd string, servers []acp.MCPServer, err error) (acp.LoadSessionResponse, error) {
	locker, ok := a.cmd.Agent.(agentprofile.SessionLocker)
	if !ok {
		return acp.LoadSessionResponse{}, err
	}
	pid, ok := locker.LockHeldBy(err)
	if !ok {
		return acp.LoadSessionResponse{}, err
	}
	if processIdentity(pid) != nil {
		return acp.LoadSessionResponse{}, fmt.Errorf("%w（PID %d）", ErrExternalWriter, pid)
	}
	if cerr := link.conn.CloseSessionMeta(ctx, sid, locker.ForceCloseMeta()); cerr != nil {
		a.svc.log.Warn("hybrid force session close failed", "id", a.id, "error", cerr)
		return acp.LoadSessionResponse{}, err
	}
	a.svc.log.Info("hybrid stale session lock released", "id", a.id, "agent", a.cmd.Profile.Name, "pid", pid)
	r, rerr := link.conn.LoadSession(ctx, acp.LoadSessionRequest{SessionID: sid, Cwd: cwd, MCPServers: servers})
	if rerr != nil {
		if pid, ok := locker.LockHeldBy(rerr); ok && processIdentity(pid) != nil {
			return r, fmt.Errorf("%w（PID %d）", ErrExternalWriter, pid)
		}
		return r, rerr
	}
	return r, nil
}

// installLink makes an opened connection the session's: versions,
// steering and the agent's options.
func (a *actor) installLink(link *acpLink, init acp.InitializeResponse, config []acp.ConfigOption, notes []string) {
	a.finishControlReplay()
	a.link = link
	// A foreground process baseline belongs to its ACP connection. Keeping
	// the old PID set after recovery would classify the new writer as work.
	a.stopBackgroundWatch()
	a.bgBase = nil
	a.agentSessionID = link.sessionID
	a.chat.ConnectorMissing = nil
	a.setConnector(nil)
	if init.AgentInfo != nil {
		a.chat.Outdated = a.outdated(init.AgentInfo.Version, link.tools.agentVersion)
	}
	steerer, ok := a.cmd.Agent.(agentprofile.Steerer)
	a.chat.Steering = ok && steerer.SteeringAvailable(init.Meta)
	a.chat.PromptCapabilities = PromptCapabilities{
		Image:           link.prompt.Image,
		EmbeddedContext: link.prompt.EmbeddedContext,
	}
	a.dirty = true
	if config != nil {
		a.emit(Event{Kind: EventConfig, Config: configOptions(config, a.cmd.Profile.ModelChoiceValues)})
	}
	if link.tools.agentMissing && !a.agentPathNoted {
		a.agentPathNoted = true
		notes = append(notes, "未找到 "+filepath.Base(a.cmd.Executable)+"，聊天端使用适配器自带的 "+a.cmd.Profile.Title)
	}
	for _, note := range notes {
		a.emit(Event{Kind: EventNotice, Notice: &Notice{Level: acp.NoticeWarning, Text: note}})
	}
}

// outdated lists the programs below their verified versions: the ACP side
// reports the connector's version when the profile has one, and the
// agent's own (from its executable) is checked beside it.
func (a *actor) outdated(reported, agentVersion string) []Outdated {
	p := a.cmd.Profile
	out := []Outdated{}
	agent := reported
	if c := p.Connector; c != nil {
		agent = agentVersion
		if order, ok := agentprofile.CompareVersions(reported, c.VerifiedVersion); ok && order < 0 {
			out = append(out, Outdated{Name: c.Title, Version: reported, Verified: c.VerifiedVersion})
		}
	} else if p.VersionExecutable {
		// The ACP side's agentInfo.version is not a release version; the
		// executable's --version decides (empty skips the notice).
		agent = agentVersion
	}
	if p.BelowVerified(agent) {
		out = append(out, Outdated{Name: p.Title, Version: agent, Verified: p.VerifiedVersion})
	}
	return out
}

// restartLog begins a new log generation for a session/load replay.
func (a *actor) restartLog() {
	a.beginControlReplay()
	a.log.restart()
	a.tr = translator{modelChoiceValues: a.cmd.Profile.ModelChoiceValues}
	a.chat.Plan, a.chat.Usage = nil, nil
	a.preview = preview{}
	a.dirty = true
}

// sync runs fn on the actor and waits for it; false if the actor ended
// before running it.
func (a *actor) sync(fn func()) bool {
	done := make(chan struct{})
	if !a.post(func() {
		fn()
		a.flush()
		close(done)
	}) {
		return false
	}
	select {
	case <-done:
		return true
	case <-a.quit:
		select {
		case <-done:
			return true
		default:
			return false
		}
	}
}
