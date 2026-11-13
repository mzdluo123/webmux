package codeview

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/workspace"
)

func diskGitMode(root *os.Root, relPath string) string {
	if root == nil {
		return "100644"
	}
	info, err := root.Lstat(relPath)
	if err != nil {
		return "100644"
	}
	mode := info.Mode()
	switch {
	case mode&os.ModeSymlink != 0:
		return "120000"
	case mode.IsRegular() && mode.Perm()&0o111 != 0:
		return "100755"
	default:
		return "100644"
	}
}

func (b *binding) diskBlobOID(ctx context.Context, relPath string) string {
	out, err := git(ctx, b.Root, "hash-object", "--no-filters", "--", relPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// scanTaskNetChanges computes the single net diff from baseline (empty tree
// when Unborn, or BaseOID) directly to b.Root's live working tree without
// touching .git/index or double-counting committed + uncommitted edits.
func (b *binding) scanTaskNetChanges(ctx context.Context, baseline workspace.TaskBaseline, visit func(Change) error) (string, error) {
	if !b.Diff || b.Root == "" {
		return "", problemf(ErrReadOnly, "此目录的 Git 任务范围差异不可用")
	}
	diffRef, canonicalFrom, _, err := b.resolveFromTreeRef(ctx, baseline.BaseOID)
	if err != nil {
		return "", err
	}
	root, err := os.OpenRoot(b.Root)
	if err != nil {
		return "", fmt.Errorf("open worktree root for task review: %w", err)
	}
	defer func() { _ = root.Close() }()

	var raw []Change
	deletedIdxByPath := map[string]int{}
	args := []string{"diff", "--raw", "-z", "--no-abbrev", "--no-ext-diff", "--no-textconv", "--find-renames", diffRef, "--"}
	phase := 0
	var cur Change
	err = workspace.ReadGitRecords(ctx, b.Root, func(record string) error {
		switch phase {
		case 0:
			fields := strings.Fields(record)
			if len(fields) != 5 || !strings.HasPrefix(fields[0], ":") {
				return problemf(ErrInternal, "无效 Git 任务差异输出")
			}
			cur = Change{
				Group:   "task",
				OldMode: strings.TrimPrefix(fields[0], ":"),
				NewMode: fields[1],
				OldOID:  fields[2],
				NewOID:  fields[3],
				Status:  fields[4],
			}
			phase = 1
		case 1:
			if record == "" {
				return problemf(ErrInternal, "无效 Git 任务差异路径")
			}
			cur.OldPath, cur.NewPath = record, record
			if strings.HasPrefix(cur.Status, "R") || strings.HasPrefix(cur.Status, "C") {
				phase = 2
			} else {
				phase = 0
				if cur.Status == "D" {
					deletedIdxByPath[cur.OldPath] = len(raw)
				}
				raw = append(raw, cur)
			}
		case 2:
			if record == "" {
				return problemf(ErrInternal, "无效 Git 任务重命名路径")
			}
			cur.NewPath = record
			phase = 0
			raw = append(raw, cur)
		}
		return nil
	}, args...)
	if err != nil {
		return "", err
	}
	if phase != 0 {
		return "", problemf(ErrInternal, "Git 任务差异输出不完整")
	}

	dropIndices := map[int]bool{}
	err = workspace.ReadGitRecords(ctx, b.Root, func(untrackedPath string) error {
		if untrackedPath == "" {
			return problemf(ErrInternal, "无效 Git 未跟踪路径")
		}
		mode := diskGitMode(root, untrackedPath)
		if idx, wasDeleted := deletedIdxByPath[untrackedPath]; wasDeleted {
			// File existed in baseline, was deleted from index/HEAD, and then
			// recreated on disk as an untracked file: collapse into a single net
			// change (or omit if identical to baseline).
			prev := raw[idx]
			var blobOID string
			if mode == "100644" || mode == "100755" {
				blobOID = b.diskBlobOID(ctx, untrackedPath)
			}
			if blobOID != "" && blobOID == prev.OldOID && mode == prev.OldMode {
				dropIndices[idx] = true
				return nil
			}
			status := "M"
			if (prev.OldMode == "120000") != (mode == "120000") {
				status = "T"
			}
			raw[idx] = Change{
				Group:   "task",
				OldPath: prev.OldPath,
				NewPath: untrackedPath,
				OldMode: prev.OldMode,
				NewMode: mode,
				OldOID:  prev.OldOID,
				NewOID:  blobOID,
				Status:  status,
			}
			return nil
		}
		raw = append(raw, Change{
			Group:   "task",
			OldPath: untrackedPath,
			NewPath: untrackedPath,
			OldMode: "000000",
			NewMode: mode,
			Status:  "A",
		})
		return nil
	}, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", err
	}

	merged := make([]Change, 0, len(raw)-len(dropIndices))
	for i, item := range raw {
		if dropIndices[i] {
			continue
		}
		merged = append(merged, item)
	}
	slices.SortFunc(merged, func(a, b Change) int {
		if c := cmp.Compare(a.NewPath, b.NewPath); c != 0 {
			return c
		}
		return cmp.Compare(a.OldPath, b.OldPath)
	})

	hash := sha256.New()
	for _, item := range merged {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		item.NodeID = nodeID(item.NewPath)
		item.ID = version([]string{
			"task",
			canonicalFrom,
			item.OldPath,
			item.NewPath,
			item.Status,
			item.OldOID,
			item.NewOID,
			item.OldMode,
			item.NewMode,
		})
		item.ReviewVersion = reviewVersion(root, item)
		_, _ = io.WriteString(hash, item.ID+"\x00"+item.ReviewVersion+"\x00")
		if visit != nil {
			if err := visit(item); err != nil {
				return "", err
			}
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// GitTaskChanges returns the single net change listing from worktreeID's fixed
// TaskBaseline to its live working tree alongside commitCount and uncommittedCount.
func (s *Service) GitTaskChanges(ctx context.Context, repositoryID, worktreeID string, q ChangeQuery) (TaskChanges, error) {
	if repositoryID == "" || worktreeID == "" {
		return TaskChanges{}, problemf(ErrInvalid, "请选择已登记的仓库与工作树")
	}
	resolver, err := s.resolver()
	if err != nil {
		return TaskChanges{}, err
	}
	summary, err := resolver.ResolveTaskScope(ctx, repositoryID, worktreeID)
	if err != nil {
		return TaskChanges{}, problemf(errorKind(err), "%s", err.Error())
	}
	out := TaskChanges{
		Status:           summary.Status,
		Reason:           summary.Reason,
		Baseline:         summary.Baseline,
		HeadOID:          summary.HeadOID,
		HeadUnborn:       summary.HeadUnborn,
		CommitCount:      summary.CommitCount,
		UncommittedCount: summary.UncommittedCount,
		Changes:          Changes{Entries: []Change{}, Checked: time.Now().UTC()},
	}
	if summary.Baseline == nil {
		return out, nil
	}
	t := Target{RepositoryID: repositoryID, WorktreeID: worktreeID}
	changes, err := withScope(ctx, s, targetScope(t), func(b *binding) (Changes, error) {
		r := Changes{Entries: []Change{}, Checked: time.Now().UTC()}
		var filterPath string
		if q.NodeID != "" {
			var pErr error
			filterPath, pErr = nodePath(q.NodeID)
			if pErr != nil {
				return r, pErr
			}
		}
		start, tag := 0, ""
		if q.Cursor != "" {
			var offset string
			var ok bool
			var cErr error
			tag, offset, ok = strings.Cut(q.Cursor, ":")
			start, cErr = strconv.Atoi(offset)
			if !ok || tag == "" || cErr != nil || start < 0 {
				return r, problemf(ErrInvalid, "无效修改游标")
			}
		}
		search := strings.ToLower(q.Q)
		ver, sErr := b.scanTaskNetChanges(ctx, *summary.Baseline, func(c Change) error {
			if q.NodeID != "" && c.NewPath != filterPath && c.OldPath != filterPath {
				return nil
			}
			if search != "" && !strings.Contains(strings.ToLower(displayPath(c.NewPath)), search) && !strings.Contains(strings.ToLower(displayPath(c.OldPath)), search) {
				return nil
			}
			if r.Total >= start && len(r.Entries) < ChangePageSize {
				r.Entries = append(r.Entries, c)
			}
			r.Total++
			return nil
		})
		if sErr != nil {
			return r, sErr
		}
		r.Version = ver
		expected := version([]string{b.ID, summary.Baseline.BaselineID, r.Version, q.Q, q.NodeID})
		if q.Cursor != "" && tag != expected {
			return r, problemf(ErrConflict, "修改列表已改变，请刷新任务范围列表")
		}
		if start > r.Total {
			return r, problemf(ErrInvalid, "无效修改页")
		}
		end := start + len(r.Entries)
		if end < r.Total {
			r.Next = fmt.Sprintf("%s:%d", expected, end)
		}
		return r, nil
	})
	if err != nil {
		return TaskChanges{}, err
	}
	out.Changes = changes
	return out, nil
}

// GitTaskDiff captures a frozen two-sided net diff snapshot from worktreeID's
// fixed TaskBaseline directly to its live working tree for path or changeID.
func (s *Service) GitTaskDiff(ctx context.Context, repositoryID, worktreeID, path, changeID string) (Diff, error) {
	if repositoryID == "" || worktreeID == "" {
		return Diff{}, problemf(ErrInvalid, "请选择已登记的仓库与工作树")
	}
	var cleanPath string
	if path != "" {
		var err error
		cleanPath, err = nodePath(nodeID(path))
		if err != nil || cleanPath == "" {
			return Diff{}, problemf(ErrInvalid, "无效文件路径")
		}
	}
	if cleanPath == "" && changeID == "" {
		return Diff{}, problemf(ErrInvalid, "请指定要查看净差异的文件路径")
	}
	resolver, err := s.resolver()
	if err != nil {
		return Diff{}, err
	}
	summary, err := resolver.ResolveTaskScope(ctx, repositoryID, worktreeID)
	if err != nil {
		return Diff{}, problemf(errorKind(err), "%s", err.Error())
	}
	if summary.Baseline == nil {
		return Diff{}, problemf(ErrNoTaskBaseline, "%s", summary.Reason)
	}
	baseline := *summary.Baseline
	t := Target{RepositoryID: repositoryID, WorktreeID: worktreeID}
	key := targetScope(t)
	return withScope(ctx, s, key, func(b *binding) (Diff, error) {
		_, canonicalFrom, isEmpty, rErr := b.resolveFromTreeRef(ctx, baseline.BaseOID)
		if rErr != nil {
			return Diff{}, rErr
		}
		var found *Change
		var fallback *Change
		before, sErr := b.scanTaskNetChanges(ctx, baseline, func(entry Change) error {
			if changeID != "" {
				if entry.ID == changeID {
					copyEntry := entry
					found = &copyEntry
				}
				return nil
			}
			if entry.NewPath == cleanPath {
				copyEntry := entry
				found = &copyEntry
			} else if fallback == nil && entry.OldPath == cleanPath {
				copyEntry := entry
				fallback = &copyEntry
			}
			return nil
		})
		if sErr != nil {
			return Diff{}, sErr
		}
		if found == nil {
			found = fallback
		}
		if found == nil {
			return Diff{}, problemf(ErrNotFound, "指定的任务范围变更文件不存在，请刷新")
		}
		if cleanPath != "" && found.NewPath != cleanPath && found.OldPath != cleanPath {
			return Diff{}, problemf(ErrInvalid, "变更标识与指定路径不匹配")
		}
		if _, pErr := nodePath(nodeID(found.OldPath)); pErr != nil {
			return Diff{}, pErr
		}
		if _, pErr := nodePath(nodeID(found.NewPath)); pErr != nil {
			return Diff{}, pErr
		}
		oldSource := "empty"
		if !isEmpty && found.OldMode != "000000" {
			oldSource = commitSourcePrefix + canonicalFrom
		}
		oldDoc, oErr := b.blob(ctx, found.OldPath, oldSource, found.OldOID, found.OldMode)
		if oErr != nil {
			return Diff{}, oErr
		}
		var newDoc Document
		switch {
		case found.NewMode == "000000":
			newDoc = document(b.Root, found.NewPath, "empty", "", nil)
		case found.NewMode != "100644" && found.NewMode != "100755":
			newDoc = document(b.Root, found.NewPath, "worktree", "", nil)
			newDoc.Kind = "unsupported"
			newDoc.Reason = "子模块、链接或特殊模式仅显示元信息"
		default:
			var nErr error
			newDoc, nErr = b.read(found.NewPath)
			if nErr != nil {
				return Diff{}, nErr
			}
		}
		after, aErr := b.scanTaskNetChanges(ctx, baseline, nil)
		if aErr != nil {
			return Diff{}, aErr
		}
		if after != before {
			return Diff{}, problemf(ErrConflict, "读取期间工作树内容已改变，请刷新")
		}
		diff := Diff{
			Old:     oldDoc,
			New:     newDoc,
			Kind:    "text",
			Checked: time.Now().UTC(),
			Change:  *found,
		}
		if kind, reason := classify(oldDoc, newDoc); kind != "text" {
			diff.Kind = kind
			diff.Reason = reason
		} else {
			attrs, attrErr := git(ctx, b.Root, "check-attr", "-z", "filter", "working-tree-encoding", "--", found.NewPath)
			if attrErr != nil {
				return Diff{}, attrErr
			}
			transformed := hasTransformAttr(attrs)
			if !transformed && !isEmpty && found.OldPath != found.NewPath {
				transformed, attrErr = b.hasCommitTransformAttr(ctx, canonicalFrom, found.OldPath)
				if attrErr != nil {
					return Diff{}, attrErr
				}
			}
			if transformed {
				diff.Kind = "metadata"
				diff.Reason = "需要内容转换，不能保证原始行映射"
			}
		}
		snap := &snapshot{owner: ownerScope(key), context: b.ID, diff: &diff, size: documentSize(oldDoc) + documentSize(newDoc)}
		diff.Snapshot = s.put(snap)
		return diff, nil
	})
}
