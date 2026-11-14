package pluginapi

import (
	"encoding/json"
	"fmt"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/files"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDirectoryPickerListsAllDirectories(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	names := []string{"webmux", "zz directory"}
	for i := range 25 {
		names = append(names, fmt.Sprintf(".cache-%02d", i))
	}
	for _, name := range names {
		if err := os.Mkdir(filepath.Join(root, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "a-linked")); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "ordinary-file")
	if err := os.WriteFile(file, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"file-link": file, "broken-link": filepath.Join(root, "missing")} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	allNames := append([]string{"a-linked"}, names...)
	wantAll := make([]string, 0, len(allNames))
	for _, name := range allNames {
		wantAll = append(wantAll, filepath.Join(root, name))
	}
	server := &Server{Deps: Deps{Config: config.Config{DefaultCwd: root}, Files: files.New(root)}}
	for _, row := range []struct {
		name, prefix string
		want         []string
	}{
		{"default", "", wantAll},
		{"trailing slash", root + "/", wantAll},
		{"home", "~", wantAll},
		{"home slash", "~/", wantAll},
		{"partial name", root + "/web", []string{filepath.Join(root, "webmux")}},
		{"home partial name", "~/web", []string{filepath.Join(root, "webmux")}},
		{"hidden prefix", root + "/.cache-2", wantAll[len(wantAll)-5:]},
		{"no match", root + "/absent", []string{}},
		{"symlink navigation", root + "/a-linked/", []string{}},
	} {
		t.Run(row.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			server.dirs(recorder, httptest.NewRequest("GET", "/api/fs/dirs?prefix="+url.QueryEscape(row.prefix), nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
			}
			var got []string
			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, row.want) {
				t.Fatalf("directories = %v, want %v", got, row.want)
			}
		})
	}
	for _, prefix := range []string{root + "/missing/", file + "/"} {
		recorder := httptest.NewRecorder()
		server.dirs(recorder, httptest.NewRequest("GET", "/api/fs/dirs?prefix="+url.QueryEscape(prefix), nil))
		var body map[string]string
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if recorder.Code != http.StatusBadRequest || body["error"] == "" {
			t.Fatalf("directory error: status %d, body %v", recorder.Code, body)
		}
	}
}
