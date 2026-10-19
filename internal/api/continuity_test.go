package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/session"
)

type continuityFixture struct {
	t      *testing.T
	ctx    context.Context
	server *httptest.Server
	item   *session.Session
}

// recorder runs a raw, non-echoing PTY that records its input in a file.
func recorder(path string) []string {
	return []string{"/bin/bash", "-c", `stty raw -echo; printf READY; exec cat > "$0"`, path}
}

func newContinuityFixture(t *testing.T, argv []string) *continuityFixture {
	t.Helper()
	presets := []session.Preset{{Name: "record", Cmd: argv}}
	m := session.NewManager(session.Options{DefaultCwd: t.TempDir(), Presets: presets})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(func() {
		cancel()
		shutdown, done := context.WithTimeout(context.Background(), 8*time.Second)
		defer done()
		m.Shutdown(shutdown)
	})
	item, err := m.Create("", "record", nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(config.Config{Presets: presets}, m, testWorkspace(t, m, t.TempDir())))
	t.Cleanup(server.Close)
	until := time.Now().Add(5 * time.Second)
	for !bytes.Contains(item.Buffer(), []byte("READY")) {
		if time.Now().After(until) {
			t.Fatal("recorder did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return &continuityFixture{t: t, ctx: ctx, server: server, item: item}
}

func (f *continuityFixture) dial(query string) *websocket.Conn {
	f.t.Helper()
	conn, _, err := websocket.Dial(f.ctx, "ws"+strings.TrimPrefix(f.server.URL, "http")+"/ws/s/"+f.item.Info().ID+"?"+query, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}

// next returns the next frame of the given type, skipping everything else.
func (f *continuityFixture) next(conn *websocket.Conn, kind byte) []byte {
	f.t.Helper()
	for {
		_, p, err := conn.Read(f.ctx)
		if err != nil {
			f.t.Fatalf("waiting for frame %d: %v", kind, err)
		}
		if p[0] == kind {
			return p[1:]
		}
	}
}

func (f *continuityFixture) control(conn *websocket.Conn) session.ControlState {
	f.t.Helper()
	var state session.ControlState
	if err := json.Unmarshal(f.next(conn, 6), &state); err != nil {
		f.t.Fatal(err)
	}
	return state
}

func (f *continuityFixture) status(conn *websocket.Conn) session.SubmissionStatus {
	f.t.Helper()
	var status session.SubmissionStatus
	if err := json.Unmarshal(f.next(conn, 5), &status); err != nil {
		f.t.Fatal(err)
	}
	return status
}

func (f *continuityFixture) send(conn *websocket.Conn, kind byte, p []byte) {
	f.t.Helper()
	if err := conn.Write(f.ctx, websocket.MessageBinary, session.Frame(kind, p)); err != nil {
		f.t.Fatal(err)
	}
}

func submitFrame(id, generation, body, submit string) []byte {
	header, _ := json.Marshal(map[string]any{"id": id, "generation": generation, "bodyBytes": len(body)})
	return append(append(header, '\n'), body+submit...)
}

func waitFile(t *testing.T, path, want string) {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for {
		got, _ := os.ReadFile(path)
		if string(got) == want {
			return
		}
		if time.Now().After(until) {
			t.Fatalf("recorded input %q, want %q", got, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestResumeAppendsOnlyMissingOutput(t *testing.T) {
	f := newContinuityFixture(t, []string{"/bin/bash", "-c", `stty -echo; printf READY; read -r line; printf "after-%s" "$line"; sleep 30`})
	generation := f.item.Info().Generation
	first := f.dial("client=device-one")
	position := readPosition(f.ctx, t, first, "reset")
	if position.Generation != generation || position.Start != 0 || position.End != uint64(len(f.item.Buffer())) || position.Truncated {
		t.Fatalf("initial position: %+v", position)
	}
	_ = first.CloseNow()

	before := len(f.item.Buffer())
	// Output produced while no browser is attached.
	tail := []byte("after-disconnect")
	if err := f.item.Write([]byte("disconnect\n")); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(3 * time.Second)
	for !bytes.HasSuffix(f.item.Buffer(), tail) {
		if time.Now().After(until) {
			t.Fatalf("output %q", f.item.Buffer())
		}
		time.Sleep(10 * time.Millisecond)
	}
	conn := f.dial("client=device-one&generation=" + generation + "&offset=" + strconv.Itoa(before))
	resumed := readPosition(f.ctx, t, conn, "append")
	if resumed.Start != uint64(before) || resumed.End != uint64(before+len(tail)) {
		t.Fatalf("append position: %+v", resumed)
	}
	if delta := f.next(conn, 0); !bytes.Equal(delta, tail) {
		t.Fatalf("delta %q", delta)
	}

	for _, query := range []string{
		"generation=0000000000000000&offset=" + strconv.Itoa(before),
		"generation=" + generation + "&offset=999999",
		"generation=" + generation + "&offset=not-a-number",
	} {
		reset := f.dial("client=device-two&" + query)
		if position := readPosition(f.ctx, t, reset, "reset"); position.End != uint64(before+len(tail)) {
			t.Fatalf("%s: %+v", query, position)
		}
		if snapshot := f.next(reset, 1); !bytes.Equal(snapshot, f.item.Buffer()) {
			t.Fatalf("%s: snapshot %q", query, snapshot)
		}
	}
}

func TestSubmissionIsWrittenOnceAndQueryable(t *testing.T) {
	record := filepath.Join(t.TempDir(), "input")
	f := newContinuityFixture(t, recorder(record))
	generation := f.item.Info().Generation
	conn := f.dial("client=device-one")
	id := "submission-0001"
	f.send(conn, 3, submitFrame(id, generation, "hello\x1b[201~", "\r"))
	if status := f.status(conn); status.ID != id || status.State != session.SubmissionReceived {
		t.Fatalf("receipt %+v", status)
	}
	if status := f.status(conn); status.State != session.SubmissionWritten {
		t.Fatalf("completion %+v", status)
	}
	waitFile(t, record, "hello\x1b[201~\r")

	// A retry after a lost acknowledgement reports the result without writing again.
	retry := f.dial("client=device-one")
	f.send(retry, 3, submitFrame(id, generation, "hello\x1b[201~", "\r"))
	if status := f.status(retry); status.State != session.SubmissionWritten {
		t.Fatalf("duplicate %+v", status)
	}
	f.send(retry, 4, []byte(`{"ids":["`+id+`","never-sent-0001","bad id"]}`))
	if status := f.status(retry); status.ID != id || status.State != session.SubmissionWritten {
		t.Fatalf("query %+v", status)
	}
	if status := f.status(retry); status.ID != "never-sent-0001" || status.State != session.SubmissionMissing {
		t.Fatalf("missing query %+v", status)
	}
	f.send(retry, 3, submitFrame("submission-0002", "0000000000000000", "stale", "\r"))
	if status := f.status(retry); status.State != session.SubmissionFailed || status.Error != "generation" {
		t.Fatalf("stale generation %+v", status)
	}
	f.send(retry, 3, []byte(`{"id":"submission-0003","generation":"`+generation+`","bodyBytes":99}`+"\nshort"))
	if status := f.status(retry); status.State != session.SubmissionFailed || status.Error != "invalid" {
		t.Fatalf("invalid frame %+v", status)
	}
	time.Sleep(100 * time.Millisecond)
	waitFile(t, record, "hello\x1b[201~\r")
}

func TestOnlyControllerResizesAndAnswersTerminalQueries(t *testing.T) {
	record := filepath.Join(t.TempDir(), "input")
	f := newContinuityFixture(t, recorder(record))
	phone := f.dial("client=phone-client&label=iPhone")
	if state := f.control(phone); state.Controller == nil || state.Controller.ClientID != "phone-client" || state.Controller.Label != "iPhone" {
		t.Fatalf("first client control: %+v", state)
	}
	desk := f.dial("client=desk-client&label=" + strings.Repeat("长", 50) + "%07")
	if state := f.control(desk); state.Controller == nil || state.Controller.ClientID != "phone-client" {
		t.Fatalf("observer sees controller: %+v", state)
	}

	f.send(desk, 1, []byte(`{"cols":200,"rows":60}`))
	f.send(phone, 1, []byte(`{"cols":50,"rows":20}`))
	if state := f.control(desk); state.Cols != 50 || state.Rows != 20 {
		t.Fatalf("observer follows controller grid: %+v", state)
	}
	if info := f.item.Info(); info.Cols != 50 || info.Rows != 20 {
		t.Fatalf("observer resized PTY: %dx%d", info.Cols, info.Rows)
	}
	f.control(phone)

	// Both xterms answer a device-attributes query; only the controller's reply is written.
	f.send(desk, 0, []byte("\x1b[?1;2c"))
	f.send(phone, 0, []byte("\x1b[?6c"))
	waitFile(t, record, "\x1b[?6c")

	// User input from the observer takes control.
	f.send(desk, 0, []byte("typed"))
	state := f.control(phone)
	if state.Controller == nil || state.Controller.ClientID != "desk-client" || len([]rune(state.Controller.Label)) != 40 || strings.ContainsRune(state.Controller.Label, 7) {
		t.Fatalf("input claim: %+v", state)
	}
	f.control(desk)
	waitFile(t, record, "\x1b[?6ctyped")
	f.send(phone, 0, []byte("\x1b[?6c"))
	f.send(phone, 5, nil)
	if state := f.control(desk); state.Controller == nil || state.Controller.ClientID != "phone-client" {
		t.Fatalf("explicit claim: %+v", state)
	}
	waitFile(t, record, "\x1b[?6ctyped")

	// When the controller leaves, nobody inherits control or the grid.
	_ = phone.CloseNow()
	if state := f.control(desk); state.Controller != nil || state.Cols != 50 {
		t.Fatalf("vacant control: %+v", state)
	}
	f.send(desk, 0, []byte("\x1b[0n"))
	waitFile(t, record, "\x1b[?6ctyped\x1b[0n")
}
