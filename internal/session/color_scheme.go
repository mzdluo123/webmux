package session

import (
	"context"
	"sync"
)

// DEC mode 2031 reports: CSI ? 997 ; 1 n is dark, 2 is light. The browser
// reports the effective page theme to the manager; the manager forwards it
// as an automatic terminal reply to every program that asked for reports.
const (
	colorSchemeDarkReport  = "\x1b[?997;1n"
	colorSchemeLightReport = "\x1b[?997;2n"
)

// ReportColorScheme writes a color scheme report when this session's program
// enabled DEC mode 2031. It uses the automatic-reply path so the report never
// counts as user input or takes terminal control.
func (s *Session) ReportColorScheme(ctx context.Context, dark bool) error {
	s.mu.Lock()
	enabled := s.preview.ColorSchemeReports()
	s.mu.Unlock()
	if !enabled {
		return nil
	}
	report := colorSchemeLightReport
	if dark {
		report = colorSchemeDarkReport
	}
	return s.WriteInput(ctx, []byte(report), true)
}

// ReportColorScheme forwards the effective theme of one browser to every
// running session whose program asked for reports. Notifications are events:
// sessions created or programs enabled after an earlier notification still
// need a repeated report of the same theme.
func (m *Manager) ReportColorScheme(ctx context.Context, dark bool) {
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, item := range m.sessions {
		sessions = append(sessions, item)
	}
	m.mu.Unlock()
	var workers sync.WaitGroup
	for _, item := range sessions {
		// A session can exit or lose its run concurrently with the broadcast;
		// the next theme change reaches later runs again.
		workers.Go(func() { _ = item.ReportColorScheme(ctx, dark) })
	}
	workers.Wait()
}
