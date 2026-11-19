package featurehub

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/voice"
)

type testFrame struct {
	kind hub.MessageType
	data []byte
}
type testConn struct {
	input  chan testFrame
	done   chan struct{}
	once   sync.Once
	code   hub.StatusCode
	reason string
	limit  int64
	frames []testFrame
}

func newTestConn() *testConn {
	return &testConn{input: make(chan testFrame, 2), done: make(chan struct{})}
}
func (c *testConn) Read(ctx context.Context) (hub.MessageType, []byte, error) {
	select {
	case frame := <-c.input:
		return frame.kind, frame.data, nil
	case <-ctx.Done():
		return 0, nil, ctx.Err()
	case <-c.done:
		return 0, nil, context.Canceled
	}
}
func (c *testConn) Write(_ context.Context, kind hub.MessageType, data []byte) error {
	c.frames = append(c.frames, testFrame{kind, bytes.Clone(data)})
	return nil
}
func (c *testConn) Close(code hub.StatusCode, reason string) error {
	c.code, c.reason = code, reason
	return c.CloseNow()
}
func (c *testConn) CloseNow() error                               { c.once.Do(func() { close(c.done) }); return nil }
func (c *testConn) SetReadLimit(limit int64)                      { c.limit = limit }
func (c *testConn) CloseRead(ctx context.Context) context.Context { return ctx }

var _ hub.MessageConn = (*testConn)(nil)

func TestChatCloseReasons(t *testing.T) {
	for _, tt := range []struct {
		name   string
		cause  error
		code   hub.StatusCode
		reason string
	}{
		{"slow", hybrid.ErrSlow, hub.StatusSlowClient, "slow client"},
		{"reset", hybrid.ErrReset, hub.StatusTryAgainLater, "chat log reset; reconnect"},
		{"ended", hybrid.ErrEnded, hub.StatusSessionExited, "session ended"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			conn := newTestConn()
			closeChat(conn, tt.cause)
			if conn.code != tt.code || conn.reason != tt.reason {
				t.Fatalf("close = %d %q", conn.code, conn.reason)
			}
		})
	}
}

func TestVoiceFinalFrameAndReaderJoin(t *testing.T) {
	svc := voice.New(voice.Options{Cmd: []string{"sh", "-c", `cat >/dev/null; printf '{"type":"final","text":"hello"}\n'`}, Cwd: t.TempDir(), ReadyDelay: 5 * time.Millisecond, FinishTimeout: 2 * time.Second})
	sess, err := svc.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	conn := newTestConn()
	conn.input <- testFrame{hub.MessageBinary, []byte{0, 1, 2, 3}}
	conn.input <- testFrame{hub.MessageText, []byte(`{"type":"stop"}`)}
	Voice(t.Context(), sess, svc.FinishTimeout(), conn)
	if conn.limit != 65536 || conn.code != hub.StatusNormalClosure || conn.reason != "done" {
		t.Fatalf("close = %d %q, limit %d", conn.code, conn.reason, conn.limit)
	}
	for _, frame := range conn.frames {
		if frame.kind != hub.MessageText {
			t.Fatalf("unexpected frame kind %v", frame.kind)
		}
		var event contract.VoiceServerEvent
		if err := json.Unmarshal(frame.data, &event); err != nil {
			t.Fatal(err)
		}
		if event.Type == "final" && event.Text == "hello" {
			return
		}
	}
	t.Fatal("missing final transcription")
}

func TestVoiceStreamUnblocksFullPipeAndInheritedStdout(t *testing.T) {
	t.Run("full stdin pipe", func(t *testing.T) {
		svc := voice.New(voice.Options{
			Cmd:           []string{"/bin/sh", "-c", `printf '{"type":"ready"}\n'; exec sleep 30`},
			Cwd:           t.TempDir(),
			ReadyDelay:    5 * time.Millisecond,
			FinishTimeout: 50 * time.Millisecond,
		})
		sess, err := svc.Start(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		conn := &testConn{input: make(chan testFrame, 64), done: make(chan struct{})}
		chunk := bytes.Repeat([]byte{1}, 65536)
		for range 48 {
			conn.input <- testFrame{hub.MessageBinary, chunk}
		}
		ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
		defer cancel()
		finished := make(chan struct{})
		go func() {
			Voice(ctx, sess, svc.FinishTimeout(), conn)
			close(finished)
		}()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Fatal("featurehub.Voice deadlocked on full stdin pipe")
		}
	})

	t.Run("stop frame not starved behind hung subprocess", func(t *testing.T) {
		svc := voice.New(voice.Options{
			Cmd:           []string{"/bin/sh", "-c", `printf '{"type":"ready"}\n'; exec sleep 30`},
			Cwd:           t.TempDir(),
			ReadyDelay:    5 * time.Millisecond,
			FinishTimeout: 80 * time.Millisecond,
		})
		sess, err := svc.Start(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		conn := &testConn{input: make(chan testFrame, 16), done: make(chan struct{})}
		chunk := bytes.Repeat([]byte{7}, 65536)
		// Send 512 KiB (> 256 KiB) of binary PCM frames followed by {"type":"stop"}
		// without cancelling the caller's context.
		for range 8 {
			conn.input <- testFrame{hub.MessageBinary, chunk}
		}
		conn.input <- testFrame{hub.MessageText, []byte(`{"type":"stop"}`)}

		start := time.Now()
		finished := make(chan struct{})
		go func() {
			Voice(t.Context(), sess, svc.FinishTimeout(), conn)
			close(finished)
		}()
		select {
		case <-finished:
		case <-time.After(2 * time.Second):
			t.Fatal("featurehub.Voice starved stop frame on hung subprocess")
		}
		if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
			t.Fatalf("featurehub.Voice took %v, want bounded by finishTimeout", elapsed)
		}
		if conn.code != hub.StatusNormalClosure || conn.reason != "done" {
			t.Fatalf("close = %d %q, want normal closure", conn.code, conn.reason)
		}
	})

	t.Run("ready boundary and inherited stdout", func(t *testing.T) {
		svc := voice.New(voice.Options{
			Cmd:           []string{"/bin/sh", "-c", `(sleep 30 &) ; printf '{"type":"final","text":"stream-ok"}\n'`},
			Cwd:           t.TempDir(),
			ReadyDelay:    time.Millisecond,
			FinishTimeout: time.Second,
		})
		for range 15 {
			sess, err := svc.Start(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			conn := newTestConn()
			done := make(chan struct{})
			go func() {
				Voice(t.Context(), sess, svc.FinishTimeout(), conn)
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("featurehub.Voice hung with inherited stdout")
			}
		}
	})
}
