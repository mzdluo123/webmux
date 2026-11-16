package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/artifacts"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/relaysetting"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/transport"
)

// uploadAPI serves one live session whose artifacts folders live under
// root; uploads is where that session's uploads must land.
func uploadAPI(t *testing.T) (handler http.Handler, m *session.Manager, item *session.Session, root, uploads string) {
	t.Helper()
	root = t.TempDir()
	cwd := t.TempDir()
	m = session.NewManager(session.Options{DefaultCwd: cwd})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	})
	item, err := m.Create(session.CreateInput{Cwd: cwd, Cmd: []string{"/bin/sh"}, Name: "upload", Env: nil, ConfigID: ""})
	if err != nil {
		t.Fatal(err)
	}
	folders := artifacts.New(artifacts.Options{TempDir: root, StateDir: t.TempDir(), Manager: m})
	folders.Start(func(string) bool { return false })
	t.Cleanup(folders.Close)
	folder, err := folders.Path(item.Info().ID)
	if err != nil {
		t.Fatal(err)
	}
	handler = New(testDeps(t, testServices{Config: config.Config{Token: "secret"}, Manager: m, Workspace: testWorkspace(t, m, cwd), Artifacts: folders}))
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil || !strings.HasPrefix(folder, canonical+string(filepath.Separator)) {
		t.Fatalf("folder %q outside %q (%v)", folder, root, err)
	}
	return handler, m, item, root, filepath.Join(folder, artifacts.UploadsDir)
}

func uploadBody(t *testing.T, name string, data []byte, extra string) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": name}))
	header.Set("Content-Type", "application/octet-stream")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if extra != "" {
		if err := writer.WriteField(extra, "extra"); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func uploadRequest(item *session.Session, body io.Reader, contentType string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "http://webmux.example/api/sessions/"+item.Info().ID+"/upload", body)
	r.Header.Set("Authorization", "Bearer secret")
	r.Header.Set("Content-Type", contentType)
	return r
}

func requireUploadError(t *testing.T, w *httptest.ResponseRecorder, status int, root string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status %d, want %d: %s", w.Code, status, w.Body.String())
	}
	var result struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Error == "" {
		t.Fatalf("missing contextual JSON error: %s (%v)", w.Body.String(), err)
	}
	// Directories may have been prepared; no file may remain.
	var files []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return err
	})
	if err != nil || len(files) != 0 {
		t.Fatalf("failed upload left files: %v (%v)", files, err)
	}
}

// Uploads land in the session's private uploads/ folder; a taken name gets a
// " (2)" suffix and never replaces the first file.
func TestUploadPersistsBinaryInSessionUploadsFolder(t *testing.T) {
	handler, _, item, _, uploads := uploadAPI(t)
	first := []byte{0, 1, 2, 255, '\n', '\r', 128}
	second := []byte("second upload")
	var paths []string
	for _, data := range [][]byte{first, second} {
		body, contentType := uploadBody(t, "same name.bin", data, "")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, uploadRequest(item, bytes.NewReader(body), contentType))
		if w.Code != http.StatusCreated {
			t.Fatalf("upload: %d %s", w.Code, w.Body.String())
		}
		var result struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		path := result.Path
		if filepath.Dir(path) != uploads {
			t.Fatalf("unexpected upload path %q, want it in %q", path, uploads)
		}
		stored, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(stored, data) {
			t.Fatalf("persisted bytes %v, want %v (%v)", stored, data, err)
		}
		for target, mode := range map[string]os.FileMode{path: 0600, uploads: 0700, filepath.Dir(uploads): 0700} {
			info, err := os.Stat(target)
			if err != nil || info.Mode().Perm() != mode {
				t.Fatalf("permissions for %s: %v (%v), want %o", target, info, err, mode)
			}
		}
		paths = append(paths, path)
	}
	if filepath.Base(paths[0]) != "same name.bin" || filepath.Base(paths[1]) != "same name (2).bin" {
		t.Fatalf("names %q", paths)
	}
	stored, err := os.ReadFile(paths[0])
	if err != nil || !bytes.Equal(stored, first) {
		t.Fatalf("second upload overwrote first: %v (%v)", stored, err)
	}
}

