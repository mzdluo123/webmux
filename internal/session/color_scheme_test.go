package session

import (
	"context"
	"io"
	"os"
	"testing"
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

func writtenColorScheme(t *testing.T, reader *os.File, writer *os.File) string {
	t.Helper()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestReportColorSchemeWritesAutomaticReportOnlyWhenEnabled(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		dark    bool
		want    string
	}{
		{name: "dark", enabled: true, dark: true, want: colorSchemeDarkReport},
		{name: "light", enabled: true, dark: false, want: colorSchemeLightReport},
		{name: "disabled", enabled: false, dark: true, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, reader, writer := colorSchemePipeSession(t, tc.enabled)
			if err := s.ReportColorScheme(context.Background(), tc.dark); err != nil {
				t.Fatal(err)
			}
			if got := writtenColorScheme(t, reader, writer); got != tc.want {
				t.Fatalf("PTY input %q, want %q", got, tc.want)
			}
		})
	}
}

func TestManagerColorSchemeBroadcastsNotifications(t *testing.T) {
	enabled, enabledReader, enabledWriter := colorSchemePipeSession(t, true)
	disabled, disabledReader, disabledWriter := colorSchemePipeSession(t, false)
	m := &Manager{sessions: map[string]*Session{"enabled": enabled, "disabled": disabled}}

	m.ReportColorScheme(context.Background(), true)
	m.ReportColorScheme(context.Background(), true)
	m.ReportColorScheme(context.Background(), false)

	if got := writtenColorScheme(t, enabledReader, enabledWriter); got != colorSchemeDarkReport+colorSchemeDarkReport+colorSchemeLightReport {
		t.Fatalf("enabled PTY input %q", got)
	}
	if got := writtenColorScheme(t, disabledReader, disabledWriter); got != "" {
		t.Fatalf("disabled PTY input %q", got)
	}
}
