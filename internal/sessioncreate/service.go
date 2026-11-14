package sessioncreate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/pathutil"
	"github.com/rainchan/webmux/internal/session"
)

const maxPromptBytes = 64 << 10
const maxAttachments = 8

var requestID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

type flight struct {
	done   chan struct{}
	result Result
	err    error
	timer  *time.Timer
}
type Service struct {
	manager   *session.Manager
	options   Options
	mu        sync.Mutex
	flights   map[string]*flight
	retention time.Duration
	lifetime  context.Context
	cancel    context.CancelFunc
	active    sync.WaitGroup
	closed    bool
}

func New(manager *session.Manager, options Options) *Service {
	lifetime, cancel := context.WithCancel(context.Background())
	return &Service{manager: manager, options: options, flights: make(map[string]*flight), lifetime: lifetime, cancel: cancel}
}

// Close stops admission, cancels creation and initial-input producers, and waits
// for their cleanup before the composition root shuts down optional services.
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	for _, f := range s.flights {
		if f.timer != nil {
			f.timer.Stop()
		}
	}
	s.mu.Unlock()
	s.cancel()
	s.active.Wait()
	s.mu.Lock()
	clear(s.flights)
	s.mu.Unlock()
}
func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", session.ErrInvalid, fmt.Sprintf(format, args...))
}
func (s *Service) Create(ctx context.Context, in Input) (Result, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Result{}, &Error{Step: StepPrecheck, Err: session.ErrShuttingDown}
	}
	s.active.Add(1)
	s.mu.Unlock()
	defer s.active.Done()
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.lifetime, cancel)
	defer stop()
	defer cancel()
	if s.lifetime.Err() != nil {
		cancel()
	}
	if in.ClientRequestID == "" {
		return s.create(ctx, in)
	}
	if !requestID.MatchString(in.ClientRequestID) {
		return Result{}, &Error{Step: StepPrecheck, Err: invalid("clientRequestId 必须是 8 到 64 个字母、数字、- 或 _")}
	}
	s.mu.Lock()
	f, exists := s.flights[in.ClientRequestID]
	if !exists {
		f = &flight{done: make(chan struct{})}
		s.flights[in.ClientRequestID] = f
	}
	s.mu.Unlock()
	if exists {
		select {
		case <-f.done:
			return f.result, f.err
		case <-ctx.Done():
			return Result{}, &Error{Step: StepPrecheck, Err: ctx.Err()}
		}
	}
	f.result, f.err = s.create(ctx, in)
	close(f.done)
	s.mu.Lock()
	if !s.closed {
		retention := s.retention
		if retention == 0 {
			retention = 10 * time.Minute
		}
		f.timer = time.AfterFunc(retention, func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.flights[in.ClientRequestID] == f {
				delete(s.flights, in.ClientRequestID)
			}
		})
	}
	s.mu.Unlock()
	return f.result, f.err
}

type plan struct {
	command                  Command
	location                 LocationPlan
	text, instruction, title string
	attachments              []string
}