func requireUploadReadHeaders(t *testing.T, h http.Header, wantType, wantDisposition, wantFilename string, wantLen int) {
	t.Helper()
	if got := h.Get("Content-Type"); got != wantType {
		t.Fatalf("Content-Type %q, want %q", got, wantType)
	}
	if got := h.Get("Content-Length"); got != strconv.Itoa(wantLen) {
		t.Fatalf("Content-Length %q, want %d", got, wantLen)
	}
	if got := h.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control %q, want no-store", got)
	}
	if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options %q, want nosniff", got)
	}
	if got := h.Get("Content-Security-Policy"); got != "default-src 'none'; sandbox" {
		t.Fatalf("Content-Security-Policy %q, want default-src 'none'; sandbox", got)
	}
	kind, params, err := mime.ParseMediaType(h.Get("Content-Disposition"))
	if err != nil || kind != wantDisposition || params["filename"] != wantFilename {
		t.Fatalf("Content-Disposition %q (%v), want %s filename=%q", h.Get("Content-Disposition"), err, wantDisposition, wantFilename)
	}
}

func TestReadUploadServesSessionUploadedFileWithSniffedMediaType(t *testing.T) {
	handler, _, item, _, uploads := uploadAPI(t)
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")
	body, contentType := uploadBody(t, "shot.png", png, "")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, uploadRequest(item, bytes.NewReader(body), contentType))
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	var uploaded struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &uploaded); err != nil {
		t.Fatal(err)
	}

	readURL := "http://webmux.example/api/sessions/" + item.Info().ID + "/upload?path=" + url.QueryEscape(uploaded.Path)
	get := httptest.NewRequest(http.MethodGet, readURL, nil)
	get.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, get)
	if rec.Code != http.StatusOK {
		t.Fatalf("read upload: %d %s", rec.Code, rec.Body.String())
	}
	requireUploadReadHeaders(t, rec.Header(), "image/png", "inline", "shot.png", len(png))
	if !bytes.Equal(rec.Body.Bytes(), png) {
		t.Fatalf("body %v, want %v", rec.Body.Bytes(), png)
	}

	head := httptest.NewRequest(http.MethodHead, readURL, nil)
	head.Header.Set("Authorization", "Bearer secret")
	headRec := httptest.NewRecorder()
	handler.ServeHTTP(headRec, head)
	if headRec.Code != http.StatusOK || headRec.Body.Len() != 0 {
		t.Fatalf("HEAD read upload: %d len=%d", headRec.Code, headRec.Body.Len())
	}
	requireUploadReadHeaders(t, headRec.Header(), "image/png", "inline", "shot.png", len(png))

	for _, row := range []struct {
		name   string
		url    string
		auth   string
		origin string
		status int
	}{
		{"missing session", "/api/sessions/abcdefgh/upload?path=" + uploaded.Path, "Bearer secret", "", http.StatusNotFound},
		{"missing path", "/api/sessions/" + item.Info().ID + "/upload", "Bearer secret", "", http.StatusBadRequest},
		{"outside uploads", "/api/sessions/" + item.Info().ID + "/upload?path=" + filepath.Dir(uploads), "Bearer secret", "", http.StatusBadRequest},
		{"unauthenticated", "/api/sessions/" + item.Info().ID + "/upload?path=" + uploaded.Path, "", "", http.StatusUnauthorized},
		{"untrusted origin", "/api/sessions/" + item.Info().ID + "/upload?path=" + uploaded.Path, "Bearer secret", "https://evil.example", http.StatusForbidden},
	} {
		t.Run(row.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "http://webmux.example"+row.url, nil)
			if row.auth != "" {
				r.Header.Set("Authorization", row.auth)
			}
			if row.origin != "" {
				r.Header.Set("Origin", row.origin)
			}
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, r)
			if out.Code != row.status {
				t.Fatalf("status %d, want %d: %s", out.Code, row.status, out.Body.String())
			}
		})
	}
}

