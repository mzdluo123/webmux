package pathutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTemporaryDirectoryRefusesReplacedRoot(t *testing.T) {
	t.Parallel()
	parent, foreign := t.TempDir(), t.TempDir()
	handle, err := MakeTemporaryDirectory(parent, "owned-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "preserved"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	moved := handle.Path() + "-moved"
	if err := os.Rename(handle.Path(), moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, handle.Path()); err != nil {
		t.Fatal(err)
	}
	if err := handle.Remove(); !errors.Is(err, ErrTemporaryDirectoryUnowned) {
		t.Fatal("cleanup accepted a replacement root")
	}
	if _, err := os.Stat(filepath.Join(foreign, "preserved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(handle.Path()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(moved, handle.Path()); err != nil {
		t.Fatal(err)
	}
	if err := handle.Remove(); err != nil {
		t.Fatal(err)
	}
	if err := handle.Remove(); err != nil {
		t.Fatalf("repeated cleanup: %v", err)
	}
}
