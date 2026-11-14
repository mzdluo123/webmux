package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHistoryLimit = 100
	maxHistoryLimit     = 200
	maxHistoryOutput    = 16 << 20
)

// HistoryScope names one of the history ranges.
const (
	HistoryScopeAll      = "all"
	HistoryScopeBranch   = "branch"
	HistoryScopeTask     = "task"
	HistoryScopeToPush   = "to-push"
	HistoryScopeIncoming = "incoming"
)

// HistoryStatus classifies the outcome of resolving a HistoryQuery's scope.
const (
	HistoryStatusReady          = "ready"
	HistoryStatusEmpty          = "empty"
	HistoryStatusUnborn         = "unborn"
	HistoryStatusNoWorktree     = "no_worktree"
	HistoryStatusNoTaskBaseline = "no_task_baseline"
	HistoryStatusNoUpstream     = "no_upstream"
	HistoryStatusUpstreamGone   = "upstream_gone"
	HistoryStatusDetachedNoRef  = "detached_no_branch"
)

// HistoryQuery parameters are validated and passed as literal values, never as
// shell or raw revision expressions.
type HistoryQuery struct {
	RepositoryID string
	WorktreeID   string
	Scope        string
	Branch       string
	Query        string
	Author       string
	Since        string
	Until        string
	Path         string
	Limit        int
	Cursor       string
}

// HistoryRefBadge decorates a commit with a local branch, remote-tracking ref,
// tag, or worktree HEAD pointer.
type HistoryRefBadge struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"` // "local" | "remote" | "tag" | "head"
	WorktreeID string `json:"worktreeId,omitempty"`
}

// HistoryEdge describes one outgoing parent or continuation edge from a row.
type HistoryEdge struct {
	FromLane  int    `json:"fromLane"`
	ToLane    int    `json:"toLane"`
	TargetOID string `json:"targetOid"`
	Kind      string `json:"kind"` // "direct" | "indirect" | "boundary"
}

// HistoryRow is one commit entry in a paginated history response, carrying its
// real parent OIDs alongside deterministic graph lane assignments.
type HistoryRow struct {
	OID           string            `json:"oid"`
	ShortOID      string            `json:"shortOid"`
	Parents       []string          `json:"parents"`
	Subject       string            `json:"subject"`
	AuthorName    string            `json:"authorName"`
	AuthorEmail   string            `json:"authorEmail"`
	AuthorAt      string            `json:"authorAt"`
	CommitterName string            `json:"committerName"`
	CommitterAt   string            `json:"committerAt"`
	Refs          []HistoryRefBadge `json:"refs"`
	Lane          int               `json:"lane"`
	ActiveLanes   []int             `json:"activeLanes"`
	Edges         []HistoryEdge     `json:"edges"`
	// DirectParentsMissing marks when at least one real parent was filtered out
	// between this row and the next visible row.
	DirectParentsMissing bool `json:"directParentsMissing"`
}

// HistoryPage is one bounded page of repository/worktree commit history.
type HistoryPage struct {
	Status        string        `json:"status"`
	Reason        string        `json:"reason"`
	Scope         string        `json:"scope"`
	Commits       []HistoryRow  `json:"commits"`
	NextCursor    string        `json:"nextCursor"`
	RefsChanged   bool          `json:"refsChanged"`
	LastFetchedAt *time.Time    `json:"lastFetchedAt"`
	Upstream      string        `json:"upstream,omitempty"`
	TaskBaseline  *TaskBaseline `json:"taskBaseline,omitempty"`
}

type historyScopeSpec struct {
	status   string
	reason   string
	upstream string
	include  []string
	exclude  []string
}

type historyCursor struct {
	filterDigest string
	offset       int
	include      []string
	exclude      []string
	lanes        []string
}

func validateCleanHistoryPath(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	if strings.ContainsRune(p, '\x00') || filepath.IsAbs(p) || strings.HasPrefix(p, "-") {
		return "", problemf(ErrInvalid, "筛选路径无效")
	}
	cleaned := strings.TrimPrefix(filepath.ToSlash(p), "./")
	for _, part := range strings.Split(cleaned, "/") {
		if part == "" || part == "." || part == ".." || strings.EqualFold(part, ".git") {
			return "", problemf(ErrInvalid, "筛选路径越界或指向 Git 内部目录")
		}
	}
	return cleaned, nil
}

