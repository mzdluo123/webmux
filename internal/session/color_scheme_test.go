package session

import (
	"io"
	"os"
	"testing"
	"time"
)

func colorSchemePipeSession(t *testing.T, enabled bool) (*Session, *os.File, *os.File) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	s := &Session{run: inputRun(t, writer)}
	if enabled {
		s.preview.Feed([]byte("\x1b[?2031h"))
	}
	return s, reader, writer
}

func queuedColorSchemes(s *Session) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	replies := make([]string, 0, len(s.replies))
	for _, reply := range s.replies {
		replies = append(replies, string(reply))
	}
	return replies
}

func TestQueueColorSchemeReportOnlyWhenEnabled(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		want    []string
	}{
		{name: "enabled", enabled: true, want: []string{colorSchemeDarkReport}},
		{name: "disabled", enabled: false, want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _ := colorSchemePipeSession(t, tc.enabled)
			s.QueueColorSchemeReport(true)
			got := queuedColorSchemes(s)
			if len(got) != len(tc.want) || (len(got) == 1 && got[0] != tc.want[0]) {
				t.Fatalf("queued replies %q, want %q", got, tc.want)
			}
		})
	}
}

func TestQueueColorSchemeReportKeepsOnlyLatestTheme(t *testing.T) {
	s, _, _ := colorSchemePipeSession(t, true)
	s.QueueColorSchemeReport(true)
	s.QueueColorSchemeReport(false)
	s.QueueColorSchemeReport(true)
	got := queuedColorSchemes(s)
	if len(got) != 1 || got[0] != colorSchemeDarkReport {
		t.Fatalf("queued replies %q", got)
	}
}

func TestQueuedColorSchemeReportReachesPTYWithoutAClient(t *testing.T) {
	s, reader, writer := colorSchemePipeSession(t, true)
	go s.replyLoop(s.run)
	s.QueueColorSchemeReport(true)

	got := make(chan []byte, 1)
	go func() {
		buf := make([]byte, len(colorSchemeDarkReport))
		n, _ := io.ReadFull(reader, buf)
		got <- buf[:n]
	}()
	select {
	case written := <-got:
		if string(written) != colorSchemeDarkReport {
			t.Fatalf("PTY input %q", written)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("color scheme report was never written without a client")
	}
	s.run.endLifetime()
	<-s.run.repliesDone
	_ = writer.Close()
	if extra, _ := io.ReadAll(reader); len(extra) != 0 {
		t.Fatalf("extra PTY input %q", extra)
	}
}

func TestManagerColorSchemeBroadcastQueuesAndCoalesces(t *testing.T) {
	enabled, _, _ := colorSchemePipeSession(t, true)
	disabled, _, _ := colorSchemePipeSession(t, false)
	m := &Manager{sessions: map[string]*Session{"enabled": enabled, "disabled": disabled}}

	m.BroadcastColorScheme(true)
	m.BroadcastColorScheme(false)

	if dark, ok := m.currentColorScheme(); !ok || dark {
		t.Fatalf("current color scheme dark=%v ok=%v", dark, ok)
	}
	if got := queuedColorSchemes(enabled); len(got) != 1 || got[0] != colorSchemeLightReport {
		t.Fatalf("enabled queued replies %q", got)
	}
	if got := queuedColorSchemes(disabled); len(got) != 0 {
		t.Fatalf("disabled queued replies %q", got)
	}
}

func TestColorSchemeModeEnableQueuesRecordedTheme(t *testing.T) {
	s, _, _ := colorSchemePipeSession(t, false)
	s.options.colorScheme = func() (bool, bool) { return true, true }
	s.mu.Lock()
	s.preview.Feed([]byte("\x1b[?2031h"))
	s.colorSchemeModeEnabledLocked()
	s.mu.Unlock()
	if got := queuedColorSchemes(s); len(got) != 1 || got[0] != colorSchemeDarkReport {
		t.Fatalf("queued replies %q", got)
	}
}
