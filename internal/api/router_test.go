package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/wire"
)

func TestProtocolAndSecurity(t *testing.T) {
	c, err := config.Load(t.TempDir()+"/absent", false)
	if err != nil {
		t.Fatal(err)
	}
	c.Token = "secret"
	c.DefaultCwd = t.TempDir()
	if err = c.Validate(); err != nil {
		t.Fatal(err)
	}
	m := session.NewManager(testSessionOptions(c))
	server := httptest.NewServer(New(testDeps(t, testServices{Config: c, Manager: m, Workspace: testWorkspace(t, m, c.DefaultCwd)})))
	defer server.Close()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	}()
	request := func(method, path, body, token, origin string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	for _, row := range []struct {
		token, origin string
		want          int
	}{{"", "", 401}, {"wrong", "", 401}, {"secret", "http://evil.example", 403}, {"secret", "", 200}} {
		res := request("GET", "/api/health", "", row.token, row.origin)
		_ = res.Body.Close()
		if res.StatusCode != row.want {
			t.Fatalf("status %d want %d", res.StatusCode, row.want)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dial := func(path string) *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+path, &websocket.DialOptions{Subprotocols: tokenProtocols("secret")})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.CloseNow() })
		return conn
	}
	list := dial("/ws/sessions")
	_, raw, err := list.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		Type     string
		Sessions []contract.SessionInfo
	}
	if err = json.Unmarshal(raw, &snapshot); err != nil || snapshot.Type != "snapshot" || snapshot.Sessions == nil {
		t.Fatalf("empty snapshot: %s", raw)
	}
	res := request("POST", "/api/sessions", `{"cmd":["/bin/sh"]}`, "secret", "")
	if res.StatusCode != 201 {
		t.Fatal(res.Status)
	}
	var created contract.CreateSessionResult
	_ = json.NewDecoder(res.Body).Decode(&created)
	_ = res.Body.Close()
	info := created.Session
	res = request("DELETE", "/api/sessions/"+info.ID+"?remove=1", "", "secret", "")
	_ = res.Body.Close()
	if res.StatusCode != 409 {
		t.Fatal("running removal must conflict")
	}
	conn := dial("/ws/s/" + info.ID)
	readPosition(ctx, t, conn, "reset")
	kind, p, err := conn.Read(ctx)
	if err != nil || kind != websocket.MessageBinary || p[0] != 1 {
		t.Fatalf("initial frame %v %v %v", kind, p, err)
	}
	if err = conn.Write(ctx, websocket.MessageBinary, contract.Frame(0, []byte("printf 'FINAL_OUTPUT_MARKER\\n'; exit 6\n"))); err != nil {
		t.Fatal(err)
	}
	for {
		_, p, err = conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if p[0] == 3 {
			break
		}
	}
	again := dial("/ws/s/" + info.ID)
	readPosition(ctx, t, again, "reset")
	_, p, err = again.Read(ctx)
	if err != nil || p[0] != 1 || !strings.Contains(string(p[1:]), "FINAL_OUTPUT_MARKER") {
		t.Fatalf("final replay %q %v", p, err)
	}
	for {
		_, p, err = again.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if p[0] == 3 {
			if string(p[1:]) != `{"exitCode":6}` {
				t.Fatal(string(p))
			}
			break
		}
	}
	_, _, err = again.Read(ctx)
	if websocket.CloseStatus(err) != 4410 {
		t.Fatalf("exit close: %v", err)
	}
	res = request("GET", "/api/sessions/"+info.ID+"/buffer", "", "secret", "")
	buffer, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if !strings.Contains(string(buffer), "FINAL_OUTPUT_MARKER") {
		t.Fatal("buffer lost output")
	}
	res = request("DELETE", "/api/sessions/"+info.ID+"?remove=1", "", "secret", "")
	_ = res.Body.Close()
	if res.StatusCode != 204 {
		t.Fatal(res.Status)
	}
}

