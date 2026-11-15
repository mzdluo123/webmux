package workspace

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

const (
	DefaultFileHistoryLimit = 50
	MaxFileHistoryLimit     = 200
)

// FileHistoryQuery specifies parameters for single-file commit history (`git log --follow`).
type FileHistoryQuery struct {
	RepositoryID string
	WorktreeID   string
	StartOID     string // optional full hex OID or "HEAD" (default)
	Path         string
	Limit        int
	Cursor       string
}

// FileHistoryRow represents one commit touching the tracked file along its rename chain.
type FileHistoryRow struct {
	OID               string   `json:"oid"`
	ShortOID          string   `json:"shortOid"`
	Parents           []string `json:"parents"`
	ComparedParentOID string   `json:"comparedParentOid,omitempty"`
	Subject           string   `json:"subject"`
	AuthorName        string   `json:"authorName"`
	AuthorEmail       string   `json:"authorEmail"`
	AuthorAt          string   `json:"authorAt"`
	Status            string   `json:"status"` // M, A, D, R, C, T
	Path              string   `json:"path"`
	OldPath           string   `json:"oldPath,omitempty"`
}

// FileHistoryPage holds a paginated slice of single-file history commits.
type FileHistoryPage struct {
	Status     string           `json:"status"` // "ready" | "unborn" | "empty"
	Path       string           `json:"path"`
	StartOID   string           `json:"startOid,omitempty"`
	Commits    []FileHistoryRow `json:"commits"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

// FileHistory walks single-file history with `--follow` across renames, returning
// the exact file path and previous path at each commit so callers can inspect diffs accurately.
func (s *Service) FileHistory(ctx context.Context, q FileHistoryQuery) (FileHistoryPage, error) {
	cleanPath, err := validateCleanHistoryPath(q.Path)
	if err != nil {
		return FileHistoryPage{}, err
	}
	if cleanPath == "" {
		return FileHistoryPage{}, problemf(ErrInvalid, "文件路径不能为空")
	}

	limit := q.Limit
	if limit <= 0 {
		limit = DefaultFileHistoryLimit
	} else if limit > MaxFileHistoryLimit {
		limit = MaxFileHistoryLimit
	}

	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	cwd, startOID, status, err := s.resolveFileReadTarget(ctx, q.RepositoryID, q.WorktreeID, q.StartOID)
	if err != nil {
		return FileHistoryPage{}, err
	}
	if status != HistoryStatusReady {
		return FileHistoryPage{
			Status:  status,
			Path:    cleanPath,
			Commits: []FileHistoryRow{},
		}, nil
	}

	walkOID := startOID
	walkPath := cleanPath
	digest := hashOperationRequest("file_history", startOID, cleanPath)[:16]
	if q.Cursor != "" {
		parts := strings.SplitN(q.Cursor, ":", 3)
		if len(parts) != 3 || parts[0] != digest || !isObjectID(parts[1]) {
			return FileHistoryPage{}, problemf(ErrInvalid, "无效的文件历史分页游标")
		}
		decodedPath, dErr := base64.RawURLEncoding.DecodeString(parts[2])
		if dErr != nil {
			return FileHistoryPage{}, problemf(ErrInvalid, "无效的文件历史分页游标")
		}
		curPath, pErr := validateCleanHistoryPath(string(decodedPath))
		if pErr != nil || curPath == "" {
			return FileHistoryPage{}, problemf(ErrInvalid, "无效的文件历史分页游标")
		}
		walkOID = parts[1]
		walkPath = curPath
	}

	args := []string{
		"log", "--follow", "--name-status", "-z",
		"--no-show-signature", "--no-ext-diff", "--no-textconv", "--find-renames",
		"--format=%x1e%H%x00%P%x00%s%x00%an%x00%ae%x00%aI%x00",
		"-n", strconv.Itoa(limit + 1),
		walkOID, "--", walkPath,
	}
	data, err := ReadGit(ctx, cwd, maxHistoryOutput, args...)
	if err != nil {
		return FileHistoryPage{}, err
	}

	rows := parseFileHistoryOutput(data, walkPath)
	nextCursor := ""
	if len(rows) > limit {
		nextRow := rows[limit]
		rows = rows[:limit]
		nextCursor = fmt.Sprintf("%s:%s:%s", digest, nextRow.OID, base64.RawURLEncoding.EncodeToString([]byte(nextRow.Path)))
	}

	return FileHistoryPage{
		Status:     HistoryStatusReady,
		Path:       cleanPath,
		StartOID:   startOID,
		Commits:    rows,
		NextCursor: nextCursor,
	}, nil
}

func parseFileHistoryOutput(data []byte, fallbackPath string) []FileHistoryRow {
	rows := make([]FileHistoryRow, 0)
	if len(data) == 0 {
		return rows
	}
	for _, chunk := range bytes.Split(data, []byte{0x1e}) {
		if len(chunk) == 0 {
			continue
		}
		parts := bytes.Split(chunk, []byte{0})
		if len(parts) < 6 {
			continue
		}
		oid := strings.TrimSpace(string(parts[0]))
		if !isObjectID(oid) {
			continue
		}
		parents := strings.Fields(strings.TrimSpace(string(parts[1])))
		if parents == nil {
			parents = []string{}
		}
		var comparedParent string
		if len(parents) > 0 {
			comparedParent = parents[0]
		}
		row := FileHistoryRow{
			OID:               oid,
			ShortOID:          shortOID(oid),
			Parents:           parents,
			ComparedParentOID: comparedParent,
			Subject:           string(parts[2]),
			AuthorName:        string(parts[3]),
			AuthorEmail:       string(parts[4]),
			AuthorAt:          strings.TrimSpace(string(parts[5])),
			Status:            "M",
			Path:              fallbackPath,
		}

		idx := 6
		for idx < len(parts) && strings.TrimSpace(string(parts[idx])) == "" {
			idx++
		}
		if idx < len(parts) {
			rawStatus := strings.TrimSpace(string(parts[idx]))
			idx++
			if rawStatus != "" {
				statusCode := rawStatus[:1]
				row.Status = statusCode
				if (statusCode == "R" || statusCode == "C") && idx+1 < len(parts) {
					oldP := string(parts[idx])
					newP := string(parts[idx+1])
					if newP != "" {
						row.Path = newP
					}
					if oldP != "" && oldP != row.Path {
						row.OldPath = oldP
					}
				} else if idx < len(parts) && string(parts[idx]) != "" {
					row.Path = string(parts[idx])
				}
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func (s *Service) resolveFileReadTarget(ctx context.Context, repositoryID, worktreeID, rawRev string) (cwd, commitOID, status string, err error) {
	rev := strings.TrimSpace(rawRev)
	if rev != "" && rev != "HEAD" && !isObjectID(rev) {
		return "", "", "", problemf(ErrInvalid, "版本参数必须是 HEAD 或完整的十六进制提交 OID")
	}

	if worktreeID != "" {
		loc, lErr := s.LocateWorktree(ctx, repositoryID, worktreeID)
		if lErr != nil {
			return "", "", "", lErr
		}
		if !loc.Available || loc.Path == "" {
			return "", "", "", problemf(ErrUnavailable, "%s", firstNonEmpty(loc.Reason, "工作树当前不可用"))
		}
		cwd = loc.Path
		if rev == "" || rev == "HEAD" {
			headOID, _ := s.resolveCommitReadOnly(ctx, cwd, "HEAD")
			if headOID == "" {
				return cwd, "", HistoryStatusUnborn, nil
			}
			return cwd, headOID, HistoryStatusReady, nil
		}
	} else {
		r, rErr := s.repository(repositoryID)
		if rErr != nil {
			return "", "", "", rErr
		}
		if err := acquire(ctx, r); err != nil {
			return "", "", "", err
		}
		err = s.verifyIdentity(ctx, r)
		cwd = r.entry.AnchorPath
		release(r)
		if err != nil {
			return "", "", "", err
		}
		if rev == "" || rev == "HEAD" {
			headOID, _ := s.resolveCommitReadOnly(ctx, cwd, "HEAD")
			if headOID == "" {
				data, _ := ReadGit(ctx, cwd, 4096, "for-each-ref", "--count=1", "--sort=-committerdate", "--format=%(objectname)", "refs/heads/", "refs/remotes/")
				headOID = strings.TrimSpace(string(data))
			}
			if !isObjectID(headOID) {
				return cwd, "", HistoryStatusEmpty, nil
			}
			return cwd, headOID, HistoryStatusReady, nil
		}
	}

	verified, vErr := ReadGit(ctx, cwd, 256, "rev-parse", "--verify", "--end-of-options", rev+"^{commit}")
	if vErr != nil || strings.TrimSpace(string(verified)) != rev {
		return "", "", "", problemf(ErrNotFound, "指定的提交对象不存在")
	}
	return cwd, rev, HistoryStatusReady, nil
}
