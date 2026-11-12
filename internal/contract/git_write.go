package contract

import "time"

// Git workbench staging and commit shapes for
// /api/repositories/{id}/worktrees/{worktreeId}/git/*.

// GitStageFileIdentity binds a stage or unstage request to the exact
// server-observed content and modes on both sides of the comparison.
type GitStageFileIdentity struct {
	Area         string `json:"area"`
	Path         string `json:"path"`
	OldPath      string `json:"oldPath,omitempty"`
	OldBaseOID   string `json:"oldBaseOid,omitempty"`
	OldBaseMode  string `json:"oldBaseMode,omitempty"`
	BaseOID      string `json:"baseOid,omitempty"`
	BaseMode     string `json:"baseMode,omitempty"`
	TargetOID    string `json:"targetOid,omitempty"`
	TargetSHA256 string `json:"targetSha256,omitempty"`
	TargetMode   string `json:"targetMode,omitempty"`
}

// GitStagingEntry summarizes one staged, unstaged, untracked, or conflicted
// path in a worktree.
type GitStagingEntry struct {
	Path               string               `json:"path"`
	OldPath            string               `json:"oldPath,omitempty"`
	Area               string               `json:"area"`
	Status             string               `json:"status"`
	HunkStageAvailable bool                 `json:"hunkStageAvailable"`
	DegradeReason      string               `json:"degradeReason,omitempty"`
	Identity           GitStageFileIdentity `json:"identity"`
}

// GitStagingOverview answers GET .../git/staging.
type GitStagingOverview struct {
	Unborn     bool              `json:"unborn"`
	HeadOID    string            `json:"headOid,omitempty"`
	Staged     []GitStagingEntry `json:"staged"`
	Unstaged   []GitStagingEntry `json:"unstaged"`
	Conflicted []GitStagingEntry `json:"conflicted"`
}

// GitStagingHunkLine is one line inside a stageable unified diff hunk.
type GitStagingHunkLine struct {
	Kind       string `json:"kind"`
	OldLine    int    `json:"oldLine,omitempty"`
	NewLine    int    `json:"newLine,omitempty"`
	Text       string `json:"text"`
	LineEnding string `json:"lineEnding"`
}

// GitStagingHunk is one contiguous diff hunk with a snapshot-bound ID.
type GitStagingHunk struct {
	ID       string               `json:"id"`
	Header   string               `json:"header"`
	OldStart int                  `json:"oldStart"`
	OldLines int                  `json:"oldLines"`
	NewStart int                  `json:"newStart"`
	NewLines int                  `json:"newLines"`
	Lines    []GitStagingHunkLine `json:"lines"`
}

// GitStagingFileDiff answers GET .../git/staging/diff.
type GitStagingFileDiff struct {
	Identity           GitStageFileIdentity `json:"identity"`
	Conflicted         bool                 `json:"conflicted"`
	Binary             bool                 `json:"binary"`
	HunkStageAvailable bool                 `json:"hunkStageAvailable"`
	DegradeReason      string               `json:"degradeReason,omitempty"`
	Hunks              []GitStagingHunk     `json:"hunks"`
}

// GitStageRequest is the body of POST .../git/stage and POST .../git/unstage.
type GitStageRequest struct {
	Identity GitStageFileIdentity `json:"identity"`
	HunkIDs  []string             `json:"hunkIds,omitempty"`
}

// GitCommitPreviewRequest is the optional empty JSON body of
// POST .../git/commit-preview.
type GitCommitPreviewRequest struct{}

// GitCommitIdentity summarizes effective Git author, committer, and signing
// configuration for a worktree commit preview.
type GitCommitIdentity struct {
	AuthorName     string `json:"authorName"`
	AuthorEmail    string `json:"authorEmail"`
	CommitterName  string `json:"committerName"`
	CommitterEmail string `json:"committerEmail"`
	SignEnabled    bool   `json:"signEnabled"`
	SignFormat     string `json:"signFormat,omitempty"`
}

// GitCommitPreview answers POST .../git/commit-preview. With blockers
// ConfirmationID is "" and ExpiresAt is null.
type GitCommitPreview struct {
	ConfirmationID  string                `json:"confirmationId"`
	ExpiresAt       *time.Time            `json:"expiresAt"`
	RepositoryID    string                `json:"repositoryId"`
	WorktreeID      string                `json:"worktreeId"`
	WorktreePath    string                `json:"worktreePath"`
	HeadState       string                `json:"headState"`
	Branch          string                `json:"branch,omitempty"`
	HeadOID         string                `json:"headOid,omitempty"`
	ExpectedTreeOID string                `json:"expectedTreeOid,omitempty"`
	IndexDigest     string                `json:"indexDigest"`
	StagedFiles     []GitCommitFileChange `json:"stagedFiles"`
	UnstagedCount   int                   `json:"unstagedCount"`
	UntrackedCount  int                   `json:"untrackedCount"`
	ConflictedCount int                   `json:"conflictedCount"`
	Operation       string                `json:"operation,omitempty"`
	Identity        GitCommitIdentity     `json:"identity"`
	ExecutableHooks []string              `json:"executableHooks"`
	Blockers        []Issue               `json:"blockers"`
	Warnings        []Issue               `json:"warnings"`
}

// GitCommitRequest is the body of POST .../git/commit.
type GitCommitRequest struct {
	ConfirmationID  string `json:"confirmationId"`
	ClientRequestID string `json:"clientRequestId,omitempty"`
	Message         string `json:"message"`
}

// GitCommitResult answers POST .../git/commit after reflog attribution and
// {oid, parents, tree} verification.
type GitCommitResult struct {
	OperationID     string   `json:"operationId,omitempty"`
	Outcome         string   `json:"outcome"`
	CommitOID       string   `json:"commitOid,omitempty"`
	Parents         []string `json:"parents"`
	TreeOID         string   `json:"treeOid,omitempty"`
	ExpectedTreeOID string   `json:"expectedTreeOid"`
	Subject         string   `json:"subject,omitempty"`
	ReviewReasons   []string `json:"reviewReasons"`
}

// GitConflict is the 409 JSON body of Git workbench endpoints, carrying a
// machine-readable Reason ("stale", "busy", "readonly", "context_mismatch",
// "no_task_baseline", "conflicted", "blocked", "commit_failed",
// "idempotency_conflict", "uncertain", or "draft_conflict").
type GitConflict struct {
	Error        string          `json:"error"`
	Reason       string          `json:"reason"`
	CurrentDraft *GitCommitDraft `json:"currentDraft,omitempty"`
}
