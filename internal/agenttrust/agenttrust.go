// Package agenttrust pre-accepts the "do you trust this folder" dialog of CLI
// agents before WebMux starts them in a directory.
//
// A permission-bypass flag such as --dangerously-skip-permissions does not
// skip the folder-trust dialog of Claude Code, Codex, Copilot CLI,
// Antigravity CLI, Qoder, QoderCN or Kimi Code CLI, and a session nobody is
// looking at waits on that dialog forever. Most agents record an accepted
// dialog in their own config file, so the same record is written ahead of the
// launch. Kimi instead receives its official per-process trust override.
//
// Only a directory the user chose for a session is trusted. The home
// directory, its ancestors and the filesystem root are refused because the
// agents' checks also cover every directory beneath a trusted one. A failure
// is logged and never blocks the launch: the agent then shows its dialog.
package agenttrust

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rainchan/webmux/internal/agentprofile/trustconfig"
)

// Trust writes trust records for agents launched in a directory.
type Trust struct {
	mu       sync.Mutex
	policies []trustconfig.Policy
	// home returns the user's home directory; tests replace it.
	home func() (string, error)
	// getenv reads the server's environment; tests replace it.
	getenv func(string) string
	// lookupEnv distinguishes an unset variable from an explicitly empty one.
	lookupEnv func(string) (string, bool)
}

// New selects only the explicitly assembled trust capabilities. An empty set
// performs no trust effects, independently of the runtime agent registry.
func New(policies ...trustconfig.Policy) *Trust {
	return &Trust{home: os.UserHomeDir, getenv: os.Getenv, lookupEnv: os.LookupEnv, policies: policies}
}

// Prepare is the session pre-launch hook. It recognises a supported agent in
// argv and trusts cwd for it. env holds the session's explicit variables,
// which take precedence over the server's when locating the config directory.
// The returned environment is used for the launch and may contain a scoped
// trust override for agents that support one.
func (t *Trust) Prepare(cwd string, argv []string, env map[string]string) map[string]string {
	a, ok := t.recognise(argv)
	if !ok {
		return env
	}
	if env == nil {
		env = make(map[string]string)
	}
	dir, err := t.trustable(cwd)
	if err == nil {
		t.mu.Lock()
		var plan trustconfig.Plan
		plan, err = a.Plan(trustconfig.Context{
			Home: t.home,
			Env:  func(key string) string { return t.env(env, key) },
			LookupEnv: func(key string) (string, bool) {
				if value, ok := env[key]; ok {
					return value, true
				}
				return t.lookupEnv(key)
			},
		}, dir)
		if err == nil {
			err = execute(plan, env)
		}
		t.mu.Unlock()
	}
	if err != nil {
		// Paths are not secrets; terminal data never reaches this log.
		slog.Warn("agent folder trust not prepared", "agent", a.Name, "dir", cwd, "error", err)
	}
	return env
}

// recognise finds the first supported agent in argv, whether it is argv[0] or
// the program of a startup-file shell such as bash -ic 'exec -- "$@"' webmux
// claude. It over-matches harmlessly: the directory is one the user is
// launching a command in anyway.
func (t *Trust) recognise(argv []string) (trustconfig.Policy, bool) {
	for _, token := range argv {
		for _, word := range strings.Fields(token) {
			base := filepath.Base(word)
			for _, a := range t.policies {
				if a.Match(base) {
					return a, true
				}
			}
		}
	}
	return trustconfig.Policy{}, false
}

// env returns a variable from the session's environment, falling back to the
// server's.
func (t *Trust) env(env map[string]string, key string) string {
	if v := env[key]; v != "" {
		return v
	}
	return t.getenv(key)
}

// trustable returns the canonical form of cwd, or an error when trusting it
// would cover more than a project directory.
func (t *Trust) trustable(cwd string) (string, error) {
	home, err := t.home()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	dir, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve directory: %w", err)
	}
	if home, err = filepath.EvalSymlinks(home); err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	dir = filepath.Clean(dir)
	if dir == filepath.Dir(dir) || dir == home || strings.HasPrefix(home, dir+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing to trust %s: it contains the home directory", dir)
	}
	return dir, nil
}

// execute owns every trust effect; adapters only describe pure edits.
func execute(plan trustconfig.Plan, env map[string]string) error {
	if plan.Path != "" {
		parent := filepath.Dir(plan.Path)
		if plan.RequireParent {
			if info, err := os.Stat(parent); err != nil || !info.IsDir() {
				return nil
			}
		}
		if plan.CreateParents {
			if err := os.MkdirAll(parent, 0o700); err != nil {
				return err
			}
		}
		if err := editFile(plan.Path, plan.Create, plan.Edit); err != nil {
			return err
		}
	}
	for key, value := range plan.Environment {
		env[key] = value
	}
	return nil
}

// editFile applies edit to the file at path through a temporary file and a
// rename, keeping the permissions, so a crash never leaves partial content in
// an agent's config. A missing file is left alone unless create is set; edit
// then receives nil. An edit that reports no change writes nothing.
func editFile(path string, create bool, edit func(data []byte) ([]byte, bool, error)) error {
	// Agent configs are often symlinks into a dotfiles repository. Rewrite the
	// target, because renaming over the link would replace it with a regular
	// file, and leave a dangling link alone.
	if _, err := os.Lstat(path); err == nil {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", path, err)
		}
		path = resolved
	}
	data, err := os.ReadFile(path)
	mode := fs.FileMode(0o600)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if !create {
			return nil
		}
		data = nil
	case err != nil:
		return fmt.Errorf("read %s: %w", path, err)
	default:
		info, statErr := os.Stat(path)
		if statErr != nil {
			return statErr
		}
		mode = info.Mode().Perm()
	}
	updated, changed, err := edit(data)
	if err != nil {
		return fmt.Errorf("update %s: %w", path, err)
	}
	if !changed {
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".webmux-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(updated); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
