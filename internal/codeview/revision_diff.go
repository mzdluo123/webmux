package codeview

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"
)

// GitRevisionDiff captures a frozen two-sided diff snapshot comparing a file at a
// verified full hex commit (`baseOID`) against the live working tree (`worktree`).
func (s *Service) GitRevisionDiff(ctx context.Context, repositoryID, worktreeID, cid, baseOID, path string) (Diff, error) {
	if worktreeID == "" {
		return Diff{}, problemf(ErrReadOnly, "与当前工作区文件比较需要选择具体工作树")
	}
	t, err := s.resolveGitContext(ctx, repositoryID, worktreeID, cid)
	if err != nil {
		return Diff{}, err
	}
	if t.WorktreeID == "" {
		return Diff{}, problemf(ErrReadOnly, "当前浏览上下文未绑定工作树，无法读取工作区文件")
	}
	cleanPath, err := nodePath(nodeID(path))
	if err != nil || cleanPath == "" {
		return Diff{}, problemf(ErrInvalid, "无效比较文件路径")
	}
	return withTargetContext(ctx, s, t, cid, func(key scopeKey, b *binding) (Diff, error) {
		if b.Root == "" {
			return Diff{}, problemf(ErrReadOnly, "当前工作树目录不可浏览")
		}
		if err := b.verifyCommitOID(ctx, baseOID); err != nil {
			return Diff{}, err
		}

		zeroOID := strings.Repeat("0", len(baseOID))
		oldMode := "000000"
		oldOID := zeroOID
		rawTree, err := git(ctx, b.gitCwd(), "ls-tree", "-z", baseOID, "--", cleanPath)
		if err != nil {
			return Diff{}, err
		}
		for _, rec := range strings.Split(string(rawTree), "\x00") {
			header, recPath, ok := strings.Cut(rec, "\t")
			if !ok || recPath != cleanPath {
				continue
			}
			fields := strings.Fields(header)
			if len(fields) == 3 {
				if fields[1] == "tree" {
					return Diff{}, problemf(ErrInvalid, "目标是目录，不能作为文件比较")
				}
				oldMode, oldOID = fields[0], fields[2]
				break
			}
		}

		oldSource := "empty"
		if oldMode != "000000" {
			oldSource = commitSourcePrefix + baseOID
		}
		oldDoc, err := b.blob(ctx, cleanPath, oldSource, oldOID, oldMode)
		if err != nil {
			return Diff{}, err
		}

		root, err := os.OpenRoot(b.Root)
		if err != nil {
			return Diff{}, err
		}
		infoBefore, statErr := root.Lstat(cleanPath)
		_ = root.Close()

		newMode := "000000"
		newOID := zeroOID
		var newDoc Document

		switch {
		case errors.Is(statErr, os.ErrNotExist):
			if oldMode == "000000" {
				return Diff{}, problemf(ErrNotFound, "文件 %s 在版本 %s 与当前工作区中均不存在", cleanPath, baseOID[:8])
			}
			newDoc = document(b.Root, cleanPath, "empty", "", nil)
		case statErr != nil:
			return Diff{}, statErr
		case infoBefore.IsDir():
			return Diff{}, problemf(ErrInvalid, "目标是目录，不能作为文件比较")
		case infoBefore.Mode()&os.ModeSymlink != 0:
			newMode = "120000"
			newDoc = document(b.Root, cleanPath, "worktree", "", nil)
			newDoc.Kind = "unsupported"
			newDoc.Reason = "符号链接或特殊模式仅显示元信息"
		case !infoBefore.Mode().IsRegular():
			newMode = "100644"
			newDoc = document(b.Root, cleanPath, "worktree", "", nil)
			newDoc.Kind = "unsupported"
			newDoc.Reason = "特殊文件仅显示元信息"
		default:
			if infoBefore.Mode().Perm()&0o111 != 0 {
				newMode = "100755"
			} else {
				newMode = "100644"
			}
			newOID = b.diskBlobOID(ctx, cleanPath)
			newDoc, err = b.read(cleanPath)
			if err != nil {
				return Diff{}, err
			}
			afterOID := b.diskBlobOID(ctx, cleanPath)
			if newOID != "" && afterOID != "" && newOID != afterOID {
				return Diff{}, problemf(ErrConflict, "读取期间工作区文件已改变，请刷新重试")
			}
		}

		status := "M"
		switch {
		case oldMode == "000000":
			status = "A"
		case newMode == "000000":
			status = "D"
		case oldMode != newMode && (oldMode == "120000" || newMode == "120000"):
			status = "T"
		}

		changeID := version([]string{"revision", baseOID, cleanPath, status, oldOID, newOID, oldMode, newMode})
		found := Change{
			ID:            changeID,
			ReviewVersion: changeID,
			NodeID:        nodeID(cleanPath),
			Group:         "revision",
			OldPath:       cleanPath,
			NewPath:       cleanPath,
			Status:        status,
			OldMode:       oldMode,
			NewMode:       newMode,
			OldOID:        oldOID,
			NewOID:        newOID,
		}

		diff := Diff{
			Old:     oldDoc,
			New:     newDoc,
			Kind:    "text",
			Checked: time.Now().UTC(),
			Change:  found,
		}
		if kind, reason := classify(oldDoc, newDoc); kind != "text" {
			diff.Kind = kind
			diff.Reason = reason
		} else {
			attrs, attrErr := git(ctx, b.Root, "check-attr", "-z", "filter", "working-tree-encoding", "--", cleanPath)
			if attrErr != nil {
				return Diff{}, attrErr
			}
			transformed := hasTransformAttr(attrs)
			if !transformed && oldMode != "000000" {
				transformed, attrErr = b.hasCommitTransformAttr(ctx, baseOID, cleanPath)
				if attrErr != nil {
					return Diff{}, attrErr
				}
			}
			if transformed {
				diff.Kind = "metadata"
				diff.Reason = "需要内容转换，不能保证原始行映射"
			}
		}
		snap := &snapshot{owner: ownerScope(key), context: cid, diff: &diff, size: documentSize(oldDoc) + documentSize(newDoc)}
		diff.Snapshot = s.put(snap)
		return diff, nil
	})
}