func (s *Service) prepare(ctx context.Context, in Input) (plan, error) {
	if err := ctx.Err(); err != nil {
		return plan{}, err
	}
	p := plan{text: strings.TrimSpace(in.Prompt)}
	// Capability checks precede any optional preflight, worktree or process work.
	if (in.Workspace != nil || in.NewWorktree != nil || in.UseInitInstruction) && s.options.Locations == nil {
		return p, ErrUnavailable
	}
	if in.ConfigID != nil && s.options.Commands == nil {
		return p, ErrUnavailable
	}
	if (in.Side == "acp" || len(in.AgentConfig) > 0 || in.PermissionMode != "") && s.options.Chat == nil {
		return p, ErrUnavailable
	}
	if (len(in.Attachments) > 0 || in.ExpandPrompt && (strings.Contains(p.text, "{{dir}}") || strings.Contains(p.text, "{{webmux}}"))) && s.options.Files == nil {
		return p, ErrUnavailable
	}
	if in.NewWorktree != nil && (in.Cwd != "" || in.Workspace != nil) {
		return p, invalid("cwd、workspace 与 newWorktree 只能选择一个")
	}
	if err := session.ValidateInitialSize(in.Cols, in.Rows); err != nil {
		return p, invalid("%v", err)
	}
	p.command = Command{Cmd: in.Cmd, Env: in.Env}
	if in.ConfigID != nil {
		if *in.ConfigID == "" {
			return p, invalid("configId must not be empty")
		}
		if len(in.Cmd) > 0 || len(in.Env) > 0 {
			return p, invalid("configId cannot be combined with cmd or env")
		}
		var err error
		p.command, err = s.options.Commands.Resolve(ctx, *in.ConfigID)
		if err != nil {
			return p, err
		}
	}
	if err := session.ValidateEnv(p.command.Env); err != nil {
		return p, invalid("%v", err)
	}
	switch in.Side {
	case "", "tui":
	case "acp":
		if !p.command.Chat {
			return p, invalid("聊天视图只适用于开启了聊天视图的启动配置")
		}
	default:
		return p, invalid("view 只能是 terminal 或 chat")
	}
	// A saved preference is not a request to activate a disabled contribution.
	// Explicit chat fields were rejected above; launch-only still runs its argv.
	if s.options.Chat == nil {
		p.command.Chat = false
	}
	if in.Side != "acp" && (len(in.AgentConfig) > 0 || in.PermissionMode != "") {
		return p, invalid("agentConfig 与 permissionMode 只适用于聊天视图")
	}
	if in.PermissionMode != "" && in.PermissionMode != "ask" && in.PermissionMode != "auto" {
		return p, invalid("permissionMode 只能是 ask 或 auto")
	}
	if len(in.Prompt) > maxPromptBytes {
		return p, invalid("用户指令超过 %d 字节", maxPromptBytes)
	}
	if len(in.Attachments) > maxAttachments {
		return p, invalid("每条消息最多 %d 个附件", maxAttachments)
	}
	if len(in.Attachments) > 0 && p.text == "" {
		return p, invalid("附件需要随消息发送，请输入消息")
	}
	if in.UseInitInstruction && in.Workspace == nil && in.NewWorktree == nil {
		return p, invalid("初始化指令属于仓库，请选择仓库中的位置")
	}
	for _, path := range in.Attachments {
		clean, err := s.options.Files.StagedPath(path)
		if err != nil || slices.Contains(p.attachments, clean) {
			return p, invalid("附件不是为新会话上传的文件")
		}
		p.attachments = append(p.attachments, clean)
	}
	if s.options.Locations != nil {
		var err error
		p.location, err = s.options.Locations.Prepare(ctx, in)
		if err != nil {
			return p, err
		}
	}
	p.instruction = p.location.Instruction
	for _, text := range []string{p.instruction, p.text} {
		if strings.Contains(text, "{{webmux}}") && (in.ExpandPrompt || text == p.instruction) && (s.options.Files == nil || s.options.Brief == nil) {
			return p, fmt.Errorf("%w: WebMux 工具说明不可用", ErrUnavailable)
		}
		if strings.Contains(text, "{{dir}}") && (in.ExpandPrompt || text == p.instruction) && s.options.Files == nil {
			return p, fmt.Errorf("%w: 临时文件夹不可用", ErrUnavailable)
		}
	}
	if p.instruction != "" {
		n := strings.Count(p.instruction, "{{user}}")
		size := len(p.instruction) + 2 + len(p.text)
		if n > 0 {
			size = len(p.instruction) - n*len("{{user}}") + n*len(p.text)
		}
		if size > maxPromptBytes {
			return p, invalid("消息与初始化指令合计超过 %d 字节", maxPromptBytes)
		}
	}
	title := p.text
	if in.ExpandPrompt {
		title = strings.NewReplacer("{{dir}}", "", "{{webmux}}", "").Replace(title)
	}
	p.title = promptTitle(title)
	cwd := p.location.Cwd
	if cwd == "" {
		cwd = in.Cwd
	}
	if cwd == "" {
		cwd = s.options.DefaultCwd
	}
	if p.command.Chat {
		directory, err := pathutil.Expand(cwd)
		if err != nil {
			return p, invalid("directory: %v", err)
		}
		if err := s.options.Chat.Check(ctx, in, p.command, directory); err != nil {
			return p, err
		}
	}
	return p, nil
}

