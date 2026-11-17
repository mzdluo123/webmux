package hub

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
)

type idleDriver struct{}

func (idleDriver) Close() error  { return nil }
func (idleDriver) RunExited(int) {}

// A hybrid shell refuses terminal clients with 4409 whenever it is not on
// its terminal side, even while its run still exists (a switch that is
// interrupting the terminal UI); on the terminal side they attach.
func TestAttachRefusedOffTheTerminalSide(t *testing.T) {
	m := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	})
	s, err := m.CreateShell(session.ShellInput{
		CreateInput: session.CreateInput{Cmd: []string{"cat"}}, Argv: []string{"cat"},
		Overlay: &session.Overlay{Destination: "terminal"},
	}, idleDriver{})
	if err != nil {
		t.Fatal(err)
	}
	var allowed atomic.Bool
	allowed.Store(true)
	contribution := Options{Terminal: func(string) (bool, string) { return allowed.Load(), "chat" }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		Attach(r.Context(), s, NewWebSocketConn(c), session.AttachOptions{}, false, contribution)
	}))
	t.Cleanup(server.Close)
	dial := func() (*websocket.Conn, context.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.CloseNow() })
		return c, ctx
	}
	for range 2 {
		allowed.Store(false)
		c, ctx := dial()
		_, _, err := c.Read(ctx)
		var closeErr websocket.CloseError
		if !errors.As(err, &closeErr) || closeErr.Code != contract.CloseMoved || closeErr.Reason != "session moved: chat" {
			t.Fatalf("off-terminal attach got %v", err)
		}
	}
	allowed.Store(true)
	c, ctx := dial()
	if kind, frame, err := c.Read(ctx); err != nil || kind != websocket.MessageBinary || len(frame) == 0 || frame[0] != contract.FrameControl {
		t.Fatalf("terminal side attach: kind %v frame %x err %v", kind, frame, err)
	}
}
