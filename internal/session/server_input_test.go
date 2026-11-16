package session

import (
	"context"
	"io"
	"os"
	"testing"
	"testing/synctest"
	"time"
)

// Server input is one acknowledged submission plus keys. It takes control
// with its label while writing, gives it back after, and a repeated request
// ID is never written twice.
func TestSubmitServer(t *testing.T) {
	m := NewManager(Options{DefaultCwd: t.TempDir()})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	}()
	s, err := m.Create(CreateInput{Cmd: []string{"/bin/cat"}})
	if err != nil {
		t.Fatal(err)
	}
	observer, err := s.Attach()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Detach(observer)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	in := ServerInput{RequestID: "req-00000001", Label: "script", Body: []byte("once"), Submit: []byte("\r"), Keys: [][]byte{[]byte("x\r")}}
	status, err := s.SubmitServer(ctx, in)
	if err != nil || status.State != SubmissionWritten {
		t.Fatalf("submit %+v %v", status, err)
	}
	waitForOutput(t, s, "once")
	waitForOutput(t, s, "x")
	time.Sleep(100 * time.Millisecond)
	written := countOutput(s.Buffer(), "once")
	if again, err := s.SubmitServer(ctx, in); err != nil || again != status {
		t.Fatalf("repeat %+v %v", again, err)
	}
	time.Sleep(100 * time.Millisecond)
	if n := countOutput(s.Buffer(), "once"); n != written {
		t.Fatalf("repeat wrote again: %d then %d", written, n)
	}
	labels := []string{}
	deadline := time.After(2 * time.Second)
	for len(labels) < 3 {
		select {
		case m := <-observer.Out:
			if m.Control != nil {
				label := ""
				if m.Control.Controller != nil {
					label = m.Control.Controller.Label
				}
				labels = append(labels, label)
			}
		case <-deadline:
			t.Fatalf("control frames %q", labels)
		}
	}
	// The observer took control on attach; the script took it over and
	// released it.
	if last := labels[len(labels)-1]; last != "" || !contains(labels, "script") {
		t.Fatalf("control labels %q", labels)
	}
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func countOutput(b []byte, text string) int {
	n := 0
	for i := 0; i+len(text) <= len(b); i++ {
		if string(b[i:i+len(text)]) == text {
			n++
		}
	}
	return n
}

// A server submission is pasted as the browser pastes a draft: line breaks
// become CR, bracketed while the program asked for that, and the submit key
// follows only after the submit gap, then the keys.
func TestSubmitServerPastesThenSubmits(t *testing.T) {
	for _, tc := range []struct {
		name      string
		output    string
		agent     time.Duration
		want      string
		wantDelay time.Duration
	}{
		{name: "plain", want: "a\rb" + "\r" + "k", wantDelay: submitDelay},
		{name: "bracketed", output: "\x1b[?2004h", want: "\x1b[200~a\rb\x1b[201~" + "\r" + "k", wantDelay: submitDelay},
		{name: "agent asks for more", agent: 300 * time.Millisecond, want: "a\rb" + "\r" + "k", wantDelay: 300 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				reader, writer, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = reader.Close() }()
				s := &Session{run: inputRun(t, writer), options: startOptions{submitDelay: tc.agent}}
				s.preview.Feed([]byte(tc.output))
				in := ServerInput{RequestID: "req", Body: []byte("a\nb"), Submit: []byte("\r"), Keys: [][]byte{[]byte("k")}}
				start := time.Now()
				status, err := s.SubmitServer(context.Background(), in)
				if err != nil || status.State != SubmissionWritten {
					t.Fatalf("submit %+v %v", status, err)
				}
				if elapsed := time.Since(start); elapsed != tc.wantDelay {
					t.Fatalf("submission took %v, want %v", elapsed, tc.wantDelay)
				}
				if again, err := s.SubmitServer(context.Background(), in); err != nil || again != status {
					t.Fatalf("repeat %+v %v", again, err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				written, err := io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				if string(written) != tc.want {
					t.Fatalf("wrote %q, want %q", written, tc.want)
				}
			})
		})
	}
}

// The input gate stays held through the submit gap, and a run that ends
// during it leaves a partial receipt without the submit key or the keys.
func TestSubmitServerGapHoldsGate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = reader.Close() }()
		r := inputRun(t, writer)
		s := &Session{run: r}
		in := ServerInput{RequestID: "req", Body: []byte("body"), Submit: []byte("\r"), Keys: [][]byte{[]byte("k")}}
		var status SubmissionStatus
		done := make(chan struct{})
		go func() {
			defer close(done)
			status, _ = s.SubmitServer(context.Background(), in)
		}()
		time.Sleep(submitDelay / 2)
		synctest.Wait()
		select {
		case <-r.inputGate:
			t.Fatal("input gate released during the submit gap")
		default:
		}
		close(r.done)
		<-done
		if status.State != SubmissionPartial || status.Error != "write" {
			t.Fatalf("status %+v", status)
		}
		if again, err := s.SubmitServer(context.Background(), in); err != nil || again != status {
			t.Fatalf("repeat %+v %v", again, err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		written, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if string(written) != "body" {
			t.Fatalf("wrote %q", written)
		}
	})
}
