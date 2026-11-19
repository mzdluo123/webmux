package voice

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func helperService(t *testing.T, script string, extraEnv ...string) *Service {
	t.Helper()
	return New(Options{
		Cmd:           []string{"/bin/sh", "-c", script},
		Env:           extraEnv,
		ReadyDelay:    20 * time.Millisecond,
		FinishTimeout: 200 * time.Millisecond,
	})
}

func collectEvents(ch <-chan Event) []Event {
	var out []Event
	for ev := range ch {
		out = append(out, ev)
	}
	return out
}

func TestNDJSONStreamingProtocol(t *testing.T) {
	script := `printf '{"type":"ready"}\n'; bytes=$(wc -c | tr -d ' '); printf '{"type":"partial","text":"bytes:%s"}\n' "$bytes"; printf '{"type":"final","text":"%s:%s"}\n' "$CUSTOM_VOICE_TAG" "$bytes"`
	svc := helperService(t, script, "CUSTOM_VOICE_TAG=qwen")
	if !svc.Enabled() {
		t.Fatal("expected service enabled")
	}
	sess, err := svc.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	first := <-sess.Events()
	if first.Type != EventReady {
		t.Fatalf("first event = %+v, want ready", first)
	}

	if err := sess.WriteAudio(make([]byte, 640)); err != nil {
		t.Fatal(err)
	}
	if err := sess.WriteAudio(make([]byte, 640)); err != nil {
		t.Fatal(err)
	}
	sess.StopInput(2 * time.Second)

	rest := collectEvents(sess.Events())
	want := []Event{
		{Type: EventPartial, Text: "bytes:1280"},
		{Type: EventFinal, Text: "qwen:1280"},
	}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("events = %+v, want %+v", rest, want)
	}
}

func TestPlainTextFallbackProtocol(t *testing.T) {
	script := `bytes=$(wc -c | tr -d ' '); printf 'plain result (%s bytes)\n' "$bytes"`
	svc := helperService(t, script)
	sess, err := svc.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	first := <-sess.Events()
	if first.Type != EventReady {
		t.Fatalf("first event = %+v, want implicit ready", first)
	}
	if err := sess.WriteAudio(make([]byte, 320)); err != nil {
		t.Fatal(err)
	}
	sess.StopInput(2 * time.Second)

	rest := collectEvents(sess.Events())
	want := []Event{
		{Type: EventFinal, Text: "plain result (320 bytes)"},
	}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("events = %+v, want %+v", rest, want)
	}
}

