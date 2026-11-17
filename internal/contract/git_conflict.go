package contract

// Git workbench V2 3-way conflict inspection, worktree draft saving, and
// index conflict resolution wire contracts.

// GitConflictSide describes Base (stage 1), Left, or Right in a 3-way conflict.
type GitConflictSide struct {
	Side       string `json:"side"`
	Label      string `json:"label"`
	Role       string `json:"role"`
	RefName    string `json:"refName,omitempty"`
	CommitOID  string `json:"commitOid,omitempty"`
	Subject    string `json:"subject,omitempty"`
	Stage      int    `json:"stage"`
	Present    bool   `json:"present"`
	Mode       string `json:"mode,omitempty"`
	BlobOID    string `json:"blobOid,omitempty"`
	SizeBytes  int    `json:"sizeBytes"`
	Binary     bool   `json:"binary"`
	Text       string `json:"text,omitempty"`
	LineEnding string `json:"lineEnding,omitempty"`
}

// GitConflictSegment represents one aligned 3-way region across Base, Left, and Right.
type GitConflictSegment struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	BaseStart  int      `json:"baseStart"`
	BaseLines  []string `json:"baseLines"`
	LeftStart  int      `json:"leftStart"`
	LeftLines  []string `json:"leftLines"`
	RightStart int      `json:"rightStart"`
	RightLines []string `json:"rightLines"`
}

// GitConflictSnapshotIdentity binds conflict save and resolve operations to the
// exact observed unmerged index stages and on-disk worktree SHA-256.
type GitConflictSnapshotIdentity struct {
	Path           string `json:"path"`
	OperationKind  string `json:"operationKind,omitempty"`
	Stage1Mode     string `json:"stage1Mode,omitempty"`
	Stage1OID      string `json:"stage1Oid,omitempty"`
	Stage2Mode     string `json:"stage2Mode,omitempty"`
	Stage2OID      string `json:"stage2Oid,omitempty"`
	Stage3Mode     string `json:"stage3Mode,omitempty"`
	Stage3OID      string `json:"stage3Oid,omitempty"`
	LeftStage      int    `json:"leftStage"`
	RightStage     int    `json:"rightStage"`
	WorktreeMode   string `json:"worktreeMode,omitempty"`
	WorktreeSHA256 string `json:"worktreeSha256,omitempty"`
}

// GitConflictFileDetail answers GET .../git/conflict?path=...
type GitConflictFileDetail struct {
	RepositoryID          string                      `json:"repositoryId"`
	WorktreeID            string                      `json:"worktreeId"`
	WorktreePath          string                      `json:"worktreePath"`
	Path                  string                      `json:"path"`
	XY                    string                      `json:"xy"`
	Kind                  string                      `json:"kind"`
	TextMergeable         bool                        `json:"textMergeable"`
	DegradeReason         string                      `json:"degradeReason,omitempty"`
	OperationKind         string                      `json:"operationKind,omitempty"`
	Identity              GitConflictSnapshotIdentity `json:"identity"`
	Base                  GitConflictSide             `json:"base"`
	Left                  GitConflictSide             `json:"left"`
	Right                 GitConflictSide             `json:"right"`
	Segments              []GitConflictSegment        `json:"segments"`
	InitialResultText     string                      `json:"initialResultText"`
	DiskText              string                      `json:"diskText,omitempty"`
	DiskHasConflictMarker bool                        `json:"diskHasConflictMarker"`
	DiskModifiedFromBase  bool                        `json:"diskModifiedFromBase"`
}

// GitSaveConflictRequest is the body of POST .../git/conflict/save.
type GitSaveConflictRequest struct {
	Identity   GitConflictSnapshotIdentity `json:"identity"`
	ResultText string                      `json:"resultText"`
}

// GitSaveConflictResponse answers POST .../git/conflict/save.
type GitSaveConflictResponse struct {
	Path     string                      `json:"path"`
	Identity GitConflictSnapshotIdentity `json:"identity"`
}

// GitResolveConflictRequest is the body of POST .../git/conflict/resolve.
type GitResolveConflictRequest struct {
	Identity   GitConflictSnapshotIdentity `json:"identity"`
	Resolution string                      `json:"resolution"`
	ResultText string                      `json:"resultText,omitempty"`
}

// GitResolveConflictResponse answers POST .../git/conflict/resolve.
type GitResolveConflictResponse struct {
	Path       string                 `json:"path"`
	InProgress GitInProgressOperation `json:"inProgress"`
}
