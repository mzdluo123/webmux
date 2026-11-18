package pathutil

import (
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

func TestSniffContentType(t *testing.T) {
	tests := []struct {
		name string
		give []byte
		want string
	}{
		{name: "avif major", give: []byte("\x00\x00\x00\x18ftypavif\x00\x00\x00\x00mif1"), want: "image/avif"},
		{name: "avif compat", give: append([]byte{0, 0, 0, 0x1c}, []byte("ftypmif1\x00\x00\x00\x00mif1avifmiaf")...), want: "image/avif"},
		{name: "avis compat", give: append([]byte{0, 0, 0, 0x18}, []byte("ftypmif1\x00\x00\x00\x00mif1avis")...), want: "image/avif"},
		{name: "other ftyp", give: []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"), want: "video/mp4"},
		{name: "png", give: []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), want: "image/png"},
		{name: "html", give: []byte("<!DOCTYPE html><html></html>"), want: "text/html"},
		{name: "xml", give: []byte(`<?xml version="1.0"?><svg/>`), want: "text/xml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SniffContentType(tt.give); got != tt.want {
				t.Fatalf("SniffContentType(%s) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
