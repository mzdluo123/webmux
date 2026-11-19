//go:build unix

package hub

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
)

const inputFenceSocketEnv = "HUB_INPUT_FENCE_SOCKET"

// This subprocess owns a real PTY in raw mode. Its separate control socket
// holds off PTY reads until the parent has observed the hub reader's progress.
func TestHubInputFencePTYProcess(t *testing.T) {
	path := os.Getenv(inputFenceSocketEnv)
	if path == "" {
		return
	}
	stty := exec.Command("stty", "raw", "-echo")
	stty.Stdin = os.Stdin
	if output, err := stty.CombinedOutput(); err != nil {
		t.Fatalf("set raw PTY: %v: %s", err, output)
	}
	control, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer control.Close()
	if _, err := control.Write([]byte{'R'}); err != nil {
		t.Fatal(err)
	}
	for {
		var size uint32
		if err := binary.Read(control, binary.BigEndian, &size); err != nil {
			if err == io.EOF {
				return
			}
			t.Fatal(err)
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(os.Stdin, data); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		if _, err := control.Write(digest[:]); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFrameQueryFencesPTYWrites(t *testing.T) {
	for _, kind := range []struct {
		name  string
		frame byte
	}{
		{name: "raw", frame: contract.FrameInput},
		{name: "reply", frame: contract.FrameReply},
		{name: "submission", frame: contract.FrameSubmit},
	} {
		t.Run(kind.name, func(t *testing.T) {
			// Unix socket paths have a small platform-specific length limit.
			socketDir, err := os.MkdirTemp("/tmp", "hub-fence-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
			listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(socketDir, "control.sock"), Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			manager := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				manager.Shutdown(ctx)
			})
			s, err := manager.Create(session.CreateInput{
				Cmd: []string{os.Args[0], "-test.run=^TestHubInputFencePTYProcess$"},
				Env: map[string]string{inputFenceSocketEnv: listener.Addr().String()},
			})
			if err != nil {
				t.Fatal(err)
			}
			control, err := listener.AcceptUnix()
			if err != nil {
				t.Fatal(err)
			}
			if err := control.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			var ready [1]byte
			if _, err := io.ReadFull(control, ready[:]); err != nil || ready[0] != 'R' {
				t.Fatalf("PTY process readiness: %q, %v", ready, err)
			}

			conn := newFakeMessageConn()
			frames := make(chan recordedFrame, 64)
			conn.writeHook = func(ctx context.Context, typ MessageType, data []byte) error {
				select {
				case frames <- recordedFrame{typ: typ, data: bytes.Clone(data)}:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan struct{})
			go func() {
				defer close(done)
				Attach(ctx, s, conn, session.AttachOptions{Client: "fence-client"}, false)
			}()
			t.Cleanup(func() {
				cancel()
				_ = control.Close()
				_ = conn.CloseNow()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Error("input fence attachment did not join")
				}
			})
			nextFrame := func() []byte {
				t.Helper()
				select {
				case frame := <-frames:
					if frame.typ != MessageBinary || len(frame.data) == 0 {
						t.Fatal("invalid terminal frame")
					}
					return frame.data
				case <-done:
					t.Fatal("input fence attachment exited")
				case <-time.After(5 * time.Second):
					t.Fatal("terminal frame did not arrive")
				}
				return nil
			}
			for nextFrame()[0] != contract.FrameState {
			}
			sendJSON := func(frame byte, value any) {
				t.Helper()
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				conn.sendFromPeer(MessageBinary, contract.Frame(frame, data))
			}
			// Larger than the kernel PTY input queue: this write cannot finish
			// while the subprocess is waiting on its separate control socket.
			older := bytes.Repeat([]byte{'A'}, 512*1024)
			if kind.frame == contract.FrameSubmit {
				header, err := json.Marshal(contract.SubmissionHeader{ID: "original-submit", Generation: s.Info().Generation, BodyBytes: len(older)})
				if err != nil {
					t.Fatal(err)
				}
				older = append(older, '\r')
				payload := append(append(header, '\n'), older...)
				conn.sendFromPeer(MessageBinary, contract.Frame(kind.frame, payload))
			} else {
				conn.sendFromPeer(MessageBinary, contract.Frame(kind.frame, older))
			}
			const firstFence = "fence-first-unique"
			const secondFence = "fence-second-unique"
			sendJSON(contract.FrameQuery, contract.SubmissionQuery{IDs: []string{firstFence}})
			later := []byte("LATER_INPUT")
			conn.sendFromPeer(MessageBinary, contract.Frame(contract.FrameInput, later))
			sendJSON(contract.FrameQuery, contract.SubmissionQuery{IDs: []string{secondFence}})
			sendJSON(contract.FrameResize, contract.TerminalResize{Cols: 91, Rows: 23})
			// Resize runs in the reader and its control frame has the same sole
			// outbound writer. Reaching it proves both queries were dispatched;
			// neither may have answered before the blocked PTY write finishes.
			for {
				frame := nextFrame()
				if frame[0] == contract.FrameSubmission {
					var status contract.SubmissionStatus
					if err := json.Unmarshal(frame[1:], &status); err != nil {
						t.Fatal(err)
					}
					if status.ID == firstFence || status.ID == secondFence {
						t.Fatalf("query overtook blocked %s PTY write: %+v", kind.name, status)
					}
				}
				if frame[0] == contract.FrameControl {
					var state contract.ControlState
					if err := json.Unmarshal(frame[1:], &state); err != nil {
						t.Fatal(err)
					}
					if state.Cols == 91 && state.Rows == 23 {
						break
					}
				}
			}
			expected := append(bytes.Clone(older), later...)
			if err := binary.Write(control, binary.BigEndian, uint32(len(expected))); err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{firstFence, secondFence} {
				for {
					frame := nextFrame()
					if frame[0] != contract.FrameSubmission {
						continue
					}
					var status contract.SubmissionStatus
					if err := json.Unmarshal(frame[1:], &status); err != nil {
						t.Fatal(err)
					}
					if status.ID != firstFence && status.ID != secondFence {
						continue
					}
					if status.ID != id || status.State != "missing" {
						t.Fatalf("out-of-order fence response: %+v, want %s missing", status, id)
					}
					break
				}
			}
			checkDigest := func(data []byte) {
				t.Helper()
				var actual [sha256.Size]byte
				if _, err := io.ReadFull(control, actual[:]); err != nil {
					t.Fatal(err)
				}
				if actual != sha256.Sum256(data) {
					t.Fatal("PTY input was reordered, duplicated or lost across query fences")
				}
			}
			checkDigest(expected)
			// A query neither writes probe bytes nor ends the persistent PTY.
			survivor := []byte("AFTER_FENCE")
			conn.sendFromPeer(MessageBinary, contract.Frame(contract.FrameInput, survivor))
			if err := binary.Write(control, binary.BigEndian, uint32(len(survivor))); err != nil {
				t.Fatal(err)
			}
			checkDigest(survivor)
			if s.Info().Status == session.StatusExited {
				t.Fatal("query fence ended the persistent terminal")
			}
		})
	}
}

func TestInputQueueAbsorbsStartupBurstAndBoundsBytes(t *testing.T) {
	socketDir, err := os.MkdirTemp("/tmp", "hub-burst-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(socketDir, "control.sock"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	manager := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		manager.Shutdown(ctx)
	})
	s, err := manager.Create(session.CreateInput{
		Cmd: []string{os.Args[0], "-test.run=^TestHubInputFencePTYProcess$"},
		Env: map[string]string{inputFenceSocketEnv: listener.Addr().String()},
	})
	if err != nil {
		t.Fatal(err)
	}
	control, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	if err := control.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var ready [1]byte
	if _, err := io.ReadFull(control, ready[:]); err != nil || ready[0] != 'R' {
		t.Fatalf("PTY process readiness: %q, %v", ready, err)
	}

	conn := newFakeMessageConn()
	frames := make(chan recordedFrame, 128)
	conn.writeHook = func(ctx context.Context, typ MessageType, data []byte) error {
		select {
		case frames <- recordedFrame{typ: typ, data: bytes.Clone(data)}:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		Attach(ctx, s, conn, session.AttachOptions{Client: "burst-client"}, false)
	}()
	t.Cleanup(func() {
		cancel()
		_ = control.Close()
		_ = conn.CloseNow()
		<-done
	})
	nextFrame := func() []byte {
		t.Helper()
		select {
		case frame := <-frames:
			return frame.data
		case <-done:
			t.Fatal("burst attachment exited unexpectedly")
		case <-time.After(5 * time.Second):
			t.Fatal("terminal frame did not arrive")
		}
		return nil
	}
	for nextFrame()[0] != contract.FrameState {
	}

	// Block the PTY writer, then queue 64 replies/inputs (4x the old 16-slot
	// ceiling) plus a fence query; all must survive in FIFO order.
	blocker := bytes.Repeat([]byte{'B'}, 512*1024)
	conn.sendFromPeer(MessageBinary, contract.Frame(contract.FrameInput, blocker))
	expected := bytes.Clone(blocker)
	for i := range 64 {
		kind := contract.FrameReply
		if i%2 == 1 {
			kind = contract.FrameInput
		}
		chunk := []byte{byte('a' + (i % 26))}
		expected = append(expected, chunk...)
		conn.sendFromPeer(MessageBinary, contract.Frame(kind, chunk))
	}
	const fenceID = "burst-fence-check"
	queryJSON, err := json.Marshal(contract.SubmissionQuery{IDs: []string{fenceID}})
	if err != nil {
		t.Fatal(err)
	}
	conn.sendFromPeer(MessageBinary, contract.Frame(contract.FrameQuery, queryJSON))
	resizeJSON, err := json.Marshal(contract.TerminalResize{Cols: 93, Rows: 27})
	if err != nil {
		t.Fatal(err)
	}
	conn.sendFromPeer(MessageBinary, contract.Frame(contract.FrameResize, resizeJSON))
	for {
		frame := nextFrame()
		if frame[0] == contract.FrameControl {
			var state contract.ControlState
			if err := json.Unmarshal(frame[1:], &state); err == nil && state.Cols == 93 && state.Rows == 27 {
				break
			}
		}
	}
	if err := binary.Write(control, binary.BigEndian, uint32(len(expected))); err != nil {
		t.Fatal(err)
	}
	for {
		frame := nextFrame()
		if frame[0] != contract.FrameSubmission {
			continue
		}
		var status contract.SubmissionStatus
		if err := json.Unmarshal(frame[1:], &status); err != nil {
			t.Fatal(err)
		}
		if status.ID == fenceID && status.State == "missing" {
			break
		}
	}
	var actual [sha256.Size]byte
	if _, err := io.ReadFull(control, actual[:]); err != nil {
		t.Fatal(err)
	}
	if actual != sha256.Sum256(expected) {
		t.Fatal("burst frames were lost or reordered")
	}

	// Exceeding the 2 MiB queued-input byte budget while the PTY stays blocked
	// disconnects the slow/abusive connection without ending the session.
	chunk := bytes.Repeat([]byte{'X'}, 768*1024)
	for range 5 {
		conn.sendFromPeer(MessageBinary, contract.Frame(contract.FrameInput, chunk))
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("byte-budget overflow did not disconnect client")
	}
	if s.Info().Status == session.StatusExited {
		t.Fatal("byte-budget disconnect ended persistent session")
	}
}
