// Package agenttrust pre-accepts the "do you trust this folder" dialog of CLI
// agents before WebMux starts them in a directory.
//
// A permission-bypass flag such as --dangerously-skip-permissions does not
// skip Claude Code's folder-trust dialog, and a session nobody is looking at
// waits on that dialog forever. Claude Code records an accepted dialog as
// projects[<dir>].hasTrustDialogAccepted in its global config file, so the
// same record is written ahead of the launch. Only a directory the user chose
// for a session is trusted; the home directory, its ancestors and the
// filesystem root are refused because Claude Code's check also covers every
// directory beneath a trusted one.
package agenttrust

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	claudeConfigName = ".claude.json"
	trustKey         = "hasTrustDialogAccepted"
)

// Trust writes trust records for agents launched in a directory.
type Trust struct {
	mu sync.Mutex
	// home returns the user's home directory; tests replace it.
	home func() (string, error)
	// getenv reads the server's environment; tests replace it.
	getenv func(string) string
}

// New returns a Trust that uses the process home directory and environment.
func New() *Trust {
	return &Trust{home: os.UserHomeDir, getenv: os.Getenv}
}

// Prepare is the session pre-launch hook. It recognises Claude Code in argv
// and trusts cwd for it. env holds the session's explicit variables, which
// take precedence over the server's when locating the config directory.
// Failures are logged and never block the launch: the agent then simply shows
// its dialog.
func (t *Trust) Prepare(cwd string, argv []string, env map[string]string) {
	if !runsClaude(argv) {
		return
	}
	if err := t.trustClaude(cwd, env); err != nil {
		// Paths are not secrets, but terminal data never reaches this log.
		slog.Warn("claude folder trust not prepared", "dir", cwd, "error", err)
	}
}

// runsClaude reports whether argv starts Claude Code, directly or through a
// startup-file shell such as bash -ic 'exec -- "$@"' webmux claude.
func runsClaude(argv []string) bool {
	for _, token := range argv {
		for _, word := range strings.Fields(token) {
			if filepath.Base(word) == "claude" {
				return true
			}
		}
	}
	return false
}

func (t *Trust) trustClaude(cwd string, env map[string]string) error {
	home, err := t.home()
	if err != nil {
		return fmt.Errorf("home directory: %w", err)
	}
	dir, err := trustable(cwd, home)
	if err != nil {
		return err
	}
	configDir := env["CLAUDE_CONFIG_DIR"]
	if configDir == "" {
		configDir = t.getenv("CLAUDE_CONFIG_DIR")
	}
	path := filepath.Join(home, claudeConfigName)
	if configDir != "" {
		path = filepath.Join(configDir, claudeConfigName)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return setTrusted(path, dir)
}

// trustable returns the canonical form of cwd, or an error when trusting it
// would cover more than a project directory.
func trustable(cwd, home string) (string, error) {
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

// setTrusted marks dir as trusted in the Claude config at path, keeping every
// other field. A missing file is left alone: Claude Code then runs its first
// time setup, which includes the dialog, and creating the file would skip it.
func setTrusted(path, dir string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	updated, changed, err := withTrusted(data, dir)
	if err != nil {
		return fmt.Errorf("update %s: %w", path, err)
	}
	if !changed {
		return nil
	}
	return replaceFile(path, updated)
}

// withTrusted returns data with projects[dir].hasTrustDialogAccepted set to
// true. changed is false when the record is already there.
func withTrusted(data []byte, dir string) (updated []byte, changed bool, err error) {
	root, err := objectOf(data)
	if err != nil {
		return nil, false, err
	}
	projects, err := objectOf(root["projects"])
	if err != nil {
		return nil, false, fmt.Errorf("projects: %w", err)
	}
	entry, err := objectOf(projects[dir])
	if err != nil {
		return nil, false, fmt.Errorf("projects[%q]: %w", dir, err)
	}
	if bytes.Equal(entry[trustKey], []byte("true")) {
		return data, false, nil
	}
	entry[trustKey] = json.RawMessage("true")
	if projects[dir], err = json.Marshal(entry); err != nil {
		return nil, false, err
	}
	if root["projects"], err = json.Marshal(projects); err != nil {
		return nil, false, err
	}
	var out bytes.Buffer
	encoded, err := json.Marshal(root)
	if err != nil {
		return nil, false, err
	}
	if err = json.Indent(&out, encoded, "", "  "); err != nil {
		return nil, false, err
	}
	out.WriteByte('\n')
	return out.Bytes(), true, nil
}

// objectOf decodes a JSON object into its raw members; empty input is an
// empty object and any other JSON type is an error.
func objectOf(raw []byte) (map[string]json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return map[string]json.RawMessage{}, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("not a JSON object: %w", err)
	}
	return object, nil
}

// replaceFile swaps path for data through a temporary file in the same
// directory, keeping the original permissions, so a crash never leaves
// partial JSON in the agent's config.
func replaceFile(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".webmux-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Chmod(info.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