func TestErrorJSONAndStderrExtraction(t *testing.T) {
	t.Run("json error event", func(t *testing.T) {
		svc := helperService(t, `printf '{"type":"error","error":"upstream quota exceeded"}\n'; exit 1`)
		sess, err := svc.Start(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer sess.Close()
		events := collectEvents(sess.Events())
		if len(events) == 0 || events[len(events)-1].Type != EventError || events[len(events)-1].Error != "upstream quota exceeded" {
			t.Fatalf("events = %+v", events)
		}
	})
	t.Run("stderr non-zero exit", func(t *testing.T) {
		svc := helperService(t, `printf 'fatal: api key invalid\n' >&2; exit 2`)
		sess, err := svc.Start(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer sess.Close()
		events := collectEvents(sess.Events())
		if len(events) == 0 || events[len(events)-1].Type != EventError || !strings.Contains(events[len(events)-1].Error, "api key invalid") {
			t.Fatalf("events = %+v", events)
		}
	})
}

func TestHungSubprocessTerminatedOnTimeoutAndClose(t *testing.T) {
	svc := helperService(t, `printf '{"type":"ready"}\n'; exec sleep 30`)
	sess, err := svc.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	first := <-sess.Events()
	if first.Type != EventReady {
		t.Fatalf("first event = %+v", first)
	}
	sess.StopInput(50 * time.Millisecond)
	done := make(chan struct{})
	go func() {
		for range sess.Events() {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("hung subprocess did not terminate after StopInput timeout")
	}
	sess.Close()
}

func TestReadyDelayBoundaryConcurrentExitRace(t *testing.T) {
	svc := New(Options{
		Cmd:           []string{"/bin/sh", "-c", `sleep 0.001; printf '{"type":"final","text":"ok"}\n'`},
		ReadyDelay:    time.Millisecond,
		FinishTimeout: 200 * time.Millisecond,
	})
	for range 40 {
		sess, err := svc.Start(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Go(func() {
			for range sess.Events() {
			}
		})
		time.Sleep(time.Millisecond)
		sess.Close()
		wg.Wait()
	}
}

func TestFullStdinPipeUnblocksOnStopInputAndClose(t *testing.T) {
	for _, action := range []string{"stop", "close"} {
		t.Run(action, func(t *testing.T) {
			svc := New(Options{
				Cmd:           []string{"/bin/sh", "-c", `printf '{"type":"ready"}\n'; exec sleep 30`},
				ReadyDelay:    10 * time.Millisecond,
				FinishTimeout: 50 * time.Millisecond,
			})
			sess, err := svc.Start(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer sess.Close()
			if ev := <-sess.Events(); ev.Type != EventReady {
				t.Fatalf("first event = %+v", ev)
			}

			writerDone := make(chan error, 1)
			go func() {
				chunk := make([]byte, 64*1024)
				for range 64 {
					if err := sess.WriteAudio(chunk); err != nil {
						writerDone <- err
						return
					}
				}
				writerDone <- nil
			}()
			// Give the writer time to fill the OS pipe buffer and block.
			time.Sleep(40 * time.Millisecond)

			unblocked := make(chan struct{})
			go func() {
				if action == "stop" {
					sess.StopInput(50 * time.Millisecond)
				} else {
					sess.Close()
				}
				close(unblocked)
			}()

			select {
			case <-unblocked:
			case <-time.After(2 * time.Second):
				t.Fatalf("%s deadlocked while WriteAudio blocked on full pipe", action)
			}
			select {
			case err := <-writerDone:
				if err == nil {
					t.Fatal("expected WriteAudio to return an error when stdin closed")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("blocked WriteAudio did not wake after stdin closed")
			}
		})
	}
}

func TestInheritedStdoutGrandchildExitsBoundedAndKillsProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	svc := New(Options{
		Cmd: []string{"/bin/sh", "-c", `sleep 30 & echo $! > "$PID_FILE"; printf '{"type":"final","text":"bounded"}\n'; exit 0`},
		Env: []string{"PID_FILE=" + pidFile},
	})
	start := time.Now()
	sess, err := svc.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	events := collectEvents(sess.Events())
	sess.Close()
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("inherited stdout delayed exit for %v", elapsed)
	}
	if len(events) == 0 || events[len(events)-1].Type != EventFinal || events[len(events)-1].Text != "bounded" {
		t.Fatalf("events = %+v", events)
	}
	rawPID, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for syscall.Kill(childPID, 0) == nil {
		if time.Now().After(deadline) {
			_ = syscall.Kill(childPID, syscall.SIGKILL)
			t.Fatalf("background process-group child %d survived Close", childPID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestStartSynchronousSpawnFailures(t *testing.T) {
	nonExec := filepath.Join(t.TempDir(), "not-executable.sh")
	if err := os.WriteFile(nonExec, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		give Options
		want error
	}{
		{
			name: "missing path lookup",
			give: Options{Cmd: []string{"webmux-definitely-missing-voice-cmd"}},
			want: exec.ErrNotFound,
		},
		{
			name: "missing executable path",
			give: Options{Cmd: []string{filepath.Join(t.TempDir(), "missing-bin")}},
			want: os.ErrNotExist,
		},
		{
			name: "permission denied executable",
			give: Options{Cmd: []string{nonExec}},
			want: os.ErrPermission,
		},
		{
			name: "missing working directory",
			give: Options{
				Cmd: []string{"/bin/true"},
				Cwd: filepath.Join(t.TempDir(), "missing-cwd"),
			},
			want: os.ErrNotExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := New(tt.give)
			sess, err := svc.Start(t.Context())
			if sess != nil {
				sess.Close()
				t.Fatalf("Start returned non-nil session on %s", tt.name)
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("Start error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestEarlyChildPipeCloseWhileChildAlive(t *testing.T) {
	t.Run("stdin close propagates EPIPE while child alive", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "stdin-closed.pid")
		svc := New(Options{
			Cmd: []string{"/bin/sh", "-c", `echo $$ > "$PID_FILE"; exec 0<&-; printf '{"type":"ready"}\n'; exec sleep 30`},
			Env: []string{"PID_FILE=" + pidFile},
		})
		sess, err := svc.Start(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer sess.Close()

		if ev := <-sess.Events(); ev.Type != EventReady {
			t.Fatalf("first event = %+v, want ready", ev)
		}
		rawPID, err := os.ReadFile(pidFile)
		if err != nil {
			t.Fatal(err)
		}
		childPID, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
		if err != nil {
			t.Fatal(err)
		}

		var writeErr error
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			writeErr = sess.WriteAudio([]byte{1, 2, 3, 4})
			if writeErr != nil {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if !errors.Is(writeErr, syscall.EPIPE) {
			t.Fatalf("WriteAudio error = %v, want EPIPE", writeErr)
		}
		if err := syscall.Kill(childPID, 0); err != nil {
			t.Fatalf("child %d exited before EPIPE check: %v", childPID, err)
		}
	})

	t.Run("stdout close propagates EOF while child alive", func(t *testing.T) {
		svc := New(Options{
			Cmd: []string{"/bin/sh", "-c", `printf '{"type":"final","text":"early-eof"}\n'; exec 1>&- 2>&-; exec cat >/dev/null`},
		})
		sess, err := svc.Start(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer sess.Close()

		done := make(chan []Event, 1)
		go func() {
			done <- collectEvents(sess.Events())
		}()
		select {
		case events := <-done:
			if len(events) == 0 || events[len(events)-1].Type != EventFinal || events[len(events)-1].Text != "early-eof" {
				t.Fatalf("events = %+v, want final early-eof", events)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("parent held child stdout pipe open and blocked EOF")
		}
	})
}
