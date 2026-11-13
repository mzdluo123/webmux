package contract

import "time"

// Git workbench read-only shapes for /api/repositories/{id}/git/*.

type GitContext struct {
	ID     string `json:"contextId"`
	Root   string `json:"rootPath"`
	Kind   string `json:"rootKind"`
	Browse bool   `json:"browseAvailable"`
	Diff   bool   `json:"diffAvailable"`
	Ignore bool   `json:"ignoreAvailable"`
	Reason string `json:"reason"`
	Write  bool   `json:"writableAvailable"`
}

type GitDocument struct {
	NodeID        string `json:"nodeId"`
	Path          string `json:"path"`
	Absolute      string `json:"absolutePath"`
	Source        string `json:"source"`
	OID           string `json:"oid"`
	Version       string `json:"contentVersion"`
	Kind          string `json:"kind"`
	Text          string `json:"text"`
	Lines         int    `json:"lineCount"`
	Referenceable bool   `json:"referenceable"`
	Reason        string `json:"reason"`
	MediaType     string `json:"mediaType"`
	Size          int64  `json:"size"`
	Downloadable  bool   `json:"downloadable"`
}

type GitFile struct {
	Snapshot string      `json:"snapshotId"`
	Document GitDocument `json:"document"`
	Checked  time.Time   `json:"checkedAt"`
}

type GitChange struct {
	NodeID        string `json:"nodeId"`
	ID            string `json:"changeId"`
	Group         string `json:"group"`
	OldPath       string `json:"oldPath"`
	NewPath       string `json:"newPath"`
	Status        string `json:"status"`
	OldOID        string `json:"oldOid"`
	NewOID        string `json:"newOid"`
	OldMode       string `json:"oldMode"`
	NewMode       string `json:"newMode"`
	ReviewVersion string `json:"reviewVersion,omitempty"`
}

type GitChanges struct {
	Next    string      `json:"nextCursor"`
	Total   int         `json:"total"`
	Entries []GitChange `json:"entries"`
	Version string      `json:"contentVersion"`
	Checked time.Time   `json:"checkedAt"`
}

type GitDiff struct {
	Snapshot string      `json:"snapshotId"`
	Old      GitDocument `json:"old"`
	New      GitDocument `json:"new"`
	Kind     string      `json:"kind"`
	Reason   string      `json:"reason"`
	Checked  time.Time   `json:"checkedAt"`
	Change   GitChange   `json:"change"`
}

// GitCompare answers GET /api/repositories/{id}/git/compare with the changed
// file page and, when path or changeId is supplied, the frozen file diff.
type GitCompare struct {
	BaseOID   string     `json:"baseOid"`
	TargetOID string     `json:"targetOid"`
	Changes   GitChanges `json:"changes"`
	Diff      *GitDiff   `json:"diff,omitempty"`
}

// GitRefBadge decorates a commit with a HEAD, local branch, remote-tracking
// branch, or tag reference.
type GitRefBadge struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	WorktreeID string `json:"worktreeId,omitempty"`
}

// GitHistoryEdge connects a commit row's graph lane to a parent or filtered
// ancestor lane on the next row.
type GitHistoryEdge struct {
	FromLane  int    `json:"fromLane"`
	ToLane    int    `json:"toLane"`
	TargetOID string `json:"targetOid"`
	Kind      string `json:"kind"`
}

// GitHistoryRow is one topological commit row in GET /api/repositories/{id}/git/history.
type GitHistoryRow struct {
	OID                  string           `json:"oid"`
	ShortOID             string           `json:"shortOid"`
	Parents              []string         `json:"parents"`
	Subject              string           `json:"subject"`
	AuthorName           string           `json:"authorName"`
	AuthorEmail          string           `json:"authorEmail"`
	AuthorAt             string           `json:"authorAt"`
	CommitterName        string           `json:"committerName"`
	CommitterAt          string           `json:"committerAt"`
	Refs                 []GitRefBadge    `json:"refs"`
	Lane                 int              `json:"lane"`
	ActiveLanes          []int            `json:"activeLanes"`
	Edges                []GitHistoryEdge `json:"edges"`
	DirectParentsMissing bool             `json:"directParentsMissing"`
}

// GitHistoryPage answers GET /api/repositories/{id}/git/history.
type GitHistoryPage struct {
	Status        string          `json:"status"`
	Reason        string          `json:"reason,omitempty"`
	Scope         string          `json:"scope"`
	LastFetchedAt *time.Time      `json:"lastFetchedAt,omitempty"`
	Upstream      string          `json:"upstream,omitempty"`
	RefsChanged   bool            `json:"refsChanged"`
	NextCursor    string          `json:"nextCursor"`
	Rows          []GitHistoryRow `json:"rows"`
}

// GitCommitParent summarizes one real parent of a commit.
type GitCommitParent struct {
	OID      string `json:"oid"`
	ShortOID string `json:"shortOid"`
	Subject  string `json:"subject"`
}

// GitCommitFileChange describes one changed file relative to the compared parent
// or the staged index relative to HEAD.
type GitCommitFileChange struct {
	OldPath     string `json:"oldPath"`
	NewPath     string `json:"newPath"`
	Status      string `json:"status"`
	OldMode     string `json:"oldMode"`
	NewMode     string `json:"newMode"`
	OldOID      string `json:"oldOid"`
	NewOID      string `json:"newOid"`
	ModeChanged bool   `json:"modeChanged"`
	Binary      bool   `json:"binary"`
}

// GitCommitDetail answers GET /api/repositories/{id}/git/commits/{oid}.
type GitCommitDetail struct {
	OID               string                `json:"oid"`
	ShortOID          string                `json:"shortOid"`
	Subject           string                `json:"subject"`
	Body              string                `json:"body"`
	Message           string                `json:"message"`
	AuthorName        string                `json:"authorName"`
	AuthorEmail       string                `json:"authorEmail"`
	AuthorAt          string                `json:"authorAt"`
	CommitterName     string                `json:"committerName"`
	CommitterEmail    string                `json:"committerEmail"`
	CommitterAt       string                `json:"committerAt"`
	DistinctCommitter bool                  `json:"distinctCommitter"`
	Parents           []GitCommitParent     `json:"parents"`
	ComparedParentOID string                `json:"comparedParentOid"`
	Refs              []GitRefBadge         `json:"refs"`
	Files             []GitCommitFileChange `json:"files"`
}
