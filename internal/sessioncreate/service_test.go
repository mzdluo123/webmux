package sessioncreate

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/session"
)

func coreManager(t *testing.T) (*session.Manager, string) {
	t.Helper()
	dir := t.TempDir()
	manager := session.NewManager(session.Options{DefaultCwd: dir})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		manager.Shutdown(ctx)
	})
	return manager, dir
}
func waitOutput(t *testing.T, item *session.Session, text string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(string(item.Buffer()), text) {
		if time.Now().After(deadline) {
			t.Fatalf("missing %q in %q", text, item.Buffer())
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRawCreateIsCompleteAndIdempotentWithoutPlugins(t *testing.T) {
	manager, dir := coreManager(t)
	creator := coreCreator(t, manager, Options{DefaultCwd: dir})
	in := Input{ClientRequestID: "raw-create-001", Cwd: dir, Cmd: []string{"/bin/sh", "-c", `printf '%s|%s\n' "$PWD" "$RAW_VALUE" >> starts; printf READY; exec cat`}, Env: map[string]string{"RAW_VALUE": "explicit environment"}, Cols: 97, Rows: 31}
	const count = 8
	results := make([]Result, count)
	errs := make([]error, count)
	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func() { defer wg.Done(); results[i], errs[i] = creator.Create(context.Background(), in) }()
	}
	wg.Wait()
	for i := range count {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if results[i].Session != results[0].Session {
			t.Fatal("retry started another process")
		}
	}
	item := results[0].Session
	waitOutput(t, item, "READY")
	starts, err := os.ReadFile(filepath.Join(dir, "starts"))
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := Directory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if string(starts) != canonical+"|explicit environment\n" {
		t.Fatalf("raw cwd/env startup or deduplication: %q", starts)
	}
	info := item.Info()
	if info.Cwd != canonical || !reflect.DeepEqual(info.Cmd, in.Cmd) {
		t.Fatalf("raw process facts %+v", info)
	}
	receipt, err := item.Input(context.Background(), session.InputRequest{ClientRequestID: "raw-input-001", Text: "core-input", Submit: true})
	if err != nil || receipt.Side != "tui" {
		t.Fatalf("ordinary input %+v %v", receipt, err)
	}
	waitOutput(t, item, "core-input")
	if err := item.Interrupt(context.Background(), "raw-interrupt-001", "test"); err != nil {
		t.Fatal(err)
	}
}

func TestDisabledCreationFieldsFailBeforeSideEffects(t *testing.T) {
	manager, dir := coreManager(t)
	creator := coreCreator(t, manager, Options{DefaultCwd: dir})
	config := "saved-config"
	cases := []Input{
		{Workspace: &Reference{RepositoryID: "repo", WorktreeID: "tree"}},
		{NewWorktree: &NewWorktree{RepositoryID: "repo", Mode: "new", Branch: "branch"}},
		{ConfigID: &config}, {Side: "acp"}, {Attachments: []string{"staged"}, Prompt: "hello"},
		{AgentConfig: map[string]string{"model": "x"}}, {PermissionMode: "auto"}, {UseInitInstruction: true},
		{ExpandPrompt: true, Prompt: "{{dir}}"}, {ExpandPrompt: true, Prompt: "{{webmux}}"},
	}
	for i, in := range cases {
		in.Cmd = []string{"/bin/sh", "-c", "touch should-not-exist"}
		_, err := creator.Create(context.Background(), in)
		var failed *Error
		if !errors.As(err, &failed) || failed.Step != StepPrecheck || !errors.Is(err, ErrUnavailable) {
			t.Fatalf("case %d: %v", i, err)
		}
	}
	if len(manager.List()) != 0 {
		t.Fatal("rejected capabilities created a session")
	}
	if _, err := os.Stat(filepath.Join(dir, "should-not-exist")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected capabilities started a process: %v", err)
	}
	locations := &recordedLocations{dir: dir}
	creator = coreCreator(t, manager, Options{DefaultCwd: dir, Locations: locations})
	_, err := creator.Create(context.Background(), Input{ConfigID: &config, NewWorktree: &NewWorktree{RepositoryID: "repo", Mode: "new"}})
	if !errors.Is(err, ErrUnavailable) || len(locations.events) != 0 {
		t.Fatalf("disabled launch reached location contribution: %v %v", locations.events, err)
	}
}

func TestRawCreateRetentionExpires(t *testing.T) {
	manager, dir := coreManager(t)
	creator := coreCreator(t, manager, Options{DefaultCwd: dir})
	creator.retention = 20 * time.Millisecond
	in := Input{ClientRequestID: "retention-001", Cmd: []string{"/bin/sh", "-c", "exit 0"}}
	first, err := creator.Create(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		again, err := creator.Create(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		if again.Session != first.Session {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("completed creation never expired")
		}
		time.Sleep(time.Millisecond)
	}
}

type recordedLocations struct {
	dir    string
	events []string
}

func (l *recordedLocations) Prepare(_ context.Context, _ Input) (LocationPlan, error) {
	l.events = append(l.events, "location-precheck")
	path := filepath.Join(l.dir, "worktree")
	return LocationPlan{Cwd: l.dir, Create: func(context.Context) (*Location, error) {
		l.events = append(l.events, "worktree")
		if err := os.Mkdir(path, 0700); err != nil {
			return nil, err
		}
		return &Location{RepositoryID: "repo", WorktreeID: "tree", Path: path}, nil
	}}, nil
}

type fixedCommand struct{}

func (fixedCommand) Resolve(context.Context, string) (Command, error) {
	return Command{Cmd: []string{"/bin/cat"}, Chat: true}, nil
}

type recordedChat struct {
	manager  *session.Manager
	events   *[]string
	failStep string
}

func (c recordedChat) Check(context.Context, Input, Command, string) error {
	*c.events = append(*c.events, "chat-precheck")
	return nil
}
func (c recordedChat) Create(_ context.Context, in Input, command Command, cwd string, _ func(string) (string, error)) (*session.Session, bool, error) {
	*c.events = append(*c.events, "session")
	item, err := c.manager.Create(session.CreateInput{Cwd: cwd, Cmd: command.Cmd})
	return item, false, err
}
func (c recordedChat) Configure(context.Context, string, Input) ([]string, error) {
	*c.events = append(*c.events, "config")
	if c.failStep == StepConfig {
		return nil, errors.New("config failed")
	}
	return []string{"warning"}, nil
}
func (c recordedChat) Prompt(context.Context, string, string, string, string, []string) (session.InputResult, error) {
	*c.events = append(*c.events, "prompt")
	if c.failStep == StepPrompt {
		return session.InputResult{}, errors.New("prompt failed")
	}
	return session.InputResult{Side: "acp", Prompt: session.InputPromptResult{TurnID: "turn"}}, nil
}
func TestContributedCreatePreservesStepOrderAndPartialOutcome(t *testing.T) {
	for _, failure := range []string{"", StepConfig, StepPrompt} {
		t.Run(failure, func(t *testing.T) {
			manager, dir := coreManager(t)
			locations := &recordedLocations{dir: dir}
			chat := recordedChat{manager: manager, events: &locations.events, failStep: failure}
			creator := coreCreator(t, manager, Options{DefaultCwd: dir, Locations: locations, Commands: fixedCommand{}, Chat: chat})
			config := "config"
			result, err := creator.Create(context.Background(), Input{ClientRequestID: "ordered-create-001", ConfigID: &config, Side: "acp", NewWorktree: &NewWorktree{RepositoryID: "repo", Mode: "new"}, AgentConfig: map[string]string{"model": "chosen"}, Prompt: "first message"})
			want := []string{"location-precheck", "chat-precheck", "worktree", "session", "config"}
			if failure != StepConfig {
				want = append(want, "prompt")
			}
			if !reflect.DeepEqual(locations.events, want) {
				t.Fatalf("step order %v want %v", locations.events, want)
			}
			if failure == "" {
				if err != nil || result.Location == nil || result.InitialInput == nil || result.InitialInput.Prompt.TurnID != "turn" {
					t.Fatalf("created outcome %+v %v", result, err)
				}
				return
			}
			var failed *Error
			if !errors.As(err, &failed) || failed.Step != failure || failed.Location == nil || failed.SessionID == "" {
				t.Fatalf("partial outcome %v", err)
			}
			if _, err := manager.Get(failed.SessionID); err != nil {
				t.Fatal("partial failure removed session", err)
			}
			if stat, err := os.Stat(failed.Location.Path); err != nil || !stat.IsDir() {
				t.Fatal("partial failure removed worktree", err)
			}
		})
	}
}

func TestRawCreatePreservesStartupErrorKinds(t *testing.T) {
	manager, dir := coreManager(t)
	creator := coreCreator(t, manager, Options{DefaultCwd: dir})
	_, err := creator.Create(context.Background(), Input{Cmd: []string{filepath.Join(dir, "missing-command")}})
	var failed *Error
	if !errors.As(err, &failed) || failed.Step != StepSession || !errors.Is(err, session.ErrInvalid) {
		t.Fatalf("invalid command lost its kind or step: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	manager.Shutdown(ctx)
	_, err = creator.Create(context.Background(), Input{Cmd: []string{"/bin/cat"}})
	if !errors.As(err, &failed) || failed.Step != StepSession || !errors.Is(err, session.ErrShuttingDown) {
		t.Fatalf("shutdown lost its kind or step: %v", err)
	}
}

func TestDirectoryRetainsHomeAndSymlinkResolution(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, "project")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(home, "alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"~/project", alias} {
		got, err := Directory(input)
		if err != nil || got != target {
			t.Fatalf("directory %q: %q %v", input, got, err)
		}
	}
}

func TestLaunchOnlyRunsStoredChatPreferenceAsTerminal(t *testing.T) {
	manager, dir := coreManager(t)
	creator := coreCreator(t, manager, Options{DefaultCwd: dir, Commands: fixedCommand{}})
	config := "saved-chat-preference"
	result, err := creator.Create(context.Background(), Input{ConfigID: &config})
	if err != nil {
		t.Fatal(err)
	}
	if result.Session == nil || len(manager.List()) != 1 {
		t.Fatalf("launch-only did not create a terminal: %+v", result)
	}
	_, err = creator.Create(context.Background(), Input{ConfigID: &config, Side: "acp"})
	var failed *Error
	if !errors.As(err, &failed) || failed.Step != StepPrecheck || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("explicit chat request was not rejected: %v", err)
	}
	if len(manager.List()) != 1 {
		t.Fatal("explicit disabled chat created another process")
	}
}

func coreCreator(t *testing.T, manager *session.Manager, options Options) *Service {
	t.Helper()
	creator := New(manager, options)
	t.Cleanup(creator.Close)
	return creator
}

type cancellingLocation struct {
	started   chan struct{}
	cancelled chan error
	release   chan struct{}
}

func (l *cancellingLocation) Prepare(_ context.Context, in Input) (LocationPlan, error) {
	return LocationPlan{Cwd: in.Cwd, Create: func(ctx context.Context) (*Location, error) {
		command := exec.CommandContext(ctx, "/bin/sh", "-c", "exec sleep 30")
		if err := command.Start(); err != nil {
			return nil, err
		}
		close(l.started)
		err := command.Wait()
		l.cancelled <- err
		<-l.release
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}}, nil
}

func TestCloseCancelsDetachedCreateAndJoinsCleanup(t *testing.T) {
	manager, dir := coreManager(t)
	location := &cancellingLocation{started: make(chan struct{}), cancelled: make(chan error, 1), release: make(chan struct{})}
	creator := coreCreator(t, manager, Options{DefaultCwd: dir, Locations: location})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(location.release) }) }
	t.Cleanup(release)
	client, cancelClient := context.WithCancel(context.Background())
	cancelClient()
	detached := context.WithoutCancel(client)
	in := Input{ClientRequestID: "shutdown-create-001", Cwd: dir, Cmd: []string{"/bin/cat"}}
	result := make(chan error, 1)
	go func() { _, err := creator.Create(detached, in); result <- err }()
	select {
	case <-location.started:
	case <-time.After(5 * time.Second):
		t.Fatal("location subprocess never started")
	}
	duplicate := make(chan error, 1)
	go func() { _, err := creator.Create(detached, in); duplicate <- err }()
	closed := make(chan struct{})
	go func() { creator.Close(); close(closed) }()
	select {
	case err := <-location.cancelled:
		if err == nil {
			t.Fatal("shutdown did not interrupt the running location subprocess")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("host shutdown did not cancel detached creation")
	}
	select {
	case <-closed:
		t.Fatal("Close returned before contribution cleanup")
	default:
	}
	release()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not join creation")
	}
	var failed *Error
	err := <-result
	if !errors.As(err, &failed) || failed.Step != StepWorktree || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled create lost its step or cause: %v", err)
	}
	select {
	case err := <-duplicate:
		if !errors.Is(err, context.Canceled) && !errors.Is(err, session.ErrShuttingDown) {
			t.Fatalf("duplicate waiter survived shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("duplicate waiter was not released")
	}
	if len(manager.List()) != 0 {
		t.Fatal("cancelled location continued to process startup")
	}
	for _, request := range []Input{in, {Cmd: []string{"/bin/cat"}}} {
		_, err := creator.Create(context.Background(), request)
		if !errors.Is(err, session.ErrShuttingDown) {
			t.Fatalf("late creation accepted after Close: %v", err)
		}
	}
}

func TestCloseClearsRetainedOutcomesWithoutClosingPTY(t *testing.T) {
	manager, dir := coreManager(t)
	creator := coreCreator(t, manager, Options{DefaultCwd: dir})
	in := Input{ClientRequestID: "shutdown-retained-001", Cmd: []string{"/bin/cat"}}
	result, err := creator.Create(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	creator.mu.Lock()
	retained := creator.flights[in.ClientRequestID]
	timerPresent := retained != nil && retained.timer != nil
	creator.mu.Unlock()
	if !timerPresent {
		t.Fatal("completed request did not retain its outcome")
	}
	creator.Close()
	creator.mu.Lock()
	remaining := len(creator.flights)
	creator.mu.Unlock()
	if remaining != 0 {
		t.Fatal("Close retained finished request timers and outcomes")
	}
	if _, err := manager.Get(result.Session.Info().ID); err != nil {
		t.Fatal("creation shutdown prematurely closed the PTY host", err)
	}
	_, err = creator.Create(context.Background(), in)
	if !errors.Is(err, session.ErrShuttingDown) {
		t.Fatalf("closed service replayed a retained outcome: %v", err)
	}
}

func TestCloseCancelsInitialTerminalPromptProducer(t *testing.T) {
	manager, dir := coreManager(t)
	creator := coreCreator(t, manager, Options{DefaultCwd: dir})
	result, err := creator.Create(context.Background(), Input{Cmd: []string{"/bin/sh", "-c", "exec sleep 30"}, Prompt: "must not outlive creation service"})
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan struct{})
	go func() { creator.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close left terminal first-input producer waiting for readiness")
	}
	if result.Session.Info().Status == session.StatusExited {
		t.Fatal("creation shutdown closed the PTY instead of just its producer")
	}
	if len(result.Session.Buffer()) != 0 {
		t.Fatal("cancelled first-input producer wrote after shutdown")
	}
}
