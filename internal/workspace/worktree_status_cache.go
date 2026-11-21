package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/rainchan/webmux/internal/session"
)

// worktreeStatusCacheTTL bounds how long a dormant worktree (no active session
// and unchanged Git/index/ref/root-dir metadata) may reuse its last status
// during List reads. Explicit Refresh and all write operations bypass this cache.
const worktreeStatusCacheTTL = 2 * time.Minute

func worktreeConcurrency() int {
	return min(max(runtime.NumCPU(), 4), 16)
}

type worktreeStatusKey struct {
	head       string
	branch     string
	detached   bool
	locked     bool
	prunable   bool
	operation  string
	indexModNS int64
	indexSize  int64
	dirModNS   int64
	refModNS   int64
	packedNS   int64
	fetchNS    int64
}

type cachedWorktreeStatus struct {
	key          worktreeStatusKey
	storedAt     time.Time
	dirty        bool
	hasUntracked bool
	hasIgnored   bool
	upstream     *string
	ahead        *int
	behind       *int
	upstreamGone bool
	changes      WorktreeChanges
}

func (c cachedWorktreeStatus) apply(tree *Worktree) {
	dirty := c.dirty
	tree.Dirty = &dirty
	tree.HasUntracked = c.hasUntracked
	tree.HasIgnored = c.hasIgnored
	if c.upstream != nil {
		tree.Upstream = strptr(*c.upstream)
	}
	if c.ahead != nil {
		v := *c.ahead
		tree.Ahead = &v
	}
	if c.behind != nil {
		v := *c.behind
		tree.Behind = &v
	}
	tree.UpstreamGone = c.upstreamGone
	ch := c.changes
	tree.Changes = &ch
}

func statModNS(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.ModTime().UnixNano()
}

func statModAndSize(path string) (int64, int64) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0
	}
	return info.ModTime().UnixNano(), info.Size()
}

func buildWorktreeStatusKey(commonDir, gitDir string, tree *Worktree, op *string, packedNS, fetchNS int64) worktreeStatusKey {
	key := worktreeStatusKey{
		detached: tree.Detached,
		locked:   tree.Locked,
		prunable: tree.Prunable,
		dirModNS: statModNS(tree.Path),
		packedNS: packedNS,
		fetchNS:  fetchNS,
	}
	if tree.Head != nil {
		key.head = *tree.Head
	}
	if tree.Branch != nil {
		key.branch = *tree.Branch
		key.refModNS = statModNS(filepath.Join(commonDir, "refs", "heads", *tree.Branch))
	}
	if op != nil {
		key.operation = *op
	}
	if gitDir != "" {
		key.indexModNS, key.indexSize = statModAndSize(filepath.Join(gitDir, "index"))
	}
	return key
}

func (s *Service) activeWorktreeIDs() map[string]bool {
	if s.manager == nil {
		return nil
	}
	sessions := s.manager.List()
	if len(sessions) == 0 {
		return nil
	}
	active := make(map[string]bool)
	for _, info := range sessions {
		if info.Status == session.StatusExited {
			continue
		}
		if assoc := s.Association(info.ID); assoc != nil && assoc.WorktreeID != "" {
			active[assoc.WorktreeID] = true
		}
	}
	return active
}

func (r *repositoryState) lookupWorktreeStatus(treeID string, key worktreeStatusKey, now time.Time, ttl time.Duration) (cachedWorktreeStatus, bool) {
	if ttl <= 0 {
		return cachedWorktreeStatus{}, false
	}
	r.statusCacheMu.Lock()
	defer r.statusCacheMu.Unlock()
	if r.statusCache == nil {
		return cachedWorktreeStatus{}, false
	}
	entry, ok := r.statusCache[treeID]
	if !ok || entry.key != key || now.Sub(entry.storedAt) > ttl {
		return cachedWorktreeStatus{}, false
	}
	return entry, true
}

func (r *repositoryState) storeWorktreeStatus(treeID string, entry cachedWorktreeStatus) {
	r.statusCacheMu.Lock()
	defer r.statusCacheMu.Unlock()
	if r.statusCache == nil {
		r.statusCache = make(map[string]cachedWorktreeStatus)
	}
	r.statusCache[treeID] = entry
}

func (r *repositoryState) pruneWorktreeStatusCache(live map[string]bool) {
	r.statusCacheMu.Lock()
	defer r.statusCacheMu.Unlock()
	for id := range r.statusCache {
		if !live[id] {
			delete(r.statusCache, id)
		}
	}
}

