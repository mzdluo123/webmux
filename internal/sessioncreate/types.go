// Package sessioncreate coordinates session creation without optional services.
package sessioncreate

import (
	"context"
	"errors"
	"time"

	"github.com/rainchan/webmux/internal/session"
)

const Timeout = 3 * time.Minute
const (
	StepPrecheck = "precheck"
	StepWorktree = "worktree"
	StepSession  = "session"
	StepConfig   = "config"
	StepPrompt   = "prompt"
)

var ErrUnavailable = errors.New("requested creation capability is disabled or unavailable")
var ErrLocationConflict = errors.New("creation location conflict")

type Reference struct{ RepositoryID, WorktreeID string }
type NewWorktree struct{ RepositoryID, Mode, Branch, Base, Path string }
type Input struct {
	ClientRequestID, Cwd             string
	Workspace                        *Reference
	NewWorktree                      *NewWorktree
	Cmd                              []string
	Env                              map[string]string
	ConfigID                         *string
	Name, Side                       string
	Prompt                           string
	UseInitInstruction, ExpandPrompt bool
	Attachments                      []string
	AgentConfig                      map[string]string
	PermissionMode                   string
	Cols, Rows                       int
}
type HandoffIdentity struct {
	RuntimeID, ProviderThreadID, ExecutionID string
	RequireNativeResume                      bool
}
type HandoffPlan struct {
	Cwd, RepositoryID, WorktreeID, Path, Branch, Base string
}

// Location is the immutable outcome of a location contribution. Detail is a
// typed projection port implemented by the contribution, never a service lookup.
type Location struct {
	RepositoryID, WorktreeID, Path string
	Detail                         LocationDetail
}

// LocationDetail permits controllers to recover their plugin-owned snapshot.
// The marker prevents arbitrary values from entering creation outcomes.
type LocationDetail interface{ CreatedLocation() }
type Result struct {
	Session        *session.Session
	Location       *Location
	ConfigWarnings []string
	InitialInput   *session.InputResult
}
type Error struct {
	Step      string
	Location  *Location
	SessionID string
	Err       error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

type Command struct {
	Cmd       []string
	Env       map[string]string
	ConfigID  string
	Chat      bool
	Attention *session.AttentionPolicy
}
type Commands interface {
	Resolve(context.Context, string) (Command, error)
}

// LocationPlan is checked before any create step. Acquire protects location
// identity across process startup and returns a release callback.
type LocationPlan struct {
	Cwd, Instruction string
	Create           func(context.Context) (*Location, error)
	Acquire          func(context.Context, string) (string, func(), error)
	Associate        func(*session.Session)
}
type Locations interface {
	Prepare(context.Context, Input) (LocationPlan, error)
}
type Files interface {
	StagedPath(string) (string, error)
	Adopt(string, []string) ([]string, error)
	Ensure(string) (string, error)
}
type Chat interface {
	Check(context.Context, Input, Command, string) error
	Create(context.Context, Input, Command, string, func(string) (string, error)) (*session.Session, bool, error)
	Configure(context.Context, string, Input) ([]string, error)
	Prompt(context.Context, string, string, string, string, []string) (session.InputResult, error)
}
type Options struct {
	DefaultCwd string
	Commands   Commands
	Locations  Locations
	Files      Files
	Chat       Chat
	Brief      func(string, string) string
}

// directoryError keeps the directory diagnostic and filesystem cause while
// classifying the request as invalid for transport error mapping.
type directoryError struct{ err error }

func (e *directoryError) Error() string        { return "directory: " + e.err.Error() }
func (e *directoryError) Unwrap() error        { return e.err }
func (e *directoryError) Is(target error) bool { return target == session.ErrInvalid }
