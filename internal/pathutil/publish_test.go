//go:build linux || darwin

package pathutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublishDirectoryNeverReplacesDestination(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"empty", "file", "symlink", "dangling symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
			if err := os.Mkdir(source, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "content"), []byte("new"), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "empty":
				err = os.Mkdir(target, 0700)
			case "file":
				err = os.WriteFile(target, []byte("existing"), 0600)
			case "symlink":
				err = os.Symlink(source, target)
			case "dangling symlink":
				err = os.Symlink(filepath.Join(root, "absent"), target)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(target)
			if err != nil {
				t.Fatal(err)
			}
			if err := PublishDirectory(source, target); err == nil {
				t.Fatal("replaced existing target")
			}
			after, err := os.Lstat(target)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("target changed: %v", err)
			}
			if err := os.Remove(target); err != nil {
				t.Fatal(err)
			}
			if err := PublishDirectory(source, target); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(target, "content"))
			if err != nil || string(data) != "new" {
				t.Fatalf("publication: %s %v", data, err)
			}
		})
	}
}
