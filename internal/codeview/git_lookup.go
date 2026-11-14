package codeview

import (
	"context"
	"strings"
	"time"
)

func (s *Service) verifyTargetExists(ctx context.Context, repositoryID, worktreeID string) error {
	if repositoryID == "" || strings.ContainsRune(repositoryID, '\x00') {
		return problemf(ErrInvalid, "无效仓库标识")
	}
	resolver, err := s.resolver()
	if err != nil {
		return err
	}
	loc, err := resolver.LocateRepository(ctx, repositoryID)
	if err != nil {
		return problemf(errorKind(err), "%s", err.Error())
	}
	if !loc.Available {
		return problemf(ErrUnavailable, "%s", loc.Reason)
	}
	if worktreeID != "" {
		if strings.ContainsRune(worktreeID, '\x00') {
			return problemf(ErrInvalid, "无效工作树标识")
		}
		if _, err := resolver.LocateWorktree(ctx, repositoryID, worktreeID); err != nil {
			return problemf(errorKind(err), "%s", err.Error())
		}
	}
	return nil
}

func splitOwnerScope(key scopeKey) (repoID, worktreeID string, isGitTarget bool) {
	switch key.kind {
	case scopeRepository:
		return key.owner, "", true
	case scopeWorktree:
		r, w, ok := strings.Cut(key.owner, "\x00")
		if !ok {
			return "", "", false
		}
		return r, w, true
	default:
		return "", "", false
	}
}

func (s *Service) resolveGitContext(ctx context.Context, repositoryID, worktreeID, cid string) (Target, error) {
	if cid == "" {
		return Target{}, problemf(ErrInvalid, "缺少浏览上下文标识")
	}
	if err := s.verifyTargetExists(ctx, repositoryID, worktreeID); err != nil {
		return Target{}, err
	}
	now := time.Now()
	s.mu.Lock()
	var matchedKey *scopeKey
	for k, b := range s.roots {
		if b != nil && b.ID == cid && now.Sub(b.timeUsed) <= Lifetime {
			copyKey := k
			matchedKey = &copyKey
			break
		}
	}
	s.mu.Unlock()
	if matchedKey == nil {
		return Target{}, problemf(ErrGone, "浏览上下文已过期，请刷新")
	}
	ownerRepo, ownerWT, ok := splitOwnerScope(*matchedKey)
	if !ok || ownerRepo != repositoryID {
		return Target{}, problemf(ErrNotFound, "浏览上下文不存在")
	}
	if worktreeID != "" && ownerWT != worktreeID {
		return Target{}, problemf(ErrContextMismatch, "浏览上下文不属于所选工作树")
	}
	return Target{RepositoryID: ownerRepo, WorktreeID: ownerWT}, nil
}

func (s *Service) resolveGitSnapshot(ctx context.Context, repositoryID, worktreeID, cid, sid string) (Target, error) {
	if sid == "" {
		return Target{}, problemf(ErrInvalid, "缺少快照标识")
	}
	if err := s.verifyTargetExists(ctx, repositoryID, worktreeID); err != nil {
		return Target{}, err
	}
	now := time.Now()
	s.mu.Lock()
	v := s.snapshots[sid]
	var snapOwner scopeKey
	var snapContext string
	expired := v == nil || now.Sub(v.created) > Lifetime
	if v != nil {
		snapOwner = v.owner
		snapContext = v.context
	}
	s.mu.Unlock()
	if v != nil {
		ownerRepo, ownerWT, ok := splitOwnerScope(snapOwner)
		if !ok || ownerRepo != repositoryID {
			return Target{}, problemf(ErrNotFound, "快照不存在")
		}
		if worktreeID != "" && ownerWT != worktreeID {
			return Target{}, problemf(ErrContextMismatch, "快照不属于所选工作树")
		}
		if cid != "" && snapContext != cid {
			return Target{}, problemf(ErrContextMismatch, "快照不属于当前浏览上下文")
		}
		if expired {
			return Target{}, problemf(ErrGone, "内容快照已过期，请刷新")
		}
		return Target{RepositoryID: ownerRepo, WorktreeID: ownerWT}, nil
	}
	return Target{}, problemf(ErrGone, "内容快照已过期，请刷新")
}

func normalizeGitSide(side string) (string, error) {
	switch side {
	case "", "old", "new":
		return side, nil
	case "before":
		return "old", nil
	case "after":
		return "new", nil
	default:
		return "", problemf(ErrInvalid, "无效快照侧标识，请使用 before 或 after")
	}
}

// GitContext resolves a repository-level (empty worktreeID) or worktree-level
// session-independent browsing context.
func (s *Service) GitContext(ctx context.Context, repositoryID, worktreeID string) (Context, error) {
	if worktreeID == "" {
		return s.RepositoryContext(ctx, repositoryID)
	}
	return s.WorktreeContext(ctx, repositoryID, worktreeID)
}

// GitTree lists a directory in a repository-bound Git browsing context.
func (s *Service) GitTree(ctx context.Context, repositoryID, worktreeID, cid, nid string, include bool, cursor string) (Listing, error) {
	t, err := s.resolveGitContext(ctx, repositoryID, worktreeID, cid)
	if err != nil {
		return Listing{}, err
	}
	return s.TargetTree(ctx, t, cid, nid, include, cursor)
}

