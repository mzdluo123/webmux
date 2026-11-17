package contract

// Messaging DTOs describe local channels and their messages.
type MessagingChannel struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Topic        string `json:"topic"`
	LatestText   string `json:"latestText"`
	LatestAt     string `json:"latestAt"`
	MessageCount int    `json:"messageCount"`
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
