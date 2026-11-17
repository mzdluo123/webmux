package codeview

import (
	"context"
	"strings"
	"time"
)

// GitStashDiff captures a frozen two-sided diff snapshot for one file inside a
// repository stash entry (`stashOID`), supporting both tracked changes (`S^1 -> S`)
// and 3rd-parent untracked files (`empty -> S^3` when `--include-untracked` was used).
func (s *Service) GitStashDiff(ctx context.Context, repositoryID, worktreeID, cid, stashOID, path string, untracked bool) (Diff, error) {
	t, err := s.resolveGitContext(ctx, repositoryID, worktreeID, cid)
	if err != nil {
		return Diff{}, err
	}
	cleanPath, err := nodePath(nodeID(path))
	if err != nil || cleanPath == "" {
		return Diff{}, problemf(ErrInvalid, "无效贮藏文件路径")
	}
	return withTargetContext(ctx, s, t, cid, func(key scopeKey, b *binding) (Diff, error) {
		if err := b.verifyCommitOID(ctx, stashOID); err != nil {
			return Diff{}, err
		}
		cwd := b.gitCwd()
		if !isCommitInStashReflog(ctx, cwd, stashOID) {
			return Diff{}, problemf(ErrNotFound, "指定的贮藏条目不存在或已被移除")
		}

		parentRaw, err := git(ctx, cwd, "log", "-1", "--no-show-signature", "--format=%P", stashOID, "--")
		if err != nil {
			return Diff{}, err
		}
		parents := strings.Fields(strings.TrimSpace(string(parentRaw)))
		if len(parents) < 2 {
			return Diff{}, problemf(ErrInvalid, "提交对象不是有效的 Git 贮藏节点")
		}
		baseOID := parents[0]
		var untrackedOID string
		if len(parents) >= 3 {
			untrackedOID = parents[2]
		}

		if untracked {
			if untrackedOID == "" {
				return Diff{}, problemf(ErrNotFound, "该贮藏条目不包含未跟踪文件快照")
			}
			return s.buildUntrackedStashDiff(ctx, key, cid, b, stashOID, untrackedOID, cleanPath)
		}

		var found *Change
		var fallback *Change
		_, err = b.scanCommitChanges(ctx, baseOID, baseOID, stashOID, func(entry Change) error {
			if entry.NewPath == cleanPath {
				copyEntry := entry
				found = &copyEntry
			} else if fallback == nil && entry.OldPath == cleanPath {
				copyEntry := entry
				fallback = &copyEntry
			}
			return nil
		})
		if err != nil {
			return Diff{}, err
		}
		if found == nil {
			found = fallback
		}
		if found == nil && untrackedOID != "" {
			if diff, uErr := s.buildUntrackedStashDiff(ctx, key, cid, b, stashOID, untrackedOID, cleanPath); uErr == nil {
				return diff, nil
			}
		}
		if found == nil {
			return Diff{}, problemf(ErrNotFound, "贮藏条目中不存在文件 %s", cleanPath)
		}

		oldSource := "empty"
		if found.OldMode != "000000" {
			oldSource = commitSourcePrefix + baseOID
		}
		oldDoc, err := b.blob(ctx, found.OldPath, oldSource, found.OldOID, found.OldMode)
		if err != nil {
			return Diff{}, err
		}
		newSource := "empty"
		if found.NewMode != "000000" {
			newSource = commitSourcePrefix + stashOID
		}
		newDoc, err := b.blob(ctx, found.NewPath, newSource, found.NewOID, found.NewMode)
		if err != nil {
			return Diff{}, err
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
			transformed, attrErr := b.hasCommitTransformAttr(ctx, stashOID, found.NewPath)
			if attrErr != nil {
				return Diff{}, attrErr
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

func (s *Service) buildUntrackedStashDiff(ctx context.Context, key scopeKey, cid string, b *binding, stashOID, untrackedOID, cleanPath string) (Diff, error) {
	raw, err := git(ctx, b.gitCwd(), "ls-tree", "-z", untrackedOID, "--", cleanPath)
	if err != nil {
		return Diff{}, err
	}
	var mode, blobOID string
	for _, record := range strings.Split(string(raw), "\x00") {
		header, recPath, ok := strings.Cut(record, "\t")
		if !ok || recPath != cleanPath {
			continue
		}
		fields := strings.Fields(header)
		if len(fields) == 3 && fields[1] != "tree" {
			mode, blobOID = fields[0], fields[2]
			break
		}
	}
	if blobOID == "" {
		return Diff{}, problemf(ErrNotFound, "贮藏未跟踪文件树中不存在 %s", cleanPath)
	}
	zeroOID := strings.Repeat("0", len(untrackedOID))
	oldDoc := document(b.Root, cleanPath, "empty", "", nil)
	newDoc, err := b.blob(ctx, cleanPath, commitSourcePrefix+untrackedOID, blobOID, mode)
	if err != nil {
		return Diff{}, err
	}
	found := Change{
		ID:            version([]string{"stash-untracked", stashOID, untrackedOID, cleanPath, blobOID, mode}),
		ReviewVersion: version([]string{"stash-untracked", stashOID, untrackedOID, cleanPath, blobOID, mode}),
		NodeID:        nodeID(cleanPath),
		Group:         "stash",
		OldPath:       cleanPath,
		NewPath:       cleanPath,
		Status:        "A",
		OldMode:       "000000",
		NewMode:       mode,
		OldOID:        zeroOID,
		NewOID:        blobOID,
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
	}
	snap := &snapshot{owner: ownerScope(key), context: cid, diff: &diff, size: documentSize(oldDoc) + documentSize(newDoc)}
	diff.Snapshot = s.put(snap)
	return diff, nil
}

func isCommitInStashReflog(ctx context.Context, cwd, stashOID string) bool {
	if _, err := git(ctx, cwd, "rev-parse", "--verify", "--quiet", "refs/stash^{commit}"); err != nil {
		return false
	}
	raw, err := git(ctx, cwd, "log", "-g", "--no-show-signature", "--format=%H", "refs/stash", "--")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.TrimSpace(line) == stashOID {
			return true
		}
	}
	return false
}
