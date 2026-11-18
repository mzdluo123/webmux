package relaysetting

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/statefile"
)

func TestMachineLazyStableAndPrivate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	path := filepath.Join(dir, "relay-machine.json")
	machine := OpenMachine(dir)
	defer machine.Close()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("opening created an identity: %v", err)
	}
	id, err := machine.ID()
	if err != nil || id.Validate() != nil {
		t.Fatalf("create identity: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			got, err := machine.ID()
			if err != nil || got != id {
				t.Errorf("identity changed: %v", err)
			}
		})
	}
	wg.Wait()
	if runtime.GOOS != "windows" {
		for path, want := range map[string]os.FileMode{dir: 0700, path: 0600, path + ".lock": 0600} {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != want {
				t.Fatalf("%s permissions = %o, want %o", path, info.Mode().Perm(), want)
			}
		}
	}
	machine.Close()
	if _, err := machine.ID(); !statefile.IsKind(err, statefile.Closed) {
		t.Fatalf("closed identity still available: %v", err)
	}
	reopened := OpenMachine(dir)
	defer reopened.Close()
	got, err := reopened.ID()
	if err != nil || got != id {
		t.Fatalf("reopening changed identity: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("reading rewrote identity: %v", err)
	}
}

func TestMachineRejectsMalformedWithoutReplacement(t *testing.T) {
	id := strings.Repeat("A", 43)
	for name, contents := range map[string]string{
		"invalid-json":    "{",
		"missing-version": `{"machineId":"` + id + `"}`,
		"future-version":  `{"v":2,"machineId":"` + id + `"}`,
		"missing-id":      `{"v":1}`,
		"empty-id":        `{"v":1,"machineId":""}`,
		"short-id":        `{"v":1,"machineId":"short"}`,
		"noncanonical-id": `{"v":1,"machineId":"` + strings.Repeat("A", 42) + `B"}`,
		"unknown-field":   `{"v":1,"machineId":"` + id + `","seed":"secret"}`,
		"duplicate-field": `{"v":1,"machineId":"` + id + `","machineId":"` + id + `"}`,
		"wrong-case":      `{"v":1,"MachineId":"` + id + `"}`,
		"trailing-value":  `{"v":1,"machineId":"` + id + `"} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "relay-machine.json")
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			machine := OpenMachine(dir)
			defer machine.Close()
			for range 2 {
				if got, err := machine.ID(); err == nil || got != "" {
					t.Fatal("malformed identity allowed pairing")
				}
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != contents {
				t.Fatalf("malformed identity was replaced: %v", err)
			}
		})
	}
}

func TestMachineLockFailureCannotCreateReplacement(t *testing.T) {
	dir := t.TempDir()
	first := OpenMachine(dir)
	defer first.Close()
	second := OpenMachine(dir)
	defer second.Close()
	if id, err := second.ID(); id != "" || !statefile.IsKind(err, statefile.Locked) {
		t.Fatalf("second owner did not fail closed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "relay-machine.json")); !os.IsNotExist(err) {
		t.Fatalf("lock conflict created identity: %v", err)
	}
	id, err := first.ID()
	if err != nil {
		t.Fatal(err)
	}
	first.Close()
	if _, err := second.ID(); !statefile.IsKind(err, statefile.Locked) {
		t.Fatalf("failed owner silently recovered: %v", err)
	}
	third := OpenMachine(dir)
	defer third.Close()
	if got, err := third.ID(); err != nil || got != id {
		t.Fatalf("lock conflict replaced original identity: %v", err)
	}
}

func TestMachineFailedCommitStaysUnavailable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	machine := OpenMachine(dir)
	defer machine.Close()
	moved := dir + "-preserved"
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if id, err := machine.ID(); id != "" || err == nil {
		t.Fatal("uncommitted identity escaped")
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(moved, dir); err != nil {
		t.Fatal(err)
	}
	if id, err := machine.ID(); id != "" || err == nil {
		t.Fatal("failed initialization silently generated a replacement")
	}
	if _, err := os.Stat(filepath.Join(dir, "relay-machine.json")); !os.IsNotExist(err) {
		t.Fatalf("failed initialization left an identity: %v", err)
	}
}

func TestMachineReadsExistingCanonicalIdentity(t *testing.T) {
	dir := t.TempDir()
	want := protocol.MachineID(strings.Repeat("A", 43))
	contents := `{"v":1,"machineId":"` + string(want) + `"}`
	if err := os.WriteFile(filepath.Join(dir, "relay-machine.json"), []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	machine := OpenMachine(dir)
	defer machine.Close()
	if got, err := machine.ID(); err != nil || got != want {
		t.Fatalf("existing identity was not reused: %v", err)
	}
}
