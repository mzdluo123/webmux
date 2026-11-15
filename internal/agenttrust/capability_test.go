package agenttrust

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rainchan/webmux/internal/agent/copilot"
	"github.com/rainchan/webmux/internal/agentprofile/trustconfig"
)

func TestEmptyCapabilitiesLeaveLaunchUntouched(t *testing.T) {
	env := map[string]string{"existing": "value"}
	trust := New()
	trust.home = func() (string, error) { t.Fatal("empty capability set queried home"); return "", nil }
	got := trust.Prepare("/", []string{"copilot", "kimi", "claude"}, env)
	if len(got) != 1 || got["existing"] != "value" {
		t.Fatalf("environment changed: %v", got)
	}
}

func TestRecognitionUsesOnlyInjectedPoliciesAndPreservesWordScan(t *testing.T) {
	trust := New(copilot.Trust)
	for _, argv := range [][]string{
		{"/opt/bin/copilot"},
		{"bash", "-ic", "exec /opt/bin/copilot --yolo"},
	} {
		policy, ok := trust.recognise(argv)
		if !ok || policy.Name != "copilot" {
			t.Fatalf("recognise(%q) = %q, %v", argv, policy.Name, ok)
		}
	}
	if _, ok := trust.recognise([]string{"claude", "kimi"}); ok {
		t.Fatal("unregistered policy recognised")
	}
}

func TestConcurrentTrustEditsPreserveAllProjects(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(path, []byte(`{"unknown":{"keep":true}}`), 0o640); err != nil {
		t.Fatal(err)
	}
	trust := newTrust(home, nil)
	var wg sync.WaitGroup
	const projects = 24
	for i := range projects {
		work := filepath.Join(home, fmt.Sprintf("project-%d", i))
		if err := os.Mkdir(work, 0o700); err != nil {
			t.Fatal(err)
		}
		wg.Go(func() { trust.Prepare(work, []string{"claude"}, nil) })
	}
	wg.Wait()
	if got := len(readProjects(t, path)); got != projects {
		t.Fatalf("projects = %d, want %d", got, projects)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Unknown map[string]bool `json:"unknown"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if !doc.Unknown["keep"] {
		t.Fatalf("unknown field changed: %s", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("permissions = %v", info.Mode())
	}
}

func TestCapabilityFailureDoesNotBlockLaunch(t *testing.T) {
	home := t.TempDir()
	work := filepath.Join(home, "project")
	if err := os.Mkdir(work, 0o700); err != nil {
		t.Fatal(err)
	}
	trust := New(trustconfig.Policy{
		Name: "test", Match: func(base string) bool { return base == "test" },
		Plan: func(trustconfig.Context, string) (trustconfig.Plan, error) {
			return trustconfig.Plan{}, fmt.Errorf("unreadable configuration")
		},
	})
	trust.home = func() (string, error) { return home, nil }
	env := trust.Prepare(work, []string{"test"}, map[string]string{"keep": "yes"})
	if len(env) != 1 || env["keep"] != "yes" {
		t.Fatalf("failed trust changed launch: %v", env)
	}
}