// GitSearch searches filenames in a repository-bound Git browsing context.
func (s *Service) GitSearch(ctx context.Context, repositoryID, worktreeID, cid, q string, include bool) (Listing, error) {
	t, err := s.resolveGitContext(ctx, repositoryID, worktreeID, cid)
	if err != nil {
		return Listing{}, err
	}
	return s.TargetSearch(ctx, t, cid, q, include)
}

// GitFile reads a file from worktree, index, head, or commit:{oid} in a
// repository-bound Git browsing context.
func (s *Service) GitFile(ctx context.Context, repositoryID, worktreeID, cid, nid, source string) (File, error) {
	t, err := s.resolveGitContext(ctx, repositoryID, worktreeID, cid)
	if err != nil {
		return File{}, err
	}
	if commitOID, ok := strings.CutPrefix(source, commitSourcePrefix); ok {
		return s.CommitFile(ctx, t, cid, commitOID, nid)
	}
	return s.TargetFileSource(ctx, t, cid, nid, source)
}

// GitChanges lists working tree and index changes in a repository-bound Git context.
func (s *Service) GitChanges(ctx context.Context, repositoryID, worktreeID, cid string, query ChangeQuery) (Changes, error) {
	t, err := s.resolveGitContext(ctx, repositoryID, worktreeID, cid)
	if err != nil {
		return Changes{}, err
	}
	return s.TargetChanges(ctx, t, cid, query)
}

// GitDiff captures a frozen diff snapshot for a working tree or index change.
func (s *Service) GitDiff(ctx context.Context, repositoryID, worktreeID, cid, changeID string) (Diff, error) {
	t, err := s.resolveGitContext(ctx, repositoryID, worktreeID, cid)
	if err != nil {
		return Diff{}, err
	}
	return s.TargetDiff(ctx, t, cid, changeID)
}

// GitCompare compares baseOID (or empty tree when baseOID is empty) against
// targetOID and optionally captures the frozen Diff snapshot for Path or ChangeID.
func (s *Service) GitCompare(ctx context.Context, repositoryID, worktreeID string, q CompareQuery) (Compare, error) {
	t, err := s.resolveGitContext(ctx, repositoryID, worktreeID, q.ContextID)
	if err != nil {
		return Compare{}, err
	}
	if q.TargetOID == "" {
		return Compare{}, problemf(ErrInvalid, "目标提交标识不能为空")
	}
	var cleanPath string
	if q.Path != "" {
		cleanPath, err = nodePath(nodeID(q.Path))
		if err != nil || cleanPath == "" {
			return Compare{}, problemf(ErrInvalid, "无效比较路径")
		}
	}
	changes, err := s.CommitChanges(ctx, t, q.ContextID, q.BaseOID, q.TargetOID, ChangeQuery{
		Cursor: q.Cursor,
		Q:      q.Q,
	})
	if err != nil {
		return Compare{}, err
	}
	res := Compare{
		BaseOID:   q.BaseOID,
		TargetOID: q.TargetOID,
		Changes:   changes,
	}
	if q.ChangeID != "" || cleanPath != "" {
		changeID := q.ChangeID
		if changeID == "" {
			filtered, fErr := s.CommitChanges(ctx, t, q.ContextID, q.BaseOID, q.TargetOID, ChangeQuery{
				NodeID: nodeID(cleanPath),
			})
			if fErr != nil {
				return Compare{}, fErr
			}
			for _, entry := range filtered.Entries {
				if entry.NewPath == cleanPath {
					changeID = entry.ID
					break
				}
				if changeID == "" && entry.OldPath == cleanPath {
					changeID = entry.ID
				}
			}
			if changeID == "" {
				return Compare{}, problemf(ErrNotFound, "指定的历史变更路径不存在")
			}
		}
		diff, dErr := s.CommitDiff(ctx, t, q.ContextID, q.BaseOID, q.TargetOID, changeID)
		if dErr != nil {
			return Compare{}, dErr
		}
		if cleanPath != "" && diff.Change.NewPath != cleanPath && diff.Change.OldPath != cleanPath {
			return Compare{}, problemf(ErrInvalid, "变更标识与指定路径不匹配")
		}
		res.Diff = &diff
	}
	return res, nil
}

// GitSnapshotFile reads the before/after full file document of a snapshot.
func (s *Service) GitSnapshotFile(ctx context.Context, repositoryID, worktreeID, cid, sid, side string) (File, error) {
	normSide, err := normalizeGitSide(side)
	if err != nil {
		return File{}, err
	}
	t, err := s.resolveGitSnapshot(ctx, repositoryID, worktreeID, cid, sid)
	if err != nil {
		return File{}, err
	}
	return s.TargetSnapshotFile(ctx, t, sid, normSide)
}

// GitSnapshotRaw opens a verified download transfer for the before/after side of a snapshot.
func (s *Service) GitSnapshotRaw(ctx context.Context, repositoryID, worktreeID, cid, sid, side string) (*Transfer, error) {
	normSide, err := normalizeGitSide(side)
	if err != nil {
		return nil, err
	}
	t, err := s.resolveGitSnapshot(ctx, repositoryID, worktreeID, cid, sid)
	if err != nil {
		return nil, err
	}
	return s.TargetRaw(ctx, t, sid, normSide)
}