func validateLiteralFilter(label, value string) error {
	if strings.ContainsRune(value, '\x00') {
		return problemf(ErrInvalid, "%s包含非法字符", label)
	}
	return nil
}

func validateTimeFilter(label, value string) error {
	if value == "" {
		return nil
	}
	if strings.ContainsRune(value, '\x00') || strings.HasPrefix(value, "-") {
		return problemf(ErrInvalid, "%s格式无效", label)
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return nil
	}
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return nil
	}
	return problemf(ErrInvalid, "%s需使用 ISO-8601 或 YYYY-MM-DD 格式", label)
}

func historyFilterDigest(q HistoryQuery, cleanPath string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		q.RepositoryID,
		q.WorktreeID,
		q.Scope,
		q.Branch,
		q.Query,
		q.Author,
		q.Since,
		q.Until,
		cleanPath,
	}, "\x00")))
	return hex.EncodeToString(sum[:16])
}

func encodeHistoryCursor(c historyCursor) string {
	parts := []string{
		"v1",
		c.filterDigest,
		strconv.Itoa(c.offset),
		strings.Join(c.include, ","),
		strings.Join(c.exclude, ","),
		strings.Join(c.lanes, ","),
	}
	return base64.RawURLEncoding.EncodeToString([]byte(strings.Join(parts, "\x1f")))
}

func decodeHistoryCursor(token string) (historyCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return historyCursor{}, problemf(ErrInvalid, "无效历史分页游标")
	}
	parts := strings.Split(string(raw), "\x1f")
	if len(parts) != 6 || parts[0] != "v1" || parts[1] == "" {
		return historyCursor{}, problemf(ErrInvalid, "无效历史分页游标")
	}
	offset, err := strconv.Atoi(parts[2])
	if err != nil || offset <= 0 {
		return historyCursor{}, problemf(ErrInvalid, "无效历史分页偏移")
	}
	parseOIDs := func(csv string, allowEmptySlot bool) ([]string, error) {
		if csv == "" {
			return nil, nil
		}
		items := strings.Split(csv, ",")
		out := make([]string, len(items))
		for i, item := range items {
			if item == "" && allowEmptySlot {
				continue
			}
			if !isObjectID(item) {
				return nil, problemf(ErrInvalid, "无效历史分页锚点")
			}
			out[i] = item
		}
		return out, nil
	}
	inc, err := parseOIDs(parts[3], false)
	if err != nil {
		return historyCursor{}, err
	}
	exc, err := parseOIDs(parts[4], false)
	if err != nil {
		return historyCursor{}, err
	}
	lanes, err := parseOIDs(parts[5], true)
	if err != nil {
		return historyCursor{}, err
	}
	return historyCursor{
		filterDigest: parts[1],
		offset:       offset,
		include:      inc,
		exclude:      exc,
		lanes:        lanes,
	}, nil
}

