package session

import "bytes"

// DEC mode 2031 reports: CSI ? 997 ; 1 n is dark, 2 is light. The browser
// reports the effective page theme to the manager; the manager forwards it
// as an automatic terminal reply to every program that asked for reports.
const (
	colorSchemeDarkReport  = "\x1b[?997;1n"
	colorSchemeLightReport = "\x1b[?997;2n"
)

const (
	colorSchemeUnknown int32 = iota
	colorSchemeDark
	colorSchemeLight
)

var (
	colorSchemeDarkReportBytes  = []byte(colorSchemeDarkReport)
	colorSchemeLightReportBytes = []byte(colorSchemeLightReport)
)

func colorSchemeReport(dark bool) []byte {
	if dark {
		return colorSchemeDarkReportBytes
	}
	return colorSchemeLightReportBytes
}

func isColorSchemeReport(reply []byte) bool {
	return bytes.Equal(reply, colorSchemeDarkReportBytes) || bytes.Equal(reply, colorSchemeLightReportBytes)
}

// queueColorSchemeReportLocked queues or replaces this run's pending color
// scheme report. Only the latest theme matters when several changes arrive
// before the reply loop can write them.
func (s *Session) queueColorSchemeReportLocked(dark bool) {
	if s.run == nil || !s.preview.ColorSchemeReports() {
		return
	}
	report := colorSchemeReport(dark)
	for i, queued := range s.replies {
		if isColorSchemeReport(queued) {
			s.replies[i] = report
			select {
			case s.run.replyWake <- struct{}{}:
			default:
			}
			return
		}
	}
	s.queueReplyLocked(report)
}

// QueueColorSchemeReport queues a report when this session's program enabled
// DEC mode 2031. The reply loop writes it as automatic input even while no
// browser is attached; it never counts as user input or takes control.
func (s *Session) QueueColorSchemeReport(dark bool) {
	s.mu.Lock()
	s.queueColorSchemeReportLocked(dark)
	s.mu.Unlock()
}

// colorSchemeModeEnabledLocked applies the recorded browser theme when a
// program newly enables DEC mode 2031.
func (s *Session) colorSchemeModeEnabledLocked() {
	if s.options.colorScheme == nil {
		return
	}
	if dark, ok := s.options.colorScheme(); ok {
		s.queueColorSchemeReportLocked(dark)
	}
}

// currentColorScheme is the latest theme reported by any browser. It is kept
// in memory because a server restart also ends every terminal it describes.
func (m *Manager) currentColorScheme() (dark bool, ok bool) {
	switch m.colorScheme.Load() {
	case colorSchemeDark:
		return true, true
	case colorSchemeLight:
		return false, true
	default:
		return false, false
	}
}

// BroadcastColorScheme records one browser's effective theme and queues it
// for every running session whose program asked for reports. Programs that
// enable DEC mode 2031 later receive the recorded theme when they enable it.
func (m *Manager) BroadcastColorScheme(dark bool) {
	state := colorSchemeLight
	if dark {
		state = colorSchemeDark
	}
	m.colorScheme.Store(state)
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, item := range m.sessions {
		sessions = append(sessions, item)
	}
	m.mu.Unlock()
	for _, item := range sessions {
		item.QueueColorSchemeReport(dark)
	}
}
