package worktreefs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteRegularAndRemoveEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	initial := []byte("hello\nworld\n")
	sum1, err := WriteRegular(root, "pkg/app.go", initial, false, true, "", 1<<20)
	if err != nil {
		t.Fatalf("WriteRegular initial: %v", err)
	}
	wantSum := sha256.Sum256(initial)
	if sum1 != hex.EncodeToString(wantSum[:]) {
		t.Fatalf("digest = %s, want %x", sum1, wantSum)
	}

	// External modification must fail with ErrStale.
	if err := os.WriteFile(filepath.Join(root, "pkg/app.go"), []byte("external\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteRegular(root, "pkg/app.go", []byte("next\n"), false, true, sum1, 1<<20); !errors.Is(err, ErrStale) {
		t.Fatalf("expected ErrStale, got %v", err)
	}

	// Symlink traversal must fail with ErrSymlink.
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteRegular(root, "escape/secret.txt", []byte("no"), false, false, "", 1<<20); !errors.Is(err, ErrSymlink) {
		t.Fatalf("expected ErrSymlink on directory symlink, got %v", err)
	}

	// Forbidden .git path must fail with ErrInvalid.
	if _, err := WriteRegular(root, ".git/config", []byte("no"), false, false, "", 1<<20); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid on .git path, got %v", err)
	}

	// RemoveEntry cleans up the file and is idempotent on repeat.
	if err := RemoveEntry(root, "pkg/app.go"); err != nil {
		t.Fatalf("RemoveEntry: %v", err)
	}
	if err := RemoveEntry(root, "pkg/app.go"); err != nil {
		t.Fatalf("RemoveEntry repeat: %v", err)
	}
}
