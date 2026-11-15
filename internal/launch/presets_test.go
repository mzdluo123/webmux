package launch

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile/agents"
)

func TestPresetsBeforeSeedPreferShellAwareList(t *testing.T) {
	s := open(t, t.TempDir())
	if presets, err := s.Presets(); err != nil || len(presets) != 0 {
		t.Fatalf("presets before seed = %+v, %v", presets, err)
	}
	if err := s.Seed(Presets("/bin/zsh")); err != nil {
		t.Fatal(err)
	}
	presets, err := s.Presets()
	if err != nil || len(presets) != 14 || !slices.Equal(presets[0].Cmd, []string{"/bin/zsh", "-l", "-i"}) {
		t.Fatalf("presets after seed = %+v, %v", presets, err)
	}
}

func TestRestorePresets(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir, agents.Default())
	if err := s.Seed(Presets("/bin/zsh")); err != nil {
		t.Fatal(err)
	}
	presets, err := s.Presets()
	if err != nil || len(presets) != 14 || !slices.Equal(presets[0].Cmd, []string{"/bin/zsh", "-l", "-i"}) {
		t.Fatalf("presets = %+v, %v", presets, err)
	}
	// Editing a preset keeps its name; deleting others leaves them restorable.
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	var claudeID string
	for _, c := range list {
		if c.Name == "Claude Code" {
			claudeID = c.ID
			if _, err := s.Update(c.ID, Input{Name: "Claude Code", Icon: "claudecode", Cmd: []string{"claude", "--resume"}}); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := s.Delete(c.ID); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()

	// Reopening an existing file and calling Seed keeps user edits while
	// remembering the shell's presets for later restoration.
	s = open(t, dir)
	if err := s.Seed(Presets("/bin/zsh")); err != nil {
		t.Fatal(err)
	}
	restored, err := s.RestorePresets("omp")
	if err != nil || len(restored) != 2 || restored[0].Name != "Claude Code" || restored[1].Name != "omp" {
		t.Fatalf("restore omp = %+v, %v", restored, err)
	}
	ompCfg, err := s.Get(restored[1].ID)
	if err != nil || ompCfg.Env["PI_FORCE_IMAGE_PROTOCOL"] != "kitty" || !ompCfg.Hybrid {
		t.Fatalf("restored omp config = %+v, %v", ompCfg, err)
	}
	if _, err := s.RestorePresets("Claude Code"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("existing preset err = %v, want ErrDuplicate", err)
	}
	if _, err := s.RestorePresets("Unknown Preset"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown preset err = %v, want ErrNotFound", err)
	}

	all, err := s.RestorePresets("")
	if err != nil || len(all) != 14 {
		t.Fatalf("restore all = %+v, %v", all, err)
	}
	claudeCfg, err := s.Get(claudeID)
	if err != nil || !reflect.DeepEqual(claudeCfg.Cmd, []string{"claude", "--resume"}) {
		t.Fatalf("user-edited preset overwritten: %+v, %v", claudeCfg, err)
	}
	if !reflect.DeepEqual(all[2].Cmd, []string{"/bin/zsh", "-l", "-i"}) {
		t.Fatalf("restored Shell cmd = %q", all[2].Cmd)
	}
	// Restoring all when every preset already exists is a no-op.
	again, err := s.RestorePresets("")
	if err != nil || !reflect.DeepEqual(again, all) {
		t.Fatalf("restore all again = %+v, %v", again, err)
	}
}

func TestRestorePresetsRespectsMaxConfigs(t *testing.T) {
	s := open(t, t.TempDir())
	for i := range MaxConfigs - 1 {
		mustCreate(t, s, Input{Name: fmt.Sprintf("custom-%d", i), Cmd: []string{"sh"}})
	}
	if err := s.Seed(Presets("")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RestorePresets(""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("restore all over limit = %v, want ErrInvalid", err)
	}
	if _, err := s.RestorePresets("Shell"); err != nil {
		t.Fatalf("restore one at limit-1 = %v", err)
	}
	if _, err := s.RestorePresets("omp"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("restore one at limit = %v, want ErrInvalid", err)
	}
}
