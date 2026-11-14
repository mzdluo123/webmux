package pluginapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/codeview"
	"github.com/rainchan/webmux/internal/session"
)

func TestCodeRawStreamingOutlivesLookupTimeout(t *testing.T) {
	// Keep the shortened controller deadline private to its owning package.
	previous := rawLookupTimeout
	rawLookupTimeout = time.Second
	t.Cleanup(func() { rawLookupTimeout = previous })

	cwd := t.TempDir()
	manager := session.NewManager(session.Options{DefaultCwd: cwd})
	t.Cleanup(func() { manager.Shutdown(context.Background()) })
	item, err := manager.Create(session.CreateInput{Cwd: cwd, Cmd: []string{"/bin/sh"}})
	if err != nil {
		t.Fatal(err)
	}
	codes := codeview.New(manager, nil)
	t.Cleanup(codes.Close)
	data := bytes.Repeat([]byte("PK\x03\x04streamed archive bytes\x00"), 1<<20)
	if err := os.WriteFile(filepath.Join(cwd, "bundle.zip"), data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	view, err := codes.Context(ctx, item.Info().ID, "")
	if err != nil {
		t.Fatal(err)
	}
	file, err := codes.File(ctx, item.Info().ID, view.ID, base64.RawURLEncoding.EncodeToString([]byte("bundle.zip")))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	Bind(Deps{Manager: manager, Code: codes}).codeRoutes(mux)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	response, err := server.Client().Get(server.URL + "/api/sessions/" + item.Info().ID + "/code/snapshots/" + file.Snapshot + "/raw")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("raw status: %d", response.StatusCode)
	}
	first := make([]byte, 1024)
	if _, err := io.ReadFull(response.Body, first); err != nil {
		t.Fatal(err)
	}
	// The remaining payload exceeds socket buffering, keeping the transfer
	// active until after its lookup deadline has passed.
	time.Sleep(2 * rawLookupTimeout)
	rest, err := io.ReadAll(response.Body)
	if err != nil || !bytes.Equal(append(first, rest...), data) {
		t.Fatalf("slow read truncated: %d of %d bytes (%v)", len(first)+len(rest), len(data), err)
	}
}

func TestContentDispositionPreservesSafeNamesAndRejectsUnsafeNames(t *testing.T) {
	for _, disposition := range []string{"inline", "attachment"} {
		for _, row := range []struct {
			name string
			want string
		}{
			{"pic.png", "pic.png"},
			{"a b.png", "a b.png"},
			{"图.png", "图.png"},
			{"", "download"},
			{"tab\there", "download"},
			{"del\x7f.png", "download"},
			{"\xff.png", "download"},
			{"bad\r\nInjected: true", "download"},
		} {
			header := contentDisposition(disposition, row.name)
			kind, params, err := mime.ParseMediaType(header)
			if err != nil || kind != disposition || params["filename"] != row.want {
				t.Errorf("%s %q: %q (%v)", disposition, row.name, header, err)
			}
		}
	}
}
