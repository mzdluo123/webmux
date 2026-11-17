package contract

import "time"

type HandoffLocation struct {
	Cwd               string `json:"cwd"`
	RepositoryID      string `json:"repositoryId"`
	WorktreeID        string `json:"worktreeId"`
	NewWorktreeBase   string `json:"newWorktreeBase"`
	NewWorktreeBranch string `json:"newWorktreeBranch"`
}
type HandoffPreviewRequest struct {
	Format          string            `json:"format"`
	MaxPages        int               `json:"maxPages"`
	ConfigID        string            `json:"configId"`
	TargetSessionID string            `json:"targetSessionId"`
	AgentConfig     map[string]string `json:"agentConfig"`
	UserText        string            `json:"userText"`
	Location        HandoffLocation   `json:"location"`
}
type HandoffBudget struct {
	Window     int64  `json:"window"`
	NativeUsed int64  `json:"nativeUsed"`
	Current    int64  `json:"current"`
	Reserve    int64  `json:"reserve"`
	History    int64  `json:"history"`
	Used       int64  `json:"used"`
	Source     string `json:"source"`
	OverBudget bool   `json:"overBudget"`
}
type HandoffImagePage struct {
	Index    int    `json:"index"`
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
	SHA256   string `json:"sha256"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Bytes    int    `json:"bytes"`
}
type HandoffImageBudget struct {
	MaxPages        int `json:"maxPages"`
	Pages           int `json:"pages"`
	Bytes           int `json:"bytes"`
	MaxBytes        int `json:"maxBytes"`
	MaxRequestBytes int `json:"maxRequestBytes"`
}
type HandoffPreview struct {
	Format          string                `json:"format"`
	Pages           []HandoffImagePage    `json:"pages"`
	ImageBudget     HandoffImageBudget    `json:"imageBudget"`
	BlockedReason   string                `json:"blockedReason"`
	Capability      string                `json:"capability"`
	HistoryText     string                `json:"historyText"`
	ID              string                `json:"id"`
	InstanceID      string                `json:"instanceId"`
	OperationID     string                `json:"operationId"`
	SourceSessionID string                `json:"sourceSessionId"`
	SnapshotID      string                `json:"snapshotId"`
	Text            string                `json:"text"`
	UserText        string                `json:"userText"`
	Request         HandoffPreviewRequest `json:"request"`
	CreatedAt       time.Time             `json:"createdAt"`
	ExpiresAt       time.Time             `json:"expiresAt"`
	SelectedCount   int                   `json:"selectedCount"`
	OmittedCount    int                   `json:"omittedCount"`
	GapCount        int                   `json:"gapCount"`
	Budget          HandoffBudget         `json:"budget"`
}
type HandoffSendRequest struct {
	OperationID    string          `json:"operationId"`
	UpstreamChoice *UpstreamChoice `json:"upstreamChoice,omitempty"`
}
type HandoffOperation struct {
	ID               string            `json:"id"`
	PreviewID        string            `json:"previewId"`
	InstanceID       string            `json:"instanceId"`
	State            string            `json:"state"`
	TargetSessionID  string            `json:"targetSessionId"`
	Error            string            `json:"error"`
	WorktreeID       string            `json:"worktreeId"`
	WorktreePath     string            `json:"worktreePath"`
	StartedAt        time.Time         `json:"startedAt"`
	FinishedAt       *time.Time        `json:"finishedAt"`
	WorktreeUpstream *WorktreeUpstream `json:"worktreeUpstream,omitempty"`
}