// Directory resolves and validates an existing process working directory.
func Directory(path string) (string, error) {
	absolute, err := pathutil.Expand(path)
	if err != nil {
		return "", &directoryError{err: err}
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", &directoryError{err: err}
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", &directoryError{err: err}
	}
	if !info.IsDir() {
		return "", &directoryError{err: fmt.Errorf("path %q is not a directory", absolute)}
	}
	return absolute, nil
}
func (s *Service) create(ctx context.Context, in Input) (Result, error) {
	var location *Location
	fail := func(step, id string, err error) (Result, error) {
		return Result{}, &Error{Step: step, Location: location, SessionID: id, Err: err}
	}
	p, err := s.prepare(ctx, in)
	if err != nil {
		return fail(StepPrecheck, "", err)
	}
	if p.location.Create != nil {
		location, err = p.location.Create(ctx)
		if err != nil {
			return fail(StepWorktree, "", err)
		}
	}
	cwd := p.location.Cwd
	if location != nil {
		cwd = location.Path
	}
	if cwd == "" {
		cwd = in.Cwd
	}
	if cwd == "" {
		cwd = s.options.DefaultCwd
	}
	cwd, err = Directory(cwd)
	if err != nil {
		return fail(StepSession, "", err)
	}
	if p.location.Acquire != nil {
		var release func()
		cwd, release, err = p.location.Acquire(ctx, cwd)
		if err != nil {
			return fail(StepSession, "", err)
		}
		defer release()
	}
	if err = ctx.Err(); err != nil {
		return fail(StepSession, "", err)
	}
	var prepared string
	var preparedOK bool
	prompt := func(id string) (string, error) {
		if preparedOK {
			return prepared, nil
		}
		text, err := s.compose(id, in, p)
		if err != nil {
			return "", err
		}
		prepared, preparedOK = text, true
		return text, nil
	}
	var initial func(string) (string, error)
	hasPrompt := p.text != "" || p.instruction != ""
	if in.Side != "acp" && hasPrompt {
		initial = prompt
	}
	if in.Name == "" && in.Side != "acp" {
		in.Name = p.title
	}
	var item *session.Session
	sent := false
	if p.command.Chat {
		item, sent, err = s.options.Chat.Create(ctx, in, p.command, cwd, initial)
	} else {
		item, err = s.manager.Create(session.CreateInput{Cwd: cwd, Cmd: p.command.Cmd, Env: p.command.Env, Name: in.Name, ConfigID: p.command.ConfigID, Attention: p.command.Attention, Cols: in.Cols, Rows: in.Rows})
	}
	if err != nil {
		return fail(StepSession, "", err)
	}
	if p.location.Associate != nil {
		p.location.Associate(item)
	}
	id := item.Info().ID
	warnings := []string{}
	if in.Side == "acp" && (len(in.AgentConfig) > 0 || in.PermissionMode != "") {
		warnings, err = s.options.Chat.Configure(ctx, id, in)
		if err != nil {
			return fail(StepConfig, id, err)
		}
	}
	var receipt *session.InputResult
	if hasPrompt && !sent {
		attachments := p.attachments
		if in.Side == "acp" && len(attachments) > 0 {
			attachments, err = s.options.Files.Adopt(id, attachments)
			if err != nil {
				return fail(StepPrompt, id, err)
			}
		}
		text, err := prompt(id)
		if err != nil {
			return fail(StepPrompt, id, err)
		}
		if in.Side == "acp" {
			request := in.ClientRequestID
			if request == "" {
				request = "first-" + id
			}
			result, err := s.options.Chat.Prompt(ctx, id, request, text, p.title, attachments)
			if err != nil {
				return fail(StepPrompt, id, err)
			}
			receipt = &result
		} else if err := s.sendTerminalPrompt(item, text); err != nil {
			return fail(StepPrompt, id, err)
		}
	}
	return Result{Session: item, Location: location, ConfigWarnings: warnings, InitialInput: receipt}, nil
}
func (s *Service) expand(id, text string) (string, error) {
	if strings.Contains(text, "{{dir}}") || strings.Contains(text, "{{webmux}}") {
		dir, err := s.options.Files.Ensure(id)
		if err != nil {
			return "", err
		}
		text = strings.ReplaceAll(text, "{{dir}}", dir)
		if strings.Contains(text, "{{webmux}}") {
			text = strings.ReplaceAll(text, "{{webmux}}", s.options.Brief(id, dir))
		}
	}
	return text, nil
}
func (s *Service) compose(id string, in Input, p plan) (string, error) {
	user := p.text
	var err error
	var paths []string
	if in.Side != "acp" && len(p.attachments) > 0 {
		paths, err = s.options.Files.Adopt(id, p.attachments)
		if err != nil {
			return "", err
		}
	}
	if in.ExpandPrompt {
		user, err = s.expand(id, user)
		if err != nil {
			return "", err
		}
	}
	text := user
	if p.instruction != "" {
		instruction, err := s.expand(id, p.instruction)
		if err != nil {
			return "", err
		}
		if strings.Contains(instruction, "{{user}}") {
			text = strings.ReplaceAll(instruction, "{{user}}", user)
		} else {
			text = instruction + "\n\n" + user
		}
		if p.text == "" {
			text = strings.TrimSpace(strings.ReplaceAll(instruction, "{{user}}", ""))
		}
	}
	if len(text) > maxPromptBytes {
		return "", invalid("展开后的消息超过 %d 字节", maxPromptBytes)
	}
	if len(paths) > 0 {
		if text != "" {
			text += "\n\n"
		}
		text += "附件路径：\n" + strings.Join(paths, "\n")
	}
	return text, nil
}
func promptTitle(text string) string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(text, ""))
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) <= 80 {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:79])) + "…"
}

func (s *Service) sendTerminalPrompt(item *session.Session, text string) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return session.ErrShuttingDown
	}
	s.active.Add(1)
	s.mu.Unlock()
	go func() { defer s.active.Done(); _ = item.SendWhenReady(s.lifetime, text) }()
	return nil
}
