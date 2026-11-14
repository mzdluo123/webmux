package workspace

// ConflictSide describes one side of a 3-way conflict (Base, Left, or Right),
// carrying real branch/ref names, commit OIDs, operation roles, and stage metadata
// without inverting Left/Right during rebase.
type ConflictSide struct {
	Side       string `json:"side"` // "base" | "left" | "right"
	Label      string `json:"label"`
	Role       string `json:"role"`
	RefName    string `json:"refName,omitempty"`
	CommitOID  string `json:"commitOid,omitempty"`
	Subject    string `json:"subject,omitempty"`
	Stage      int    `json:"stage"` // 1, 2, or 3
	Present    bool   `json:"present"`
	Mode       string `json:"mode,omitempty"`
	BlobOID    string `json:"blobOid,omitempty"`
	SizeBytes  int    `json:"sizeBytes"`
	Binary     bool   `json:"binary"`
	Text       string `json:"text,omitempty"`
	LineEnding string `json:"lineEnding,omitempty"` // dominant "lf" | "crlf" | "mixed" | "none"
}

// ConflictSegment represents one aligned 3-way region across Base, Left, and Right.
type ConflictSegment struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"` // "common" | "change_left" | "change_right" | "change_both_same" | "conflict"
	BaseStart  int      `json:"baseStart"`
	BaseLines  []string `json:"baseLines"`
	LeftStart  int      `json:"leftStart"`
	LeftLines  []string `json:"leftLines"`
	RightStart int      `json:"rightStart"`
	RightLines []string `json:"rightLines"`
}

// ConflictSnapshotIdentity binds SaveConflictResult and ResolveConflict to the
// exact observed unmerged index stages and on-disk worktree file digest.
type ConflictSnapshotIdentity struct {
	Path           string `json:"path"`
	OperationKind  string `json:"operationKind,omitempty"`
	Stage1Mode     string `json:"stage1Mode,omitempty"`
	Stage1OID      string `json:"stage1Oid,omitempty"`
	Stage2Mode     string `json:"stage2Mode,omitempty"`
	Stage2OID      string `json:"stage2Oid,omitempty"`
	Stage3Mode     string `json:"stage3Mode,omitempty"`
	Stage3OID      string `json:"stage3Oid,omitempty"`
	LeftStage      int    `json:"leftStage"`  // 2 for merge/cherry-pick/revert, 3 for rebase
	RightStage     int    `json:"rightStage"` // 3 for merge/cherry-pick/revert, 2 for rebase
	WorktreeMode   string `json:"worktreeMode,omitempty"`
	WorktreeSHA256 string `json:"worktreeSha256,omitempty"`
}

// ConflictFileDetail returns the full 3-way inspection payload for one conflicted path.
type ConflictFileDetail struct {
	RepositoryID          string                   `json:"repositoryId"`
	WorktreeID            string                   `json:"worktreeId"`
	WorktreePath          string                   `json:"worktreePath"`
	Path                  string                   `json:"path"`
	XY                    string                   `json:"xy"`
	Kind                  string                   `json:"kind"` // "text" | "binary" | "delete_modify" | "submodule" | "symlink" | "too_large"
	TextMergeable         bool                     `json:"textMergeable"`
	DegradeReason         string                   `json:"degradeReason,omitempty"`
	OperationKind         string                   `json:"operationKind,omitempty"`
	Identity              ConflictSnapshotIdentity `json:"identity"`
	Base                  ConflictSide             `json:"base"`
	Left                  ConflictSide             `json:"left"`
	Right                 ConflictSide             `json:"right"`
	Segments              []ConflictSegment        `json:"segments"`
	InitialResultText     string                   `json:"initialResultText"`
	DiskText              string                   `json:"diskText,omitempty"`
	DiskHasConflictMarker bool                     `json:"diskHasConflictMarker"`
	DiskModifiedFromBase  bool                     `json:"diskModifiedFromBase"`
}

// SaveConflictResultRequest writes merged text to the worktree file on disk
// without staging or clearing unmerged index stages.
type SaveConflictResultRequest struct {
	RepositoryID string                   `json:"repositoryId"`
	WorktreeID   string                   `json:"worktreeId"`
	Identity     ConflictSnapshotIdentity `json:"identity"`
	ResultText   string                   `json:"resultText"`
}

// SaveConflictResultResponse returns the updated worktree file identity after
// saving the merged draft to disk.
type SaveConflictResultResponse struct {
	Path     string                   `json:"path"`
	Identity ConflictSnapshotIdentity `json:"identity"`
}

// ResolveConflictRequest marks a conflicted path as resolved in the real Git index,
// either from confirmed merged text ("text") or by explicit side/delete selection
// ("choose_left" | "choose_right" | "delete").
type ResolveConflictRequest struct {
	RepositoryID string                   `json:"repositoryId"`
	WorktreeID   string                   `json:"worktreeId"`
	Identity     ConflictSnapshotIdentity `json:"identity"`
	Resolution   string                   `json:"resolution"` // "text" | "choose_left" | "choose_right" | "delete"
	ResultText   string                   `json:"resultText,omitempty"`
}

// ResolveConflictResponse reports the updated in-progress operation state after
// resolving one conflicted path in the Git index.
type ResolveConflictResponse struct {
	Path       string              `json:"path"`
	InProgress InProgressOperation `json:"inProgress"`
}