func TestReadUploadForcesAttachmentAndSandboxForNonRasterFiles(t *testing.T) {
	handler, _, item, _, uploads := uploadAPI(t)
	if err := os.MkdirAll(uploads, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(uploads), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(uploads, 0o700); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		giveName        string
		giveData        []byte
		wantContentType string
		wantDisposition string
	}{
		{
			giveName:        "photo.jpg",
			giveData:        []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'},
			wantContentType: "image/jpeg",
			wantDisposition: "inline",
		},
		{
			giveName:        "anim.gif",
			giveData:        []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
			wantContentType: "image/gif",
			wantDisposition: "inline",
		},
		{
			giveName:        "banner.webp",
			giveData:        []byte("RIFF\x0c\x00\x00\x00WEBPVP8 "),
			wantContentType: "image/webp",
			wantDisposition: "inline",
		},
		{
			giveName:        "bitmap.bmp",
			giveData:        []byte("BM\x00\x00\x00\x00"),
			wantContentType: "image/bmp",
			wantDisposition: "inline",
		},
		{
			giveName:        "favicon.ico",
			giveData:        []byte{0x00, 0x00, 0x01, 0x00, 0x01, 0x00},
			wantContentType: "image/x-icon",
			wantDisposition: "inline",
		},
		{
			giveName:        "report.html",
			giveData:        []byte("<!DOCTYPE html><html><body><script>fetch('/api/sessions',{method:'POST'})</script></body></html>"),
			wantContentType: "application/octet-stream",
			wantDisposition: "attachment",
		},
		{
			giveName:        "disguised.png",
			giveData:        []byte("<html><script>alert(1)</script></html>"),
			wantContentType: "application/octet-stream",
			wantDisposition: "attachment",
		},
		{
			giveName:        "payload.xml",
			giveData:        []byte(`<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><script>alert(1)</script></html>`),
			wantContentType: "application/octet-stream",
			wantDisposition: "attachment",
		},
		{
			giveName:        "chart-xml.svg",
			giveData:        []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
			wantContentType: "application/octet-stream",
			wantDisposition: "attachment",
		},
		{
			giveName:        "chart.svg",
			giveData:        []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
			wantContentType: "application/octet-stream",
			wantDisposition: "attachment",
		},
		{
			giveName:        "manual.pdf",
			giveData:        []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n"),
			wantContentType: "application/octet-stream",
			wantDisposition: "attachment",
		},
		{
			giveName:        "notes.txt",
			giveData:        []byte("plain text"),
			wantContentType: "application/octet-stream",
			wantDisposition: "attachment",
		},
		{
			giveName:        "empty.bin",
			giveData:        nil,
			wantContentType: "application/octet-stream",
			wantDisposition: "attachment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.giveName, func(t *testing.T) {
			target := filepath.Join(uploads, tt.giveName)
			if err := os.WriteFile(target, tt.giveData, 0o600); err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(http.MethodGet, "http://webmux.example/api/sessions/"+item.Info().ID+"/upload?path="+url.QueryEscape(target), nil)
			r.Header.Set("Authorization", "Bearer secret")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, r)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			requireUploadReadHeaders(t, rec.Header(), tt.wantContentType, tt.wantDisposition, tt.giveName, len(tt.giveData))
			if !bytes.Equal(rec.Body.Bytes(), tt.giveData) {
				t.Fatalf("body %q, want %q", rec.Body.Bytes(), tt.giveData)
			}
		})
	}
}

