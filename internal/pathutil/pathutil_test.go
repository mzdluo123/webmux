package pathutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ in, want string }{
		{"~", home},
		{"~/work/repo", filepath.Join(home, "work", "repo")},
		{"/abs/../path", "/path"},
		{"relative", filepath.Join(cwd, "relative")},
		{"~user", filepath.Join(cwd, "~user")},
	} {
		if got, err := Expand(row.in); err != nil || got != row.want {
			t.Errorf("Expand(%q) = %q, %v; want %q", row.in, got, err, row.want)
		}
	}
}

func TestUploadPath(t *testing.T) {
	dir, err := os.MkdirTemp("", UploadDirPrefix)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hi"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := UploadPath(path); err != nil {
		t.Fatalf("UploadPath: %v", err)
	} else if !filepath.IsAbs(got) {
		t.Fatalf("UploadPath returned %q", got)
	}
	outside := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(outside, []byte("hi"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "relative", outside, dir, filepath.Join(dir, "missing")} {
		if _, err := UploadPath(bad); err == nil {
			t.Errorf("UploadPath(%q) accepted", bad)
		}
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := UploadPath(link); err == nil {
		t.Errorf("UploadPath accepted a symlink")
	}
}
