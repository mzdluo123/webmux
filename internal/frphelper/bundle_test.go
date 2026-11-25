//go:build webmux_frpc && (linux || darwin) && (amd64 || arm64)

package frphelper

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterialize(t *testing.T) {
	if err := Available(); err != nil {
		t.Fatal(err)
	}
	dir := newScopeDir(t)
	path, err := Materialize(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, binaryName); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if err := Verify(path); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]os.FileMode{binaryName: 0o500, licenseName: 0o600} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != want {
			t.Fatalf("%s mode = %s, want regular %o", name, info.Mode(), want)
		}
	}
	binary, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	license, err := os.ReadFile(filepath.Join(dir, licenseName))
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyBundle(binary, license); err != nil {
		t.Fatal(err)
	}
	var version string
	for line := range strings.Lines(pins) {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "version" {
			version = fields[1]
			break
		}
	}
	got, err := exec.CommandContext(t.Context(), path, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if version == "" || strings.TrimSpace(string(got)) != version {
		t.Fatalf("frpc --version = %q, want %q", got, version)
	}
	if !errors.Is(verifyBundle([]byte("modified frpc"), license), ErrUnavailable) ||
		!errors.Is(verifyBundle(binary, []byte("modified license")), ErrUnavailable) {
		t.Fatal("modified release payload was accepted")
	}
}

func TestMaterializeRejectsUnsafeDestination(t *testing.T) {
	if _, err := Materialize("relative"); err == nil {
		t.Fatal("relative directory was accepted")
	}
	insecure := newScopeDir(t)
	if err := os.Chmod(insecure, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Materialize(insecure); err == nil {
		t.Fatal("non-private directory was accepted")
	}
	dir := newScopeDir(t)
	foreign := filepath.Join(t.TempDir(), "foreign")
	if err := os.WriteFile(foreign, []byte("foreign"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, filepath.Join(dir, binaryName)); err != nil {
		t.Fatal(err)
	}
	if _, err := Materialize(dir); !errors.Is(err, os.ErrExist) {
		t.Fatalf("materialize symlink error = %v, want ErrExist", err)
	}
	got, err := os.ReadFile(foreign)
	if err != nil || string(got) != "foreign" {
		t.Fatalf("foreign executable modified: contents = %q, err = %v", got, err)
	}
}

func TestVerifyRejectsChangedHelper(t *testing.T) {
	t.Run("changed binary", func(t *testing.T) {
		path, err := Materialize(newScopeDir(t))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0o700); err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		_, err = file.WriteAt([]byte{bundledBinary[0] ^ 1}, 0)
		if err := errors.Join(err, file.Close()); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, binaryMode); err != nil {
			t.Fatal(err)
		}
		if err := Verify(path); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("Verify modified binary = %v, want ErrUnavailable", err)
		}
	})
	t.Run("missing license", func(t *testing.T) {
		dir := newScopeDir(t)
		path, err := Materialize(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(dir, licenseName)); err != nil {
			t.Fatal(err)
		}
		if err := Verify(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Verify missing license = %v, want ErrNotExist", err)
		}
	})
}

func newScopeDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(t.TempDir(), "scope-")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
