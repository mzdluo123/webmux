package pluginwire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/forward"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/wire"
	"github.com/rainchan/webmux/internal/workspace"
)

// Projection is assembled explicitly by cmd. Every optional implementation
// remains on this controller side of the terminal host boundary.
type Projection struct {
	Workspace *workspace.Service
	Chat      *hybrid.Service
	Forward   *forward.Service
}

// Session composes the same typed projection for every transport and endpoint.
func (p Projection) Session(info session.Info) contract.SessionInfo {
	out := wire.Session(info)
	if p.Workspace != nil {
		if association := p.Workspace.Association(info.ID); association != nil {
			value := contract.SessionWorkspace(*association)
			out.Workspace = &value
		}
	}
	if p.Chat != nil {
		if metadata := p.Chat.Metadata(info.ID); metadata != nil {
			ProjectChat(&out, metadata)
		}
	}
	if p.Forward != nil {
		out.ListeningPorts = ListeningPorts(p.Forward.ListeningPorts(info.ID))
	}
	return out
}

// ProjectChat layers a chat-owned immutable snapshot onto the core DTO.
func ProjectChat(out *contract.SessionInfo, h *hybrid.Metadata) {
	out.ConversationID, out.ProviderThreadID = h.ConversationID, h.ProviderThreadID
	if h.State != "" {
		out.State = contract.SessionState(h.State)
	}
	out.Views = []contract.SessionView{contract.ViewTerminal, contract.ViewChat}
	out.View = View(h.Side)
	if h.To != "" {
		out.Transition = &contract.SessionTransition{To: View(h.To), Phase: contract.TransitionPhase(h.Phase)}
	}
	out.Agent = &contract.SessionAgent{Name: h.Agent, Title: h.AgentTitle, Icon: h.AgentIcon, Work: h.Work}
	if h.AgentSessionID != nil {
		id := *h.AgentSessionID
		out.Agent.SessionID = &id
	}
}

func (p Projection) Terminal(id string) (bool, string) {
	if p.Chat == nil {
		return true, ""
	}
	h := p.Chat.Metadata(id)
	if h == nil {
		return true, ""
	}
	if h.To != "" {
		return false, string(View(h.To))
	}
	return h.Side == string(hybrid.SideTUI), string(View(h.Side))
}

func (p Projection) SubscribeForwards() (<-chan struct{}, func()) {
	if p.Forward == nil {
		return nil, func() {}
	}
	sub := p.Forward.Subscribe()
	return sub.C, func() { p.Forward.Unsubscribe(sub) }
}

func (p Projection) Forwards() []contract.Forward {
	if p.Forward == nil {
		return []contract.Forward{}
	}
	return Forwards(p.Forward.List())
}

func projectedSessions(in []session.Info, project func(session.Info) contract.SessionInfo) []contract.SessionInfo {
	if project == nil {
		project = wire.Session
	}
	return each(in, project)
}