// AnchorPath returns the registered anchor path for repository id without
// spawning Git processes.
func (s *Service) AnchorPath(id string) (string, error) {
	r, err := s.repository(id)
	if err != nil {
		return "", err
	}
	return r.entry.AnchorPath, nil
}

// WorktreeInfo returns basic metadata for one repository and optional worktree
// using only `git worktree list`, avoiding full status scans of all worktrees.
func (s *Service) WorktreeInfo(ctx context.Context, repositoryID, worktreeID string) (Repository, *Worktree, error) {
	r, err := s.repository(repositoryID)
	if err != nil {
		return Repository{}, nil, err
	}
	repo := Repository{
		ID:         r.entry.ID,
		Name:       r.entry.DisplayName,
		CommonDir:  r.entry.CommonDir,
		AnchorPath: r.entry.AnchorPath,
	}
	if worktreeID == "" {
		return repo, nil, nil
	}
	data, err := s.git(ctx, r.entry.AnchorPath, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return repo, nil, nil
	}
	trees, err := parseWorktrees(data, r.entry.ID)
	if err != nil {
		return repo, nil, nil
	}
	for i := range trees {
		if trees[i].ID == worktreeID {
			tree := trees[i]
			return repo, &tree, nil
		}
	}
	return repo, nil, nil
}

func (s *Service) populateWorktreeStatus(ctx context.Context, commonDir string, tree *Worktree) {
	s.populateWorktreeStatusCached(ctx, nil, commonDir, tree, 0, 0, 0, time.Now(), nil)
}

func (s *Service) populateWorktreeStatusCached(ctx context.Context, r *repositoryState, commonDir string, tree *Worktree, cacheTTL time.Duration, packedNS, fetchNS int64, now time.Time, limit chan struct{}) {
	stat, statErr := os.Stat(tree.Path)
	if statErr != nil || !stat.IsDir() {
		tree.Error = fmt.Sprintf("工作树目录不可访问: %v", statErr)
		return
	}
	if tree.Kind == "bare" {
		tree.Available = true
		return
	}
	gitDir, fastCommon, fastOK := worktreeCommonDir(tree.Path)
	if !fastOK || fastCommon != commonDir {
		if limit != nil {
			select {
			case limit <- struct{}{}:
			case <-ctx.Done():
				tree.Error = ctx.Err().Error()
				return
			}
		}
		actual, checkErr := s.common(ctx, tree.Path)
		if limit != nil {
			<-limit
		}
		if checkErr != nil || actual != commonDir {
			tree.Error = fmt.Sprintf("无法验证工作树身份: %v", checkErr)
			return
		}
	}
	tree.Available = true
	if gitDir != "" {
		tree.Operation = detectOperationInDir(gitDir)
	} else {
		tree.Operation = detectOperation(tree.Path)
	}
	var key worktreeStatusKey
	if r != nil {
		key = buildWorktreeStatusKey(commonDir, gitDir, tree, tree.Operation, packedNS, fetchNS)
		if cacheTTL > 0 {
			if cached, ok := r.lookupWorktreeStatus(tree.ID, key, now, cacheTTL); ok {
				cached.apply(tree)
				return
			}
		}
	}
	if limit != nil {
		select {
		case limit <- struct{}{}:
		case <-ctx.Done():
			tree.Error = ctx.Err().Error()
			return
		}
		defer func() { <-limit }()
	}
	data, statusErr := s.git(ctx, tree.Path, "status", "--porcelain=v2", "-z", "--branch", "--untracked-files=normal", "--ignored=matching")
	if statusErr != nil {
		tree.Error = statusErr.Error()
		return
	}
	status, parseErr := parseStatus(data)
	if parseErr != nil {
		tree.Error = parseErr.Error()
		return
	}
	dirty := status.dirty
	tree.Dirty, tree.HasUntracked, tree.HasIgnored = &dirty, status.untracked, status.ignored
	tree.Upstream, tree.Ahead, tree.Behind, tree.UpstreamGone = status.upstream, status.ahead, status.behind, status.upstreamGone
	tree.Changes = &status.changes
	if r != nil {
		r.storeWorktreeStatus(tree.ID, cachedWorktreeStatus{
			key: key, storedAt: now, dirty: status.dirty, hasUntracked: status.untracked, hasIgnored: status.ignored,
			upstream: status.upstream, ahead: status.ahead, behind: status.behind, upstreamGone: status.upstreamGone, changes: status.changes,
		})
	}
}
