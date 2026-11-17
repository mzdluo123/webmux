package workspace

import (
	"bytes"
	"cmp"
	"context"
	"slices"
	"strings"
	"time"
)

// ReviewFileState reports the live server-observed content identity and reviewed
// status of one changed file in either "uncommitted" or "task" scope.
type ReviewFileState struct {
	Scope          string     `json:"scope"`
	Area           string     `json:"area"`
	Path           string     `json:"path"`
	OldPath        string     `json:"oldPath,omitempty"`
	Status         string     `json:"status"`
	BaseMode       string     `json:"baseMode,omitempty"`
	BaseIdentity   string     `json:"baseIdentity,omitempty"`
	TargetMode     string     `json:"targetMode,omitempty"`
	TargetIdentity string     `json:"targetIdentity,omitempty"`
	Reviewed       bool       `json:"reviewed"`
	ReviewedAt     *time.Time `json:"reviewedAt,omitempty"`
}

func normalizeGitMode(mode string) string {
	if mode == "000000" {
		return ""
	}
	return mode
}

func normalizeGitOID(oid string) string {
	if strings.Trim(oid, "0") == "" {
		return ""
	}
	return oid
}

func reviewIdentityMatches(a, b ReviewMarkIdentity) bool {
	return a.Scope == b.Scope &&
		a.Area == b.Area &&
		a.Path == b.Path &&
		a.OldPath == b.OldPath &&
		normalizeGitMode(a.BaseMode) == normalizeGitMode(b.BaseMode) &&
		normalizeGitOID(a.BaseIdentity) == normalizeGitOID(b.BaseIdentity) &&
		normalizeGitMode(a.TargetMode) == normalizeGitMode(b.TargetMode) &&
		normalizeGitOID(a.TargetIdentity) == normalizeGitOID(b.TargetIdentity)
}

func reviewSlotKey(area, path string) string {
	return area + "\x00" + path
}

func (s *Service) collectUncommittedReviewFiles(ctx context.Context, loc WorktreeLocation) ([]ReviewFileState, error) {
	headOID, _ := s.resolveCommitReadOnly(ctx, loc.Path, "HEAD")
	data, err := ReadGit(ctx, loc.Path, maxWorkspaceGitOutput, "status", "--porcelain=v2", "-z", "--untracked-files=normal")
	if err != nil {
		return nil, err
	}
	files := make([]ReviewFileState, 0)
	if len(data) == 0 {
		return files, nil
	}
	records := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	for i := 0; i < len(records); i++ {
		rec := string(records[i])
		if len(rec) < 3 {
			continue
		}
		switch rec[0] {
		case 'u':
			fields := strings.SplitN(rec, " ", 11)
			if len(fields) < 11 {
				continue
			}
			relPath := fields[10]
			wtMode, wtSum, _, _ := s.readWorktreeFile(ctx, loc.Path, relPath, "")
			files = append(files, ReviewFileState{
				Scope:          ReviewScopeUncommitted,
				Area:           ReviewAreaUnstaged,
				Path:           relPath,
				Status:         "U",
				BaseIdentity:   "conflicted",
				TargetMode:     normalizeGitMode(wtMode),
				TargetIdentity: wtSum,
			})
		case '1', '2':
			limit := 9
			if rec[0] == '2' {
				limit = 10
			}
			fields := strings.SplitN(rec, " ", limit)
			if len(fields) < limit {
				continue
			}
			xy := fields[1]
			relPath := fields[limit-1]
			oldPath := ""
			if rec[0] == '2' && i+1 < len(records) {
				i++
				oldPath = string(records[i])
			}
			idxEntry, idxErr := s.readIndexEntry(ctx, loc.Path, relPath)
			if idxErr != nil {
				return nil, idxErr
			}
			if len(xy) == 2 && xy[0] != '.' {
				lookupPath := relPath
				if oldPath != "" {
					lookupPath = oldPath
				}
				hMode, hOID, hErr := s.readHeadTreeEntry(ctx, loc.Path, headOID, lookupPath)
				if hErr != nil {
					return nil, hErr
				}
				files = append(files, ReviewFileState{
					Scope:          ReviewScopeUncommitted,
					Area:           ReviewAreaStaged,
					Path:           relPath,
					OldPath:        oldPath,
					Status:         string(xy[0]),
					BaseMode:       normalizeGitMode(hMode),
					BaseIdentity:   normalizeGitOID(hOID),
					TargetMode:     normalizeGitMode(idxEntry.mode),
					TargetIdentity: normalizeGitOID(idxEntry.oid),
				})
			}
			if len(xy) == 2 && xy[1] != '.' {
				wtMode, wtSum, _, wtErr := s.readWorktreeFile(ctx, loc.Path, relPath, idxEntry.mode)
				if wtErr != nil {
					return nil, wtErr
				}
				files = append(files, ReviewFileState{
					Scope:          ReviewScopeUncommitted,
					Area:           ReviewAreaUnstaged,
					Path:           relPath,
					Status:         string(xy[1]),
					BaseMode:       normalizeGitMode(idxEntry.mode),
					BaseIdentity:   normalizeGitOID(idxEntry.oid),
					TargetMode:     normalizeGitMode(wtMode),
					TargetIdentity: wtSum,
				})
			}
		case '?':
			relPath := rec[2:]
			wtMode, wtSum, _, wtErr := s.readWorktreeFile(ctx, loc.Path, relPath, "")
			if wtErr != nil {
				return nil, wtErr
			}
			files = append(files, ReviewFileState{
				Scope:          ReviewScopeUncommitted,
				Area:           ReviewAreaUnstaged,
				Path:           relPath,
				Status:         "?",
				TargetMode:     normalizeGitMode(wtMode),
				TargetIdentity: wtSum,
			})
		}
	}
	return files, nil
}

