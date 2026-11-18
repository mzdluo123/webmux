// Package voice runs the external STDIN/STDOUT speech recognition command
// configured in [voice].
package voice

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/session"
)

// ErrDisabled reports that no external voice command is configured.
var ErrDisabled = errors.New("voice recognition is not configured")

const (
	// DefaultReadyDelay is the time after process start before emitting an
	// implicit "ready" event if the subprocess has not printed one itself.
	DefaultReadyDelay = 150 * time.Millisecond
	// DefaultFinishTimeout bounds how long StopInput waits for the subprocess
	// to flush its final transcription after STDIN is closed.
	DefaultFinishTimeout = 15 * time.Second

	// sessionEventQueueCapacity absorbs bursts of partial recognition updates
	// while the WebSocket writer sends prior frames.
	sessionEventQueueCapacity = 32
	// stdoutDrainTimeout bounds how long pump waits for inherited stdout/stderr
	// descriptors to close after the recognition leader exits.
	stdoutDrainTimeout = 200 * time.Millisecond
)

// Options configures the external speech recognition service.
type Options struct {
	Cmd           []string
	Env           []string
	Cwd           string
	ReadyDelay    time.Duration
	FinishTimeout time.Duration
}

// EventType identifies a voice recognition event emitted by a Session.
type EventType string

const (
	EventReady   EventType = "ready"
	EventPartial EventType = "partial"
	EventFinal   EventType = "final"
	EventError   EventType = "error"
)

// Event is one recognition update from the subprocess.
type Event struct {
	Type  EventType
	Text  string
	Error string
}

// Service spawns speech recognition subprocesses per recording session.
type Service struct {
	mu            sync.RWMutex
	cmd           []string
	env           []string
	cwd           string
	readyDelay    time.Duration
	finishTimeout time.Duration
}

// New constructs a Service from Options.
func New(opts Options) *Service {
	readyDelay := opts.ReadyDelay
	if readyDelay <= 0 {
		readyDelay = DefaultReadyDelay
	}
	finishTimeout := opts.FinishTimeout
	if finishTimeout <= 0 {
		finishTimeout = DefaultFinishTimeout
	}
	return &Service{
		cmd:           normalizeCmd(opts.Cmd),
		env:           append([]string(nil), opts.Env...),
		cwd:           opts.Cwd,
		readyDelay:    readyDelay,
		finishTimeout: finishTimeout,
	}
}

// Enabled reports whether a voice recognition command is configured.
func (s *Service) Enabled() bool {
	if s == nil {
		return false
	}
	argv, _, _ := s.snapshot()
	return len(argv) > 0
}

// snapshot returns the command, environment and directory a new recording
// uses; a running recording keeps the ones it started with.
func (s *Service) snapshot() (argv, env []string, cwd string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cmd, s.env, s.cwd
}

// Reconfigure replaces the command, environment and directory that later
// recordings use. Recordings in progress are not affected.
func (s *Service) Reconfigure(opts Options) {
	cmd := normalizeCmd(opts.Cmd)
	env := append([]string(nil), opts.Env...)
	s.mu.Lock()
	s.cmd, s.env, s.cwd = cmd, env, opts.Cwd
	s.mu.Unlock()
}

func normalizeCmd(cmd []string) []string {
	if len(cmd) > 0 && strings.TrimSpace(cmd[0]) != "" {
		return append([]string(nil), cmd...)
	}
	return nil
}

type stdoutMessage struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Error string `json:"error"`
}

// Start launches a new recognition subprocess bound to ctx.
func (s *Service) Start(ctx context.Context) (*Session, error) {
	argv, env, cwd := s.snapshot()
	if len(argv) == 0 {
		return nil, ErrDisabled
	}
	runCtx, cancel := context.WithCancel(ctx)
	// #nosec G204 -- argv is the administrator-configured command, never shell-expanded.
	cmd := exec.CommandContext(runCtx, argv[0], argv[1:]...)
	if cmd.Err != nil {
		cancel()
		return nil, cmd.Err
	}
	if cwd != "" {
		cmd.Dir = cwd
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	configureCommand(cmd)

	var files []*os.File
	closeAll := func() {
		for _, f := range files {
			_ = f.Close()
		}
	}
	pipe := func() (*os.File, *os.File, error) {
		r, w, err := os.Pipe()
		if err == nil {
			files = append(files, r, w)
		}
		return r, w, err
	}
	stdinR, stdinW, err := pipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdoutR, stdoutW, err := pipe()
	if err != nil {
		closeAll()
		cancel()
		return nil, err
	}
	stderrR, stderrW, err := pipe()
	if err != nil {
		closeAll()
		cancel()
		return nil, err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdinR, stdoutW, stderrW
	wait, err := session.StartCommand(cmd)
	if err != nil {
		closeAll()
		cancel()
		return nil, err
	}
	_ = stdinR.Close()
	_ = stdoutW.Close()
	_ = stderrW.Close()

	var stderrBuf bytes.Buffer
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		_, _ = io.Copy(&limitedBuffer{buf: &stderrBuf, limit: 4096}, stderrR)
	}()
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- wait()
	}()
	sess := newSession(cancel, stdinW)
	go s.pump(runCtx, sess, stdoutR, stderrR, stderrDone, waitCh, &stderrBuf)
	return sess, nil
}

