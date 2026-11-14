// Package wire converts terminal core values to named contract DTOs.
package wire

import (
	"maps"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hoststat"
	"github.com/rainchan/webmux/internal/session"
)

// Session projects process and terminal facts only. Optional plugin projections
// extend this result at the composition boundary.
func Session(i session.Info) contract.SessionInfo {
	cmd := i.Cmd
	if cmd == nil {
		cmd = []string{}
	}
	out := contract.SessionInfo{
		ID: i.ID, Generation: i.Generation, Title: i.Title, Cwd: i.Cwd,
		Cmd: cmd, Status: i.Status, ExitCode: i.ExitCode, Preview: i.Preview,
		CreatedAt: i.CreatedAt, LastActiveAt: i.LastActiveAt, Cols: i.Cols, Rows: i.Rows, Clients: i.Clients,
		ConfigID: i.ConfigID, ListeningPorts: []contract.ListeningPort{},
		ParentID: i.ParentID, Labels: maps.Clone(i.Labels),
		State: contract.SessionStateActive, Views: []contract.SessionView{contract.ViewTerminal}, View: contract.ViewTerminal,
	}
	if i.Attention != nil {
		out.Attention = &contract.SessionAttention{Kind: i.Attention.Kind, Source: i.Attention.Source, Since: i.Attention.Since, Message: i.Attention.Message}
	}
	if i.ReportedCwd != nil {
		value := contract.SessionReportedCwd(*i.ReportedCwd)
		out.ReportedCwd = &value
	}
	return out
}

func Sessions(in []session.Info) []contract.SessionInfo {
	out := make([]contract.SessionInfo, len(in))
	for i, value := range in {
		out[i] = Session(value)
	}
	return out
}

// SessionEvent converts a core event; a host may inject one typed projection
// used for snapshots, increments, terminal frames and REST details alike.
func SessionEvent(e session.Event, project func(session.Info) contract.SessionInfo) contract.SessionEvent {
	if project == nil {
		project = Session
	}
	out := contract.SessionEvent{Type: e.Type, Sessions: make([]contract.SessionInfo, len(e.Sessions)), ID: e.ID}
	for i, value := range e.Sessions {
		out.Sessions[i] = project(value)
	}
	if e.Session != nil {
		value := project(*e.Session)
		out.Session = &value
	}
	return out
}

func StreamPosition(p session.Position) contract.StreamPosition { return contract.StreamPosition(p) }

func SubmissionStatus(s session.SubmissionStatus) contract.SubmissionStatus {
	return contract.SubmissionStatus{ID: s.ID, State: string(s.State), Error: s.Error}
}

func ControlState(c session.ControlState) contract.ControlState {
	out := contract.ControlState{Cols: c.Cols, Rows: c.Rows, ColorSchemeReports: c.ColorSchemeReports}
	if c.Controller != nil {
		value := contract.TerminalController(*c.Controller)
		out.Controller = &value
	}
	return out
}

func Health(sessions int, load *hoststat.Load, mem *hoststat.Memory) contract.APIHealth {
	h := contract.APIHealth{OK: true, Sessions: sessions}
	if load != nil {
		h.Load = &contract.HostLoad{One: load.One, Five: load.Five, Fifteen: load.Fifteen, CPUs: load.CPUs}
	}
	if mem != nil {
		h.Memory = &contract.HostMemory{Total: mem.Total, Available: mem.Available}
	}
	return h
}

func PendingUpdate(installed, launcher string) *contract.PendingUpdate {
	if installed == "" {
		return nil
	}
	return &contract.PendingUpdate{InstalledVersion: installed, Launcher: launcher}
}

func ScriptResize(r contract.SessionResizeRequest, label string) session.ScriptResize {
	return session.ScriptResize{Cols: r.Cols, Rows: r.Rows, Take: r.Take, Label: label}
}