func (s *Service) collectTaskReviewFiles(ctx context.Context, repositoryID, worktreeID string, loc WorktreeLocation) ([]ReviewFileState, string, string, error) {
	if err := s.requireTaskBaselines(); err != nil {
		return nil, "", "", err
	}
	baseline := s.findTaskBaseline(repositoryID, worktreeID)
	if baseline == nil {
		return []ReviewFileState{}, TaskScopeNoTaskBaseline, "当前工作树尚未设定任务起点基线，可将当前 HEAD 或历史提交设为任务起点。", nil
	}
	diffRef := baseline.BaseOID
	if baseline.Unborn || diffRef == "" {
		emptyOID, err := s.emptyTreeOID(ctx, loc.Path)
		if err != nil {
			return nil, "", "", err
		}
		diffRef = emptyOID
	}

	type rawTaskItem struct {
		oldPath string
		newPath string
		oldMode string
		newMode string
		oldOID  string
		status  string
	}
	var raw []rawTaskItem
	deletedIdxByPath := make(map[string]int)
	args := []string{"diff", "--raw", "-z", "--no-abbrev", "--no-ext-diff", "--no-textconv", "--find-renames", diffRef, "--"}
	phase := 0
	var cur rawTaskItem
	err := ReadGitRecords(ctx, loc.Path, func(record string) error {
		switch phase {
		case 0:
			fields := strings.Fields(record)
			if len(fields) != 5 || !strings.HasPrefix(fields[0], ":") {
				return problemf(ErrInternal, "无效 Git 任务差异输出")
			}
			cur = rawTaskItem{
				oldMode: strings.TrimPrefix(fields[0], ":"),
				newMode: fields[1],
				oldOID:  fields[2],
				status:  fields[4],
			}
			phase = 1
		case 1:
			if record == "" {
				return problemf(ErrInternal, "无效 Git 任务差异路径")
			}
			cur.oldPath, cur.newPath = record, record
			if strings.HasPrefix(cur.status, "R") || strings.HasPrefix(cur.status, "C") {
				phase = 2
			} else {
				phase = 0
				if cur.status == "D" {
					deletedIdxByPath[cur.oldPath] = len(raw)
				}
				raw = append(raw, cur)
			}
		case 2:
			if record == "" {
				return problemf(ErrInternal, "无效 Git 任务重命名路径")
			}
			cur.newPath = record
			phase = 0
			raw = append(raw, cur)
		}
		return nil
	}, args...)
	if err != nil {
		return nil, "", "", err
	}

	dropIndices := make(map[int]bool)
	err = ReadGitRecords(ctx, loc.Path, func(untrackedPath string) error {
		if untrackedPath == "" {
			return nil
		}
		if idx, wasDeleted := deletedIdxByPath[untrackedPath]; wasDeleted {
			prev := raw[idx]
			wtMode, _, wtRaw, wtErr := s.readWorktreeFile(ctx, loc.Path, untrackedPath, prev.oldMode)
			if wtErr != nil {
				return wtErr
			}
			if isRegularGitMode(wtMode) && prev.oldMode == wtMode {
				out, _, hErr := s.gitInputOutput(ctx, loc.Path, bytes.NewReader(wtRaw), []string{"GIT_OPTIONAL_LOCKS=0"}, "hash-object", "--no-filters", "--stdin")
				if hErr == nil && outputLine(out) == prev.oldOID {
					dropIndices[idx] = true
					return nil
				}
			}
			status := "M"
			if (prev.oldMode == "120000") != (wtMode == "120000") {
				status = "T"
			}
			raw[idx] = rawTaskItem{
				oldPath: prev.oldPath,
				newPath: untrackedPath,
				oldMode: prev.oldMode,
				newMode: wtMode,
				oldOID:  prev.oldOID,
				status:  status,
			}
			return nil
		}
		raw = append(raw, rawTaskItem{
			oldPath: untrackedPath,
			newPath: untrackedPath,
			oldMode: "000000",
			newMode: "100644",
			status:  "A",
		})
		return nil
	}, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, "", "", err
	}

	merged := make([]rawTaskItem, 0, len(raw)-len(dropIndices))
	for i, item := range raw {
		if !dropIndices[i] {
			merged = append(merged, item)
		}
	}
	slices.SortFunc(merged, func(a, b rawTaskItem) int {
		if c := cmp.Compare(a.newPath, b.newPath); c != 0 {
			return c
		}
		return cmp.Compare(a.oldPath, b.oldPath)
	})

	files := make([]ReviewFileState, 0, len(merged))
	for _, item := range merged {
		oldPath := ""
		if item.oldPath != item.newPath {
			oldPath = item.oldPath
		}
		var wtMode, wtSum string
		if item.newMode != "000000" {
			var wtErr error
			wtMode, wtSum, _, wtErr = s.readWorktreeFile(ctx, loc.Path, item.newPath, item.newMode)
			if wtErr != nil {
				return nil, "", "", wtErr
			}
		}
		files = append(files, ReviewFileState{
			Scope:          ReviewScopeTask,
			Area:           ReviewAreaTask,
			Path:           item.newPath,
			OldPath:        oldPath,
			Status:         item.status,
			BaseMode:       normalizeGitMode(item.oldMode),
			BaseIdentity:   normalizeGitOID(item.oldOID),
			TargetMode:     normalizeGitMode(wtMode),
			TargetIdentity: wtSum,
		})
	}
	return files, TaskScopeReady, "", nil
}