func (s *Service) FinishTimeout() time.Duration {
	if s == nil || s.finishTimeout <= 0 {
		return DefaultFinishTimeout
	}
	return s.finishTimeout
}

func (s *Service) pump(ctx context.Context, sess *Session, stdoutR, stderrR *os.File, stderrDone <-chan struct{}, waitCh <-chan error, stderrBuf *bytes.Buffer) {
	defer close(sess.done)
	defer close(sess.events)
	defer sess.stopFinishTimer()

	lines := make(chan string)
	scanStop := make(chan struct{})
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		defer close(lines)
		scanner := bufio.NewScanner(stdoutR)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			select {
			case lines <- line:
			case <-scanStop:
				return
			}
		}
	}()

	readyEmitted := false
	sendEvent := func(ev Event) {
		select {
		case sess.events <- ev:
		case <-ctx.Done():
		}
	}
	emitReady := func() {
		if !readyEmitted {
			readyEmitted = true
			sendEvent(Event{Type: EventReady})
		}
	}

	readyTimer := time.NewTimer(s.readyDelay)
	defer readyTimer.Stop()

	var (
		plainLines []string
		finals     []string
		sawFinal   bool
		sawError   bool
		waitErr    error
		exited     bool
		drainTimer *time.Timer
		drainC     <-chan time.Time
	)
	stopDrain := func() {
		if drainTimer != nil {
			drainTimer.Stop()
		}
	}
	defer stopDrain()

	scanCh := (<-chan string)(lines)
	for scanCh != nil {
		select {
		case <-ctx.Done():
			scanCh = nil
		case <-readyTimer.C:
			emitReady()
		case err := <-waitCh:
			waitErr = err
			exited = true
			waitCh = nil
			drainTimer = time.NewTimer(stdoutDrainTimeout)
			drainC = drainTimer.C
		case <-drainC:
			drainC = nil
			_ = stdoutR.Close()
			_ = stderrR.Close()
		case line, ok := <-scanCh:
			if !ok {
				scanCh = nil
				continue
			}
			if strings.HasPrefix(line, "{") {
				var msg stdoutMessage
				if json.Unmarshal([]byte(line), &msg) == nil && msg.Type != "" {
					switch EventType(msg.Type) {
					case EventReady:
						readyTimer.Stop()
						emitReady()
					case EventPartial:
						readyTimer.Stop()
						emitReady()
						sendEvent(Event{Type: EventPartial, Text: msg.Text})
					case EventFinal:
						readyTimer.Stop()
						emitReady()
						sawFinal = true
						if msg.Text != "" {
							finals = append(finals, msg.Text)
						}
					case EventError:
						readyTimer.Stop()
						readyEmitted = true
						sawError = true
						errMsg := strings.TrimSpace(msg.Error)
						if errMsg == "" {
							errMsg = "voice recognition error"
						}
						sendEvent(Event{Type: EventError, Error: errMsg})
					}
					continue
				}
			}
			plainLines = append(plainLines, line)
		}
	}

	close(scanStop)
	sess.abortInput()
	if ctx.Err() != nil {
		_ = stdoutR.Close()
		_ = stderrR.Close()
	}
	if !exited {
		waitErr = <-waitCh
	}
	_ = stdoutR.Close()
	<-scanDone
	<-sess.writerDone

	stderrTimer := time.NewTimer(stdoutDrainTimeout)
	select {
	case <-stderrDone:
		stderrTimer.Stop()
	case <-stderrTimer.C:
		_ = stderrR.Close()
		<-stderrDone
	}
	_ = stderrR.Close()

	readyTimer.Stop()
	if ctx.Err() != nil && !sawFinal && !sawError {
		return
	}
	if sawError {
		return
	}
	if sawFinal {
		sendEvent(Event{Type: EventFinal, Text: strings.Join(finals, "")})
		return
	}
	if waitErr == nil {
		sendEvent(Event{Type: EventFinal, Text: strings.Join(plainLines, "\n")})
		return
	}
	errMsg := lastNonEmptyLine(stderrBuf.String())
	if errMsg == "" {
		errMsg = waitErr.Error()
	}
	sendEvent(Event{Type: EventError, Error: errMsg})
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			if len(line) > 240 {
				return line[:240]
			}
			return line
		}
	}
	return ""
}

type limitedBuffer struct {
	mu    sync.Mutex
	buf   *bytes.Buffer
	limit int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	rem := l.limit - l.buf.Len()
	if rem > 0 {
		if len(p) > rem {
			l.buf.Write(p[:rem])
		} else {
			l.buf.Write(p)
		}
	}
	return len(p), nil
}