func TestReadUploadTopLevelNavigationBlocksScriptExecutionAcrossAuthModes(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	m := session.NewManager(session.Options{DefaultCwd: cwd})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	})
	item, err := m.Create(session.CreateInput{Cwd: cwd, Cmd: []string{"/bin/sh"}, Name: "nav"})
	if err != nil {
		t.Fatal(err)
	}
	folders := artifacts.New(artifacts.Options{TempDir: root, StateDir: t.TempDir(), Manager: m})
	folders.Start(func(string) bool { return false })
	t.Cleanup(folders.Close)
	uploads, err := folders.EnsureUploads(item.Info().ID)
	if err != nil {
		t.Fatal(err)
	}

	htmlPayload := []byte("<!DOCTYPE html><html><script>fetch('/api/sessions',{method:'POST'})</script></html>")
	xmlSVGPayload := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>fetch('/api/relay/auth/session')</script></svg>`)
	htmlPath := filepath.Join(uploads, "report.html")
	xmlPath := filepath.Join(uploads, "diagram.svg")
	if err := os.WriteFile(htmlPath, htmlPayload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xmlPath, xmlSVGPayload, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("local passwordless top-level GET", func(t *testing.T) {
		localHandler := New(testDeps(t, testServices{
			Config:    config.Config{Listen: "127.0.0.1:7681", Token: ""},
			Manager:   m,
			Workspace: testWorkspace(t, m, cwd),
			Artifacts: folders,
		}))
		for _, tt := range []struct {
			giveName string
			givePath string
			giveData []byte
		}{
			{giveName: "report.html", givePath: htmlPath, giveData: htmlPayload},
			{giveName: "diagram.svg", givePath: xmlPath, giveData: xmlSVGPayload},
		} {
			r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7681/api/sessions/"+item.Info().ID+"/upload?path="+url.QueryEscape(tt.givePath), nil)
			rec := httptest.NewRecorder()
			localHandler.ServeHTTP(rec, r)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s status %d: %s", tt.giveName, rec.Code, rec.Body.String())
			}
			requireUploadReadHeaders(t, rec.Header(), "application/octet-stream", "attachment", tt.giveName, len(tt.giveData))
		}
	})

	t.Run("relay owner browser top-level GET", func(t *testing.T) {
		public := &transport.PublicOrigins{}
		if err := public.Set("relay", "https://h.example.com"); err != nil {
			t.Fatal(err)
		}
		deps := testDeps(t, testServices{
			Config:    config.Config{Listen: "127.0.0.1:7681", Token: strings.Repeat("s", 43)},
			Manager:   m,
			Workspace: testWorkspace(t, m, cwd),
			Artifacts: folders,
		})
		deps.Public = public
		relayHandler := New(deps)
		for _, tt := range []struct {
			giveName string
			givePath string
			giveData []byte
		}{
			{giveName: "report.html", givePath: htmlPath, giveData: htmlPayload},
			{giveName: "diagram.svg", givePath: xmlPath, giveData: xmlSVGPayload},
		} {
			r := httptest.NewRequest(http.MethodGet, "https://h.example.com/api/sessions/"+item.Info().ID+"/upload?path="+url.QueryEscape(tt.givePath), nil)
			r = r.WithContext(RelayOwnerContext(r.Context(), relaysetting.AuthPrincipal{}))
			rec := httptest.NewRecorder()
			relayHandler.ServeHTTP(rec, r)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s status %d: %s", tt.giveName, rec.Code, rec.Body.String())
			}
			requireUploadReadHeaders(t, rec.Header(), "application/octet-stream", "attachment", tt.giveName, len(tt.giveData))
		}
	})
}

func TestUploadNormalizesTraversalFilenames(t *testing.T) {
	for _, name := range []string{"../../escape.bin", "/absolute/path/escape.bin", `..\..\escape.bin`, `C:\folder\escape.bin`} {
		t.Run(name, func(t *testing.T) {
			handler, _, item, _, uploads := uploadAPI(t)
			body, contentType := uploadBody(t, name, []byte("safe"), "")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, uploadRequest(item, bytes.NewReader(body), contentType))
			if w.Code != http.StatusCreated {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			var result map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result["path"] != filepath.Join(uploads, "escape.bin") {
				t.Fatalf("filename escaped temporary directory: %q", result["path"])
			}
		})
	}
}

func TestUploadRejectsUnsafeFilenames(t *testing.T) {
	for _, name := range []string{"", ".", "..", "/", `..\`, "bad\x00name", "bad\rname", "bad\nname", "bad\tname", "bad\x1bname", "bad\x7fname", "bad\u0085name", "unsafe\nparent/safe.bin"} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			handler, _, item, root, _ := uploadAPI(t)
			body, contentType := uploadBody(t, name, []byte("data"), "")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, uploadRequest(item, bytes.NewReader(body), contentType))
			requireUploadError(t, w, http.StatusBadRequest, root)
		})
	}
}

func TestUploadRejectsMalformedAndExtraMultipartParts(t *testing.T) {
	for _, name := range []string{"not multipart", "missing boundary", "empty multipart", "field instead of file", "wrong field", "extra field", "extra file", "truncated file", "truncated final boundary", "malformed extra part"} {
		t.Run(name, func(t *testing.T) {
			handler, _, item, root, _ := uploadAPI(t)
			body, contentType := uploadBody(t, "file.bin", []byte("data"), "")
			switch name {
			case "not multipart":
				contentType = "application/octet-stream"
			case "missing boundary":
				contentType = "multipart/form-data"
			case "empty multipart":
				body = []byte("--empty--\r\n")
				contentType = "multipart/form-data; boundary=empty"
			case "field instead of file":
				body = []byte("--field\r\nContent-Disposition: form-data; name=\"file\"\r\n\r\ndata\r\n--field--\r\n")
				contentType = "multipart/form-data; boundary=field"
			case "wrong field":
				body = bytes.Replace(body, []byte(`; name=file`), []byte(`; name=other`), 1)
			case "extra field":
				body, contentType = uploadBody(t, "file.bin", []byte("data"), "extra")
			case "extra file":
				body, contentType = uploadBody(t, "file.bin", []byte("data"), "extra")
				body = bytes.Replace(body, []byte(`name="extra"`), []byte(`name="file"; filename="second.bin"`), 1)
			case "truncated file":
				body = body[:bytes.Index(body, []byte("\r\n\r\ndata"))+len("\r\n\r\nda")]
			case "truncated final boundary":
				body = body[:len(body)-8]
			case "malformed extra part":
				body, contentType = uploadBody(t, "file.bin", []byte("data"), "extra")
				body = bytes.Replace(body, []byte("Content-Disposition: form-data; name=\"extra\""), []byte("not-a-valid-header"), 1)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, uploadRequest(item, bytes.NewReader(body), contentType))
			requireUploadError(t, w, http.StatusBadRequest, root)
		})
	}
}

// Generate arbitrarily large request bodies without allocating their contents.
// A nonzero byte also ensures no multipart boundary is accidentally generated.
type uploadGeneratedReader struct{}

func (uploadGeneratedReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

func TestUploadSizeLimitsRemovePartialFiles(t *testing.T) {
	// The public upload contract allows 100 MiB per file and 101 MiB total,
	// including multipart framing and any epilogue.
	const fileLimit, bodyLimit int64 = 100 << 20, 101 << 20
	for _, name := range []string{"oversized file", "oversized declared body", "oversized chunked epilogue"} {
		t.Run(name, func(t *testing.T) {
			handler, _, item, root, _ := uploadAPI(t)
			const boundary = "upload-size-boundary"
			header := "--" + boundary + "\r\nContent-Disposition: form-data; name=\"file\"; filename=\"large.bin\"\r\n\r\n"
			ending := "\r\n--" + boundary + "--\r\n"
			var body io.Reader
			if name == "oversized chunked epilogue" {
				body = io.MultiReader(strings.NewReader(header+"small"+ending), io.LimitReader(uploadGeneratedReader{}, bodyLimit))
			} else {
				body = io.MultiReader(strings.NewReader(header), io.LimitReader(uploadGeneratedReader{}, fileLimit+1), strings.NewReader(ending))
			}
			r := uploadRequest(item, body, "multipart/form-data; boundary="+boundary)
			if name == "oversized declared body" {
				r.ContentLength = bodyLimit + 1
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			requireUploadError(t, w, http.StatusRequestEntityTooLarge, root)
		})
	}
}

func TestUploadRequiresLiveSessionAndExistingSecurityMiddleware(t *testing.T) {
	for _, row := range []struct {
		name   string
		status int
	}{
		{"missing session", http.StatusNotFound},
		{"exited session", http.StatusConflict},
		{"missing authentication", http.StatusUnauthorized},
		{"wrong authentication", http.StatusUnauthorized},
		{"query authentication", http.StatusUnauthorized},
		{"untrusted origin", http.StatusForbidden},
	} {
		t.Run(row.name, func(t *testing.T) {
			handler, manager, item, root, _ := uploadAPI(t)
			body, contentType := uploadBody(t, "file.bin", []byte("data"), "")
			r := uploadRequest(item, bytes.NewReader(body), contentType)
			switch row.name {
			case "missing session":
				r.URL.Path = "/api/sessions/missing/upload"
			case "exited session":
				if err := manager.Close(item.Info().ID); err != nil {
					t.Fatal(err)
				}
				select {
				case <-item.Done():
				case <-time.After(7 * time.Second):
					t.Fatal("session did not exit")
				}
			case "missing authentication":
				r.Header.Del("Authorization")
			case "wrong authentication":
				r.Header.Set("Authorization", "Bearer wrong")
			case "query authentication":
				r.Header.Del("Authorization")
				r.URL.RawQuery = "token=secret"
			case "untrusted origin":
				r.Header.Set("Origin", "https://evil.example")
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			requireUploadError(t, w, row.status, root)
		})
	}
}

type uploadCancellingReader struct {
	reader io.Reader
	cancel context.CancelFunc
	read   int
}

func (r *uploadCancellingReader) Read(p []byte) (int, error) {
	if len(p) > 1024 {
		p = p[:1024]
	}
	n, err := r.reader.Read(p)
	r.read += n
	if r.read >= 8192 {
		r.cancel()
	}
	return n, err
}

func TestUploadCancellationRemovesPartialFile(t *testing.T) {
	handler, _, item, root, _ := uploadAPI(t)
	body, contentType := uploadBody(t, "cancelled.bin", bytes.Repeat([]byte("x"), 32<<10), "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := &uploadCancellingReader{reader: bytes.NewReader(body), cancel: cancel}
	r := uploadRequest(item, reader, contentType).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	requireUploadError(t, w, http.StatusBadRequest, root)
	if !strings.Contains(w.Body.String(), "context canceled") {
		t.Fatalf("missing cancellation context: %s", w.Body.String())
	}
}
