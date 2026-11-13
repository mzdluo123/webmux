package workspace

import (
	"bytes"
	"context"
	"strings"
)

// CommitParent describes one real parent of a commit in CommitDetail.
type CommitParent struct {
	OID      string `json:"oid"`
	ShortOID string `json:"shortOid"`
	Subject  string `json:"subject"`
}

// CommitFileChange describes one changed file of a commit relative to its
// selected parent (or the empty tree for a root commit).
type CommitFileChange struct {
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

// CommitDetail holds full metadata, parent summaries, refs, and changed files
// for one verified commit OID.
type CommitDetail struct {
	OID               string             `json:"oid"`
	ShortOID          string             `json:"shortOid"`
	Subject           string             `json:"subject"`
	Body              string             `json:"body"`
	AuthorName        string             `json:"authorName"`
	AuthorEmail       string             `json:"authorEmail"`
	AuthorAt          string             `json:"authorAt"`
	CommitterName     string             `json:"committerName"`
	CommitterEmail    string             `json:"committerEmail"`
	CommitterAt       string             `json:"committerAt"`
	DistinctCommitter bool               `json:"distinctCommitter"`
	Parents           []CommitParent     `json:"parents"`
	SelectedParent    string             `json:"selectedParent"`
	Refs              []HistoryRefBadge  `json:"refs"`
	Files             []CommitFileChange `json:"files"`
}

// CommitDetail returns the full metadata, parent list, refs, and changed files
// of commitOID in registered repositoryID. For merge commits, parentOID=""
// compares against parent 1; callers may pass any real parent OID of commitOID
// to inspect changes relative to that parent.
func (s *Service) CommitDetail(ctx context.Context, repositoryID, commitOID, parentOID string) (CommitDetail, error) {
	if repositoryID == "" {
		return CommitDetail{}, problemf(ErrInvalid, "请选择已登记的仓库")
	}
	if !isObjectID(commitOID) {
		return CommitDetail{}, problemf(ErrInvalid, "commitOid 必须是完整的十六进制提交 OID")
	}
	if parentOID != "" && !isObjectID(parentOID) {
		return CommitDetail{}, problemf(ErrInvalid, "parentOid 必须是完整的十六进制提交 OID")
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return CommitDetail{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	if err := s.verifyIdentity(ctx, r); err != nil {
		return CommitDetail{}, err
	}
	anchor := r.entry.AnchorPath

	verifiedOut, err := ReadGit(ctx, anchor, 256, "rev-parse", "--verify", "--end-of-options", commitOID+"^{commit}")
	if err != nil || strings.TrimSpace(string(verifiedOut)) != commitOID {
		return CommitDetail{}, problemf(ErrNotFound, "提交不存在")
	}

	metaRaw, err := ReadGit(ctx, anchor, maxHistoryOutput, "log", "-1", "--no-show-signature",
		"--format=%H%x00%P%x00%s%x00%b%x00%an%x00%ae%x00%aI%x00%cn%x00%ce%x00%cI%x00",
		commitOID, "--")
	if err != nil {
		return CommitDetail{}, err
	}
	fields := bytes.Split(metaRaw, []byte{0})
	if len(fields) < 10 {
		return CommitDetail{}, problemf(ErrInternal, "提交详情输出不完整")
	}
	rawParentIDs := strings.Fields(strings.TrimSpace(string(fields[1])))
	parents, err := describeParentSummaries(ctx, anchor, rawParentIDs)
	if err != nil {
		return CommitDetail{}, err
	}

	selectedParent := ""
	if len(rawParentIDs) == 0 {
		if parentOID != "" {
			return CommitDetail{}, problemf(ErrInvalid, "根提交没有父提交可供选择")
		}
	} else if parentOID == "" {
		selectedParent = rawParentIDs[0]
	} else {
		found := false
		for _, p := range rawParentIDs {
			if p == parentOID {
				found = true
				break
			}
		}
		if !found {
			return CommitDetail{}, problemf(ErrInvalid, "指定的对比节点不是该提交的真实父提交")
		}
		selectedParent = parentOID
	}

	treesData, err := ReadGit(ctx, anchor, maxWorkspaceGitOutput, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return CommitDetail{}, err
	}
	trees, err := parseWorktrees(treesData, r.entry.ID)
	if err != nil {
		return CommitDetail{}, problemf(ErrUnavailable, "%s", err.Error())
	}
	refsMap, _, err := readHistoryRefs(ctx, anchor, trees)
	if err != nil {
		return CommitDetail{}, err
	}

	files, err := readCommitFileChanges(ctx, anchor, selectedParent, commitOID)
	if err != nil {
		return CommitDetail{}, err
	}

	shortOID := commitOID
	if len(shortOID) > 8 {
		shortOID = shortOID[:8]
	}
	authorName := string(fields[4])
	authorEmail := string(fields[5])
	authorAt := string(fields[6])
	committerName := string(fields[7])
	committerEmail := string(fields[8])
	committerAt := string(fields[9])
	badges := refsMap[commitOID]
	if badges == nil {
		badges = []HistoryRefBadge{}
	}

	return CommitDetail{
		OID:               commitOID,
		ShortOID:          shortOID,
		Subject:           string(fields[2]),
		Body:              strings.TrimRight(string(fields[3]), "\n"),
		AuthorName:        authorName,
		AuthorEmail:       authorEmail,
		AuthorAt:          authorAt,
		CommitterName:     committerName,
		CommitterEmail:    committerEmail,
		CommitterAt:       committerAt,
		DistinctCommitter: authorName != committerName || authorEmail != committerEmail || authorAt != committerAt,
		Parents:           parents,
		SelectedParent:    selectedParent,
		Refs:              badges,
		Files:             files,
	}, nil
}

func describeParentSummaries(ctx context.Context, anchor string, parentOIDs []string) ([]CommitParent, error) {
	if len(parentOIDs) == 0 {
		return []CommitParent{}, nil
	}
	args := append([]string{"log", "--no-walk=unsorted", "--no-show-signature", "--format=%H%x00%s%x00"}, parentOIDs...)
	data, err := ReadGit(ctx, anchor, maxWorkspaceGitOutput, append(args, "--")...)
	if err != nil {
		return nil, err
	}
	byOID := map[string]string{}
	fields := bytes.Split(data, []byte{0})
	for i := 0; i+1 < len(fields); i += 2 {
		oid := strings.TrimPrefix(string(fields[i]), "\n")
		byOID[oid] = string(fields[i+1])
	}
	out := make([]CommitParent, len(parentOIDs))
	for i, oid := range parentOIDs {
		short := oid
		if len(short) > 8 {
			short = short[:8]
		}
		out[i] = CommitParent{
			OID:      oid,
			ShortOID: short,
			Subject:  byOID[oid],
		}
	}
	return out, nil
}

func readCommitFileChanges(ctx context.Context, anchor, parentOID, commitOID string) ([]CommitFileChange, error) {
	var rawArgs, numstatArgs []string
	if parentOID == "" {
		rawArgs = []string{"diff-tree", "--root", "-r", "--raw", "-z", "--no-abbrev", "--no-ext-diff", "--no-textconv", "--find-renames", "--find-copies", commitOID, "--"}
		numstatArgs = []string{"diff-tree", "--root", "-r", "--numstat", "-z", "--no-ext-diff", "--no-textconv", "--no-renames", commitOID, "--"}
	} else {
		rawArgs = []string{"diff-tree", "-r", "--raw", "-z", "--no-abbrev", "--no-ext-diff", "--no-textconv", "--find-renames", "--find-copies", parentOID, commitOID, "--"}
		numstatArgs = []string{"diff-tree", "-r", "--numstat", "-z", "--no-ext-diff", "--no-textconv", "--no-renames", parentOID, commitOID, "--"}
	}

	binaryPaths := map[string]bool{}
	numData, err := ReadGit(ctx, anchor, maxHistoryOutput, numstatArgs...)
	if err != nil {
		return nil, err
	}
	for _, rec := range bytes.Split(numData, []byte{0}) {
		line := strings.TrimPrefix(string(rec), "\n")
		if line == "" {
			continue
		}
		// Root diff-tree prefixes the first record with the commit OID.
		if isObjectID(line) {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) == 3 && parts[0] == "-" && parts[1] == "-" {
			binaryPaths[parts[2]] = true
		}
	}

	files := []CommitFileChange{}
	phase := 0
	var current CommitFileChange
	err = ReadGitRecords(ctx, anchor, func(record string) error {
		trimmed := strings.TrimPrefix(record, "\n")
		if phase == 0 && isObjectID(trimmed) {
			return nil
		}
		switch phase {
		case 0:
			fields := strings.Fields(trimmed)
			if len(fields) != 5 || !strings.HasPrefix(fields[0], ":") {
				return problemf(ErrInternal, "无效提交文件差异输出")
			}
			oldMode := strings.TrimPrefix(fields[0], ":")
			newMode := fields[1]
			current = CommitFileChange{
				OldMode:     oldMode,
				NewMode:     newMode,
				OldOID:      fields[2],
				NewOID:      fields[3],
				Status:      fields[4],
				ModeChanged: oldMode != "000000" && newMode != "000000" && oldMode != newMode,
			}
			phase = 1
		case 1:
			current.OldPath = record
			current.NewPath = record
			if strings.HasPrefix(current.Status, "R") || strings.HasPrefix(current.Status, "C") {
				phase = 2
			} else {
				current.Binary = binaryPaths[current.NewPath]
				files = append(files, current)
				phase = 0
			}
		case 2:
			current.NewPath = record
			current.Binary = binaryPaths[current.NewPath] || binaryPaths[current.OldPath]
			files = append(files, current)
			phase = 0
		}
		return nil
	}, rawArgs...)
	if err != nil {
		return nil, err
	}
	if phase != 0 {
		return nil, problemf(ErrInternal, "提交文件差异记录不完整")
	}
	return files, nil
}