func TestCustomCommandExecutesLiteralArgvInSelectedDirectory(t *testing.T) {
	cwd := t.TempDir()
	script := "#!/bin/sh\nprintf 'cwd=%s\\n' \"$PWD\"\nprintf '<%s>\\n' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(cwd, "report"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	m := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	}()
	cmd := []string{"./report", "two words", `"quoted value"`, "$HOME", "*", "$(touch sentinel)", "; exit 9"}
	body, err := json.Marshal(map[string]any{"cwd": cwd, "cmd": cmd, "name": "My command"})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	New(testDeps(t, testServices{Config: config.Config{}, Manager: m, Workspace: testWorkspace(t, m, cwd)})).ServeHTTP(recorder, httptest.NewRequest("POST", "/api/sessions", strings.NewReader(string(body))))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	info := createdInfo(t, recorder.Body.Bytes())
	if info.Cwd != cwd || info.Title != "My command" || !reflect.DeepEqual(info.Cmd, cmd) {
		t.Fatalf("custom session info: %+v", info)
	}
	item, err := m.Get(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	sub, err := item.Attach()
	if err != nil {
		t.Fatal(err)
	}
	defer item.Detach(sub)
	deadline := time.After(5 * time.Second)
	for {
		select {
		case m := <-sub.Out:
			if m.Exit == nil {
				continue
			}
			info = wire.Session(item.Info())
			if info.ExitCode == nil || *info.ExitCode != 0 {
				t.Fatalf("custom command exit: %+v", info)
			}
			want := "cwd=" + cwd + "\r\n"
			for _, arg := range cmd[1:] {
				want += "<" + arg + ">\r\n"
			}
			if got := string(item.Buffer()); got != want {
				t.Fatalf("PTY output = %q, want %q", got, want)
			}
			if _, err := os.Stat(filepath.Join(cwd, "sentinel")); !os.IsNotExist(err) {
				t.Fatalf("shell expansion created sentinel: %v", err)
			}
			return
		case <-deadline:
			t.Fatal("custom PTY failed to exit")
		}
	}
}

func TestSessionCreationRejectsCommandConflicts(t *testing.T) {
	m := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	}()
	handler := New(testDeps(t, testServices{Config: config.Config{}, Manager: m, Workspace: testWorkspace(t, m, t.TempDir())}))
	for _, body := range []string{
		`{"cmd":[]}`,
		`{"cmd":[""]}`,
		`{}`,
		`{"cmd":["/definitely/missing/webmux-executable"]}`,
	} {
		t.Run(body, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest("POST", "/api/sessions", strings.NewReader(body)))
			var response map[string]string
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != http.StatusBadRequest || response["error"] == "" {
				t.Fatalf("status %d, body %v", recorder.Code, response)
			}
		})
	}
	if got := m.List(); len(got) != 0 {
		t.Fatalf("invalid requests created sessions: %v", got)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("POST", "/api/sessions", strings.NewReader(`{"cmd":["/bin/echo","two words","$HOME"]}`)))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("command with arguments: status %d: %s", recorder.Code, recorder.Body.String())
	}
	info := createdInfo(t, recorder.Body.Bytes())
	if !reflect.DeepEqual(info.Cmd, []string{"/bin/echo", "two words", "$HOME"}) {
		t.Fatalf("command arguments: %+v", info)
	}
}

func TestSessionRename(t *testing.T) {
	m := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	}()
	s, err := m.Create(session.CreateInput{Cmd: []string{"/bin/sh", "-c", "sleep 30"}})
	if err != nil {
		t.Fatal(err)
	}
	id := s.Info().ID
	handler := New(testDeps(t, testServices{Config: config.Config{}, Manager: m, Workspace: testWorkspace(t, m, t.TempDir())}))
	patch := func(path, body, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PATCH", path, strings.NewReader(body))
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	if rec := patch("/api/sessions/"+id, `{"name":"x"}`, "http://evil.example"); rec.Code != http.StatusForbidden {
		t.Fatalf("foreign origin: status %d", rec.Code)
	}
	if got := s.Info().Title; got == "x" {
		t.Fatal("rejected origin still renamed the session")
	}
	rec := patch("/api/sessions/"+id, `{"name":" 发布检查 "}`, "")
	var info contract.SessionInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &info); rec.Code != http.StatusOK || err != nil || info.Title != "发布检查" {
		t.Fatalf("rename: status %d %s", rec.Code, rec.Body.String())
	}
	for _, tc := range []struct {
		path, body string
		want       int
	}{
		{"/api/sessions/" + id, `{"name":"a\nb"}`, http.StatusBadRequest},
		{"/api/sessions/" + id, `{"name":"x","extra":1}`, http.StatusBadRequest},
		{"/api/sessions/" + id, `{"name":"x"} {}`, http.StatusBadRequest},
		{"/api/sessions/zzzzzzzz", `{"name":"x"}`, http.StatusNotFound},
	} {
		if rec := patch(tc.path, tc.body, ""); rec.Code != tc.want {
			t.Errorf("%s %s: status %d, want %d", tc.path, tc.body, rec.Code, tc.want)
		}
	}
	if got := s.Info().Title; got != "发布检查" {
		t.Fatalf("a rejected rename changed the title to %q", got)
	}
}

// An ordinary shell's rename must not depend on the hybrid record store: a
// closed store only disables hybrid sessions.
func TestSessionRenameIgnoresUnavailableHybridStore(t *testing.T) {
	m := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	}()
	s, err := m.Create(session.CreateInput{Cmd: []string{"/bin/sh", "-c", "sleep 30"}})
	if err != nil {
		t.Fatal(err)
	}
	hybrids := hybrid.New(hybrid.Options{StateDir: t.TempDir(), Manager: m})
	hybrids.Close()
	handler := New(testDeps(t, testServices{Config: config.Config{}, Manager: m, Workspace: testWorkspace(t, m, t.TempDir()), Hybrid: hybrids}))
	req := httptest.NewRequest("PATCH", "/api/sessions/"+s.Info().ID, strings.NewReader(`{"name":"plain"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || s.Info().Title != "plain" {
		t.Fatalf("rename with closed hybrid store: status %d title %q %s", rec.Code, s.Info().Title, rec.Body.String())
	}
}
