//go:build !webmux_frpc || (!linux && !darwin) || (!amd64 && !arm64)

package frphelper

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUnbundledFrpcIsUnavailable(t *testing.T) {
	system := t.TempDir()
	if err := os.WriteFile(filepath.Join(system, binaryName), []byte("foreign executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", system)
	dir := t.TempDir()
	if err := Available(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Available error = %v, want ErrUnavailable", err)
	}
	if path, err := Materialize(dir); path != "" || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Materialize returned (%q, %v), want empty path and ErrUnavailable", path, err)
	}
	if err := Verify(filepath.Join(system, binaryName)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Verify error = %v, want ErrUnavailable", err)
	}
	if files, err := os.ReadDir(dir); err != nil || len(files) != 0 {
		t.Fatalf("unbundled build wrote files: %v, err = %v", files, err)
	}
}
