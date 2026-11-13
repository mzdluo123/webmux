package contract

import "time"

// GitBranchRenamePreviewRequest is the body of POST /api/repositories/{id}/branches/rename-preview.
type GitBranchRenamePreviewRequest struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
}

// GitBranchRenamePreview answers POST /api/repositories/{id}/branches/rename-preview.
type GitBranchRenamePreview struct {
	ConfirmationID    string        `json:"confirmationId"`
	ExpiresAt         *time.Time    `json:"expiresAt"`
	RepositoryID      string        `json:"repositoryId"`
	OldName           string        `json:"oldName"`
	NewName           string        `json:"newName"`
	HeadOID           string        `json:"headOid"`
	Subject           string        `json:"subject"`
	Upstream          string        `json:"upstream,omitempty"`
	AffectedWorktrees []Worktree    `json:"affectedWorktrees"`
	Sessions          []SessionInfo `json:"sessions"`
	Blockers          []Issue       `json:"blockers"`
	Warnings          []Issue       `json:"warnings"`
}

// GitBranchRenameRequest is the body of POST /api/repositories/{id}/branches/rename.
type GitBranchRenameRequest struct {
	ConfirmationID      string `json:"confirmationId"`
	AcknowledgeSessions bool   `json:"acknowledgeSessions"`
	ClientRequestID     string `json:"clientRequestId,omitempty"`
}

// GitBranchUpstreamRequest is the body of POST /api/repositories/{id}/branches/upstream.
type GitBranchUpstreamRequest struct {
	Branch          string `json:"branch"`
	Upstream        string `json:"upstream,omitempty"`
	ClientRequestID string `json:"clientRequestId,omitempty"`
}

// GitBranchMutationResult answers branch rename and upstream tracking mutations.
type GitBranchMutationResult struct {
	OperationID string     `json:"operationId,omitempty"`
	Branch      string     `json:"branch"`
	Upstream    string     `json:"upstream,omitempty"`
	Repository  Repository `json:"repository"`
	Branches    Branches   `json:"branches"`
}

// GitTagCreateRequest is the body of POST /api/repositories/{id}/tags.
type GitTagCreateRequest struct {
	Name            string `json:"name"`
	TargetOID       string `json:"targetOid"`
	Annotated       bool   `json:"annotated"`
	Message         string `json:"message,omitempty"`
	ClientRequestID string `json:"clientRequestId,omitempty"`
}

// GitTagCreateResult answers POST /api/repositories/{id}/tags.
type GitTagCreateResult struct {
	OperationID string   `json:"operationId,omitempty"`
	Tag         Tag      `json:"tag"`
	Branches    Branches `json:"branches"`
}

// GitTagDeleteRequest is the body of POST /api/repositories/{id}/tags/delete.
type GitTagDeleteRequest struct {
	Name              string `json:"name"`
	ExpectedTargetOID string `json:"expectedTargetOid,omitempty"`
	ClientRequestID   string `json:"clientRequestId,omitempty"`
}

// GitTagDeleteResult answers POST /api/repositories/{id}/tags/delete.
type GitTagDeleteResult struct {
	OperationID     string   `json:"operationId,omitempty"`
	Name            string   `json:"name"`
	TargetOID       string   `json:"targetOid"`
	RecoveryCommand string   `json:"recoveryCommand"`
	Branches        Branches `json:"branches"`
}

// GitRemoteAddRequest is the body of POST /api/repositories/{id}/remotes.
type GitRemoteAddRequest struct {
	Name            string `json:"name"`
	FetchURL        string `json:"fetchUrl"`
	PushURL         string `json:"pushUrl,omitempty"`
	ClientRequestID string `json:"clientRequestId,omitempty"`
}

// GitRemoteUpdateRequest is the body of PATCH /api/repositories/{id}/remotes/{name}.
type GitRemoteUpdateRequest struct {
	FetchURL        *string `json:"fetchUrl,omitempty"`
	PushURL         *string `json:"pushUrl,omitempty"`
	ClearPushURL    bool    `json:"clearPushUrl,omitempty"`
	ClientRequestID string  `json:"clientRequestId,omitempty"`
}

// GitRemoteRemoveRequest is the optional body of DELETE /api/repositories/{id}/remotes/{name}.
type GitRemoteRemoveRequest struct {
	ClientRequestID string `json:"clientRequestId,omitempty"`
}

// GitRemoteMutationResult answers remote add/update/remove mutations.
type GitRemoteMutationResult struct {
	OperationID string     `json:"operationId,omitempty"`
	Remotes     []Remote   `json:"remotes"`
	Repository  Repository `json:"repository"`
}

// GitFileHistoryRow summarizes one commit touching a tracked file along its rename chain.
type GitFileHistoryRow struct {
	OID               string   `json:"oid"`
	ShortOID          string   `json:"shortOid"`
	Parents           []string `json:"parents"`
	ComparedParentOID string   `json:"comparedParentOid,omitempty"`
	Subject           string   `json:"subject"`
	AuthorName        string   `json:"authorName"`
	AuthorEmail       string   `json:"authorEmail"`
	AuthorAt          string   `json:"authorAt"`
	Status            string   `json:"status"`
	Path              string   `json:"path"`
	OldPath           string   `json:"oldPath,omitempty"`
}

// GitFileHistoryPage answers GET /api/repositories/{id}/git/file-history.
type GitFileHistoryPage struct {
	Status     string              `json:"status"`
	Path       string              `json:"path"`
	StartOID   string              `json:"startOid,omitempty"`
	Commits    []GitFileHistoryRow `json:"commits"`
	NextCursor string              `json:"nextCursor,omitempty"`
}

// GitBlameCommit holds deduplicated commit metadata for lines in a Blame result.
type GitBlameCommit struct {
	OID          string `json:"oid"`
	ShortOID     string `json:"shortOid"`
	Summary      string `json:"summary"`
	AuthorName   string `json:"authorName"`
	AuthorEmail  string `json:"authorEmail"`
	AuthorAt     string `json:"authorAt"`
	PreviousOID  string `json:"previousOid,omitempty"`
	PreviousPath string `json:"previousPath,omitempty"`
	Boundary     bool   `json:"boundary,omitempty"`
}

// GitBlameLine holds per-line provenance and content for a blamed file.
type GitBlameLine struct {
	LineNumber   int    `json:"lineNumber"`
	OrigLine     int    `json:"origLine"`
	CommitOID    string `json:"commitOid"`
	ShortOID     string `json:"shortOid"`
	OrigPath     string `json:"origPath,omitempty"`
	PreviousOID  string `json:"previousOid,omitempty"`
	PreviousPath string `json:"previousPath,omitempty"`
	Text         string `json:"text"`
}

// GitBlameResult answers GET /api/repositories/{id}/git/blame.
type GitBlameResult struct {
	CommitOID     string           `json:"commitOid"`
	ShortOID      string           `json:"shortOid"`
	Path          string           `json:"path"`
	BlobOID       string           `json:"blobOid,omitempty"`
	Mode          string           `json:"mode,omitempty"`
	Kind          string           `json:"kind"`
	DegradeReason string           `json:"degradeReason,omitempty"`
	TotalLines    int              `json:"totalLines"`
	Commits       []GitBlameCommit `json:"commits"`
	Lines         []GitBlameLine   `json:"lines"`
}
