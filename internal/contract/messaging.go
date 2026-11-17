package contract

// Messaging DTOs describe local channels and their messages.
type MessagingChannel struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Topic        string `json:"topic"`
	LatestText   string `json:"latestText"`
	LatestAt     string `json:"latestAt"`
	MessageCount int    `json:"messageCount"`
	MemberCount  int    `json:"memberCount"`
}

type MessagingCreateChannel struct {
	Name  string `json:"name"`
	Topic string `json:"topic"`
}

type MessagingMessage struct {
	ID        string `json:"id"`
	AgentID   string `json:"agentId"`
	AgentName string `json:"agentName"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
}

type MessagingSend struct {
	Text            string `json:"text"`
	ClientRequestID string `json:"clientRequestId"`
}

// MessagingAgentRequest is carried only on the Unix socket. Credential is
// supplied by the host-generated plugin environment, never a tool argument.
type MessagingAgentRequest struct {
	Credential      string `json:"credential"`
	Operation       string `json:"operation"`
	Channel         string `json:"channel"`
	Text            string `json:"text"`
	ClientRequestID string `json:"clientRequestId"`
	Through         string `json:"through"`
	WaitSeconds     int    `json:"waitSeconds"`
	WakeAgents      bool   `json:"wakeAgents"`
}

type MessagingIdentity struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Profile string `json:"profile"`
}
type MessagingMember struct {
	MessagingIdentity
	Channel string `json:"channel"`
	Cursor  string `json:"cursor"`
}
type MessagingInboxItem struct {
	MessagingMessage
	Channel string `json:"channel"`
}
type MessagingInbox struct {
	Messages []MessagingInboxItem `json:"messages"`
	Through  string               `json:"through"`
}
type MessagingAgent struct {
	SessionID string `json:"sessionId"`
	Name      string `json:"name"`
	Profile   string `json:"profile"`
}
type MessagingInvite struct {
	SessionID string `json:"sessionId"`
}
