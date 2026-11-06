package agenttrust

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func newTrust(home string, env map[string]string) *Trust {
	return &Trust{
		home:   func() (string, error) { return home, nil },
		getenv: func(k string) string { return env[k] },
	}
}

func canonical(t *testing.T, path string) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func readProjects(t *testing.T, path string) map[string]map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Projects map[string]map[string]any `json:"projects"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON %q: %v", data, err)
	}
	return doc.Projects
}

func TestPrepareTrustsClaudeWorkdirKeepingOtherFields(t *testing.T) {
	home := t.TempDir()
	work := filepath.Join(home, "proj")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(home, ".claude.json")
	seed := `{"numStartups": 7, "projects": {"/other": {"allowedTools": ["Bash"]}, "` + canonical(t, work) + `": {"lastCost": 1.5}}}`
	if err := os.WriteFile(config, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}
	newTrust(home, nil).Prepare(work, []string{"claude", "--dangerously-skip-permissions"}, nil)

	projects := readProjects(t, config)
	entry := projects[canonical(t, work)]
	if entry["hasTrustDialogAccepted"] != true || entry["lastCost"] != 1.5 {
		t.Fatalf("entry = %v", entry)
	}
	if projects["/other"]["allowedTools"] == nil {
		t.Fatalf("other project lost: %v", projects)
	}
	var top map[string]any
	data, _ := os.ReadFile(config)
	if err := json.Unmarshal(data, &top); err != nil || top["numStartups"] != float64(7) {
		t.Fatalf("top level = %v, %v", top, err)
	}
	info, err := os.Stat(config)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, %v", info.Mode(), err)
	}
}

func TestPrepareRecognisesWrappedAndPathedClaude(t *testing.T) {
	for _, argv := range [][]string{
		{"bash", "-ic", `exec -- "$@"`, "webmux", "claude", "--dangerously-skip-permissions"},
		{"/home/u/.local/bin/claude"},
		{"zsh", "-ic", `exec claude "$@"`, "claude"},
	} {
		home := t.TempDir()
		work := filepath.Join(home, "p")
		if err := os.Mkdir(work, 0o755); err != nil {
			t.Fatal(err)
		}
		config := filepath.Join(home, ".claude.json")
		if err := os.WriteFile(config, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		newTrust(home, nil).Prepare(work, argv, nil)
		if readProjects(t, config)[canonical(t, work)]["hasTrustDialogAccepted"] != true {
			t.Errorf("argv %q not trusted", argv)
		}
	}
}

func TestPrepareIgnoresOtherCommands(t *testing.T) {
	home := t.TempDir()
	work := filepath.Join(home, "p")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(config, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	newTrust(home, nil).Prepare(work, []string{"/bin/sh", "-l", "-i"}, nil)
	newTrust(home, nil).Prepare(work, []string{"codex"}, nil)
	if data, _ := os.ReadFile(config); string(data) != "{}" {
		t.Fatalf("config changed: %s", data)
	}
}

func TestPrepareHonoursClaudeConfigDir(t *testing.T) {
	home := t.TempDir()
	work := filepath.Join(home, "p")
	fromEnv := filepath.Join(home, "envcfg")
	fromServer := filepath.Join(home, "servercfg")
	for _, dir := range []string{work, fromEnv, fromServer} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".claude.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	trust := newTrust(home, map[string]string{"CLAUDE_CONFIG_DIR": fromServer})
	trust.Prepare(work, []string{"claude"}, map[string]string{"CLAUDE_CONFIG_DIR": fromEnv})
	if readProjects(t, filepath.Join(fromEnv, ".claude.json"))[canonical(t, work)]["hasTrustDialogAccepted"] != true {
		t.Fatal("session CLAUDE_CONFIG_DIR not used")
	}
	trust.Prepare(work, []string{"claude"}, nil)
	if readProjects(t, filepath.Join(fromServer, ".claude.json"))[canonical(t, work)]["hasTrustDialogAccepted"] != true {
		t.Fatal("server CLAUDE_CONFIG_DIR not used")
	}
}

func TestPrepareNeverCreatesOrBreaksConfig(t *testing.T) {
	home := t.TempDir()
	work := filepath.Join(home, "p")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	trust := newTrust(home, nil)
	trust.Prepare(work, []string{"claude"}, nil)
	if _, err := os.Stat(filepath.Join(home, ".claude.json")); !os.IsNotExist(err) {
		t.Fatalf("config created: %v", err)
	}
	config := filepath.Join(home, ".claude.json")
	for _, bad := range []string{"not json", `[]`, `{"projects": 3}`} {
		if err := os.WriteFile(config, []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		trust.Prepare(work, []string{"claude"}, nil)
		if data, _ := os.ReadFile(config); string(data) != bad {
			t.Fatalf("invalid config %q rewritten to %q", bad, data)
		}
	}
	entries, _ := os.ReadDir(home)
	for _, e := range entries {
		if e.Name() != "p" && e.Name() != ".claude.json" {
			t.Fatalf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestPrepareRefusesBroadDirectories(t *testing.T) {
	parent := t.TempDir()
	home := filepath.Join(parent, "home")
	if err := os.Mkdir(home, 0o755); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(config, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	trust := newTrust(home, nil)
	for _, dir := range []string{home, parent, "/"} {
		trust.Prepare(dir, []string{"claude"}, nil)
	}
	if data, _ := os.ReadFile(config); string(data) != "{}" {
		t.Fatalf("broad directory trusted: %s", data)
	}
}

func TestPrepareIsIdempotentAndLeavesTrustedFileUntouched(t *testing.T) {
	home := t.TempDir()
	work := filepath.Join(home, "p")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(home, ".claude.json")
	seed := `{"projects":{"` + canonical(t, work) + `":{"hasTrustDialogAccepted":true}}}`
	if err := os.WriteFile(config, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}
	newTrust(home, nil).Prepare(work, []string{"claude"}, nil)
	if data, _ := os.ReadFile(config); string(data) != seed {
		t.Fatalf("file rewritten: %s", data)
	}
}
