package workspace

import (
	"bufio"
	"bytes"
	"context"
	"strconv"
	"strings"
	"time"
)

// BlameQuery specifies parameters for single-file line provenance (`git blame`).
type BlameQuery struct {
	RepositoryID string
	WorktreeID   string
	Rev          string // "" / "HEAD" or full hex commit OID
	Path         string
}

// BlameCommit holds deduplicated metadata for one commit referenced by BlameResult.Lines.
type BlameCommit struct {
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

// BlameLine holds per-line provenance and UTF-8 line content (1-indexed LineNumber).
type BlameLine struct {
	LineNumber   int    `json:"lineNumber"`
	OrigLine     int    `json:"origLine"`
	CommitOID    string `json:"commitOid"`
	ShortOID     string `json:"shortOid"`
	OrigPath     string `json:"origPath,omitempty"`
	PreviousOID  string `json:"previousOid,omitempty"`
	PreviousPath string `json:"previousPath,omitempty"`
	Text         string `json:"text"`
}

// BlameResult represents the read-only blame view for a file at an explicit commit OID.
type BlameResult struct {
	CommitOID     string        `json:"commitOid"`
	ShortOID      string        `json:"shortOid"`
	Path          string        `json:"path"`
	BlobOID       string        `json:"blobOid,omitempty"`
	Mode          string        `json:"mode,omitempty"`
	Kind          string        `json:"kind"` // "text" | "binary" | "too_large" | "unsupported"
	DegradeReason string        `json:"degradeReason,omitempty"`
	TotalLines    int           `json:"totalLines"`
	Commits       []BlameCommit `json:"commits"`
	Lines         []BlameLine   `json:"lines"`
}

// Blame computes read-only per-line provenance via `ReadGit` (`git blame --porcelain
// --no-textconv --no-progress <commitOid> -- <path>`), degrading gracefully for
// binary, oversized, symlink, or submodule entries.
func (s *Service) Blame(ctx context.Context, q BlameQuery) (BlameResult, error) {
	cleanPath, err := validateCleanHistoryPath(q.Path)
	if err != nil {
		return BlameResult{}, err
	}
	if cleanPath == "" {
		return BlameResult{}, problemf(ErrInvalid, "文件路径不能为空")
	}

	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	cwd, commitOID, status, err := s.resolveFileReadTarget(ctx, q.RepositoryID, q.WorktreeID, q.Rev)
	if err != nil {
		return BlameResult{}, err
	}
	if status != HistoryStatusReady {
		return BlameResult{}, problemf(ErrNotFound, "当前仓库尚无已提交历史，无法执行 Blame 溯源")
	}

	treeData, err := ReadGit(ctx, cwd, 4096, "ls-tree", "-z", commitOID, "--", cleanPath)
	if err != nil {
		return BlameResult{}, err
	}
	var mode, objType, blobOID string
	for _, rec := range bytes.Split(bytes.TrimSuffix(treeData, []byte{0}), []byte{0}) {
		header, recPath, ok := strings.Cut(string(rec), "\t")
		if !ok || recPath != cleanPath {
			continue
		}
		fields := strings.Fields(header)
		if len(fields) == 3 {
			mode, objType, blobOID = fields[0], fields[1], fields[2]
			break
		}
	}
	if blobOID == "" {
		return BlameResult{}, problemf(ErrNotFound, "版本 %s 中不存在文件 %s", shortOID(commitOID), cleanPath)
	}
	if objType == "tree" {
		return BlameResult{}, problemf(ErrInvalid, "目标是目录，不支持逐行 Blame")
	}

	base := BlameResult{
		CommitOID: commitOID,
		ShortOID:  shortOID(commitOID),
		Path:      cleanPath,
		BlobOID:   blobOID,
		Mode:      mode,
		Commits:   []BlameCommit{},
		Lines:     []BlameLine{},
	}

	if mode == "160000" || mode == "120000" {
		base.Kind = "unsupported"
		base.DegradeReason = "子模块指针或符号链接不支持逐行文本 Blame"
		return base, nil
	}

	switch s.classifyBlobKind(ctx, cwd, blobOID) {
	case ConflictKindTooLarge:
		base.Kind = "too_large"
		base.DegradeReason = "文件体积或行数超出在线 Blame 浏览上限（2 MiB / 20000 行）"
		return base, nil
	case ConflictKindBinary:
		base.Kind = "binary"
		base.DegradeReason = "二进制或非 UTF-8 文件不支持逐行文本 Blame"
		return base, nil
	}

	sizeRaw, sErr := ReadGit(ctx, cwd, 128, "cat-file", "-s", blobOID)
	if sErr == nil && strings.TrimSpace(string(sizeRaw)) == "0" {
		base.Kind = "text"
		return base, nil
	}

	blameRaw, err := ReadGit(ctx, cwd, maxHistoryOutput,
		"blame", "--porcelain", "--no-textconv", "--no-progress",
		commitOID, "--", cleanPath,
	)
	if err != nil {
		return BlameResult{}, err
	}

	commits, lines := parsePorcelainBlame(blameRaw)
	base.Kind = "text"
	base.TotalLines = len(lines)
	base.Commits = commits
	base.Lines = lines
	return base, nil
}

func parsePorcelainBlame(raw []byte) ([]BlameCommit, []BlameLine) {
	commitsByOID := make(map[string]*BlameCommit)
	commitOrder := make([]string, 0)
	lines := make([]BlameLine, 0)

	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64<<10), maxHunkFileBytes+4096)

	var curOID string
	var curOrigLine, curFinalLine int
	var curOrigPath, curPrevOID, curPrevPath string
	var authorSec int64
	var authorTZ string

	flushCommitTime := func(c *BlameCommit) {
		if c == nil || c.AuthorAt != "" || authorSec == 0 {
			return
		}
		c.AuthorAt = formatGitUnixTZ(authorSec, authorTZ)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if curOID != "" && strings.HasPrefix(line, "\t") {
			c := commitsByOID[curOID]
			flushCommitTime(c)
			origPath := curOrigPath
			prevOID := curPrevOID
			prevPath := curPrevPath
			if c != nil {
				if prevOID == "" {
					prevOID = c.PreviousOID
				}
				if prevPath == "" {
					prevPath = c.PreviousPath
				}
			}
			lineNum := curFinalLine
			if lineNum <= 0 {
				lineNum = len(lines) + 1
			}
			lines = append(lines, BlameLine{
				LineNumber:   lineNum,
				OrigLine:     curOrigLine,
				CommitOID:    curOID,
				ShortOID:     shortOID(curOID),
				OrigPath:     origPath,
				PreviousOID:  prevOID,
				PreviousPath: prevPath,
				Text:         line[1:],
			})
			curOID = ""
			continue
		}

		if curOID == "" {
			fields := strings.Fields(line)
			if len(fields) >= 3 && isObjectID(fields[0]) {
				origL, err1 := strconv.Atoi(fields[1])
				finalL, err2 := strconv.Atoi(fields[2])
				if err1 == nil && err2 == nil {
					curOID = fields[0]
					curOrigLine = origL
					curFinalLine = finalL
					curOrigPath = ""
					curPrevOID = ""
					curPrevPath = ""
					authorSec = 0
					authorTZ = ""
					if _, exists := commitsByOID[curOID]; !exists {
						commitsByOID[curOID] = &BlameCommit{
							OID:      curOID,
							ShortOID: shortOID(curOID),
						}
						commitOrder = append(commitOrder, curOID)
					}
				}
			}
			continue
		}

		c := commitsByOID[curOID]
		switch {
		case strings.HasPrefix(line, "author "):
			c.AuthorName = strings.TrimPrefix(line, "author ")
		case strings.HasPrefix(line, "author-mail "):
			mail := strings.TrimSpace(strings.TrimPrefix(line, "author-mail "))
			mail = strings.TrimPrefix(strings.TrimSuffix(mail, ">"), "<")
			c.AuthorEmail = mail
		case strings.HasPrefix(line, "author-time "):
			authorSec, _ = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "author-time ")), 10, 64)
		case strings.HasPrefix(line, "author-tz "):
			authorTZ = strings.TrimSpace(strings.TrimPrefix(line, "author-tz "))
		case strings.HasPrefix(line, "summary "):
			c.Summary = strings.TrimPrefix(line, "summary ")
		case line == "boundary":
			c.Boundary = true
		case strings.HasPrefix(line, "previous "):
			rest := strings.TrimPrefix(line, "previous ")
			pOID, pPath, ok := strings.Cut(rest, " ")
			if ok && isObjectID(pOID) {
				unquoted := unquoteGitPath(pPath)
				curPrevOID = pOID
				curPrevPath = unquoted
				if c.PreviousOID == "" {
					c.PreviousOID = pOID
					c.PreviousPath = unquoted
				}
			}
		case strings.HasPrefix(line, "filename "):
			curOrigPath = unquoteGitPath(strings.TrimPrefix(line, "filename "))
		}
	}

	commits := make([]BlameCommit, 0, len(commitOrder))
	for _, oid := range commitOrder {
		if c := commitsByOID[oid]; c != nil {
			commits = append(commits, *c)
		}
	}
	return commits, lines
}

func unquoteGitPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) >= 2 && strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") {
		if u, err := strconv.Unquote(trimmed); err == nil {
			return u
		}
	}
	return trimmed
}

func formatGitUnixTZ(sec int64, tz string) string {
	if len(tz) == 5 && (tz[0] == '+' || tz[0] == '-') {
		hh, err1 := strconv.Atoi(tz[1:3])
		mm, err2 := strconv.Atoi(tz[3:5])
		if err1 == nil && err2 == nil {
			offset := (hh*60 + mm) * 60
			if tz[0] == '-' {
				offset = -offset
			}
			return time.Unix(sec, 0).In(time.FixedZone("", offset)).Format(time.RFC3339)
		}
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}
