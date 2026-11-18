package web

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path"
	"testing"

	"github.com/andybalholm/brotli"
)

// Decode each embedded asset and compare it to the ordinary Vite output.
func TestEmbeddedAssetsMatchBuild(t *testing.T) {
	count := 0
	err := fs.WalkDir(os.DirFS("dist"), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || name == "embed.zip" || name == ".gitkeep" {
			return err
		}
		want, err := os.ReadFile(path.Join("dist", name))
		if err != nil {
			return err
		}
		count++

		raw, rawErr := fs.ReadFile(Assets, "dist/"+name)
		if _, err := fs.Stat(Assets, "dist/"+name+".gz"); err == nil {
			t.Errorf("redundant gzip copy of %s", name)
		}
		br, brErr := fs.ReadFile(Assets, "dist/"+name+".br")
		switch {
		case brErr == nil:
			if rawErr == nil {
				t.Errorf("duplicate identity copy of %s", name)
			}
			got, err := io.ReadAll(brotli.NewReader(bytes.NewReader(br)))
			if err != nil || !bytes.Equal(got, want) {
				t.Errorf("embedded %s.br differs from build: %v", name, err)
			}
		case rawErr == nil:
			if !bytes.Equal(raw, want) {
				t.Errorf("embedded asset %s differs from build", name)
			}
		default:
			t.Errorf("asset %s is missing from the embedded archive", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no built assets checked")
	}
	if _, err := Assets.Open("dist/embed.zip"); !os.IsNotExist(err) {
		t.Fatalf("archive contains itself: %v", err)
	}
	for _, sibling := range []string{".br", ".gz"} {
		if _, err := fs.Stat(Assets, "dist/embed.zip"+sibling); err == nil {
			t.Fatalf("archive contains its own %s sibling", sibling)
		}
	}
}
