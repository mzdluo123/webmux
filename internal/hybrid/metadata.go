package hybrid

import "github.com/rainchan/webmux/internal/contract"

// Metadata is the chat plugin's immutable session-list projection. The plugin
// owns provider identity and view transitions; the process host sees neither.
type Metadata struct {
	ConversationID, ProviderThreadID string
	Side, To, Phase, State           string
	Agent, AgentTitle, AgentIcon     string
	AgentSessionID                   *string
	Work                             string
}

func (m *Metadata) clone() *Metadata {
	if m == nil {
		return nil
	}
	out := *m
	if m.AgentSessionID != nil {
		id := *m.AgentSessionID
		out.AgentSessionID = &id
	}
	return &out
}

// Metadata returns a caller-owned snapshot, without waiting on an actor or I/O.
func (s *Service) Metadata(id string) *Metadata {
	s.metadataMu.RLock()
	defer s.metadataMu.RUnlock()
	return s.metadata[id].clone()
}

func viewOf(side Side) contract.SessionView {
	if side == SideACP {
		return contract.ViewChat
	}
	return contract.ViewTerminal
}

func (a *actor) publishMetadata() {
	side, to := a.side, Side("")
	if a.side == SideSwitching && a.switching != nil {
		side, to = a.switching.from, a.switching.to
	}
	m := &Metadata{
		ConversationID: a.conversationID, ProviderThreadID: a.activity.thread,
		Side: string(side), To: string(to), Phase: string(a.chat.Phase), State: string(a.state),
		Agent: a.cmd.Profile.Name, AgentTitle: a.cmd.Profile.Title, AgentIcon: a.cmd.Profile.Icon,
	}
	if a.agentSessionID != "" {
		id := a.agentSessionID
		m.AgentSessionID = &id
	}
	if a.side == SideACP && a.state == StateActive && a.turn != nil {
		m.Work = a.workLine
	}
	a.svc.metadataMu.Lock()
	a.svc.metadata[a.id] = m
	a.svc.metadataMu.Unlock()
}

// SetWorkspaceSource installs the optional cross-plugin association port before
// requests are accepted. Nil retains record associations without workspace I/O.
func (s *Service) SetWorkspaceSource(source func(string) *WorkspaceRef) {
	s.metadataMu.Lock()
	s.workspace = source
	s.metadataMu.Unlock()
}

func (s *Service) workspaceReference(id string) (*WorkspaceRef, bool) {
	s.metadataMu.RLock()
	source := s.workspace
	s.metadataMu.RUnlock()
	if source == nil {
		return nil, false
	}
	return source(id), true
}

func (s *Service) clearMetadata(id string) {
	s.metadataMu.Lock()
	delete(s.metadata, id)
	s.metadataMu.Unlock()
}