// History queries repository or worktree commit history by page without mutating
// the working tree, index, or remote state.
func (s *Service) History(ctx context.Context, q HistoryQuery) (HistoryPage, error) {
	if q.RepositoryID == "" {
		return HistoryPage{}, problemf(ErrInvalid, "请选择已登记的仓库")
	}
	scope := q.Scope
	if scope == "" {
		scope = HistoryScopeAll
	}
	switch scope {
	case HistoryScopeAll, HistoryScopeBranch, HistoryScopeTask, HistoryScopeToPush, HistoryScopeIncoming:
	default:
		return HistoryPage{}, problemf(ErrInvalid, "不支持的历史范围: %s", scope)
	}
	q.Scope = scope
	limit := q.Limit
	if limit <= 0 {
		limit = defaultHistoryLimit
	} else if limit > maxHistoryLimit {
		return HistoryPage{}, problemf(ErrInvalid, "单页条数上限为 %d", maxHistoryLimit)
	}
	cleanPath, err := validateCleanHistoryPath(q.Path)
	if err != nil {
		return HistoryPage{}, err
	}
	if err := validateLiteralFilter("关键字", q.Query); err != nil {
		return HistoryPage{}, err
	}
	if err := validateLiteralFilter("作者", q.Author); err != nil {
		return HistoryPage{}, err
	}
	if err := validateLiteralFilter("分支名称", q.Branch); err != nil {
		return HistoryPage{}, err
	}
	if err := validateTimeFilter("起始时间", q.Since); err != nil {
		return HistoryPage{}, err
	}
	if err := validateTimeFilter("截止时间", q.Until); err != nil {
		return HistoryPage{}, err
	}

	r, err := s.repository(q.RepositoryID)
	if err != nil {
		return HistoryPage{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	if err := s.verifyIdentity(ctx, r); err != nil {
		return HistoryPage{}, err
	}

	e := r.entry
	treesData, err := ReadGit(ctx, e.AnchorPath, maxWorkspaceGitOutput, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return HistoryPage{}, err
	}
	trees, err := parseWorktrees(treesData, e.ID)
	if err != nil {
		return HistoryPage{}, problemf(ErrUnavailable, "%s", err.Error())
	}

	refsMap, allHeads, err := readHistoryRefs(ctx, e.AnchorPath, trees)
	if err != nil {
		return HistoryPage{}, err
	}

	var taskBaseline *TaskBaseline
	if q.WorktreeID != "" {
		if scope == HistoryScopeTask {
			if err := s.requireTaskBaselines(); err != nil {
				return HistoryPage{}, err
			}
		}
		if s.requireTaskBaselines() == nil {
			taskBaseline = s.findTaskBaseline(q.RepositoryID, q.WorktreeID)
		}
	}

	spec, err := resolveHistoryScope(ctx, e.AnchorPath, q, trees, allHeads, taskBaseline)
	if err != nil {
		return HistoryPage{}, err
	}

	page := HistoryPage{
		Status:        spec.status,
		Reason:        spec.reason,
		Scope:         scope,
		Commits:       []HistoryRow{},
		LastFetchedAt: lastFetched(e.CommonDir),
		Upstream:      spec.upstream,
		TaskBaseline:  taskBaseline,
	}

	filterDigest := historyFilterDigest(q, cleanPath)
	offset := 0
	include := spec.include
	exclude := spec.exclude
	var initialLanes []string

	if q.Cursor != "" {
		cur, curErr := decodeHistoryCursor(q.Cursor)
		if curErr != nil {
			return HistoryPage{}, curErr
		}
		if cur.filterDigest != filterDigest {
			return HistoryPage{}, problemf(ErrInvalid, "筛选条件已变化，请从第一页重新查询")
		}
		if !equalStringSlices(cur.include, spec.include) || !equalStringSlices(cur.exclude, spec.exclude) {
			page.RefsChanged = true
		}
		if len(cur.include) == 0 {
			return page, nil
		}
		offset = cur.offset
		include = cur.include
		exclude = cur.exclude
		initialLanes = cur.lanes
		page.Status = HistoryStatusReady
		page.Reason = ""
	} else if spec.status != HistoryStatusReady {
		return page, nil
	} else if len(include) == 0 {
		return page, nil
	}

	filtered := q.Query != "" || q.Author != "" || q.Since != "" || q.Until != "" || cleanPath != ""
	rawRows, hasMore, err := queryHistoryRows(ctx, e.AnchorPath, include, exclude, q, cleanPath, offset, limit)
	if err != nil {
		return HistoryPage{}, err
	}
	var rewritten map[string][]string
	if filtered && len(rawRows) > 0 {
		rewritten, err = queryRewrittenParents(ctx, e.AnchorPath, include, exclude, q, cleanPath, offset, limit)
		if err != nil {
			return HistoryPage{}, err
		}
	}

	rows, nextLanes := layoutHistoryGraph(rawRows, initialLanes, refsMap, rewritten)
	page.Commits = rows
	if hasMore {
		page.NextCursor = encodeHistoryCursor(historyCursor{
			filterDigest: filterDigest,
			offset:       offset + len(rows),
			include:      include,
			exclude:      exclude,
			lanes:        nextLanes,
		})
	}
	return page, nil
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
