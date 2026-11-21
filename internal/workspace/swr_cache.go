package workspace

import (
	"context"
	"reflect"
	"slices"
	"sync"
	"time"
)

// defaultSnapshotFreshTTL bounds how long a cached repository snapshot is
// considered fresh during opt-in stale-while-revalidate reads (?stale=1) when
// neither the on-disk repo fingerprint nor an explicit mutation marked it dirty.
const defaultSnapshotFreshTTL = 5 * time.Second

// ListOptions controls how Service.ListWithOptions serves repository snapshots.
type ListOptions struct {
	// AllowStale enables stale-while-revalidate: repositories with a cached
	// snapshot return immediately and revalidate asynchronously when stale or
	// dirty, notifying /ws/repositories only if material fields changed.
	AllowStale bool
}

func (s *Service) snapshotFreshTTL() time.Duration {
	if s.freshTTL > 0 {
		return s.freshTTL
	}
	return defaultSnapshotFreshTTL
}

func (s *Service) closeSWR() {
	s.swrMu.Lock()
	s.swrClosed = true
	cancel := s.swrCancel
	s.swrMu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.swrWG.Wait()
}

func (r *repositoryState) snapshotGen() uint64 {
	r.snapMu.Lock()
	defer r.snapMu.Unlock()
	return r.snapGen
}

func (r *repositoryState) markSnapshotDirty() {
	r.snapMu.Lock()
	r.snapGen++
	r.snapDirty = true
	r.snapMu.Unlock()
}

func (r *repositoryState) storeSnapshot(repo Repository, now time.Time, fp uint64, startedGen uint64) {
	cloned := cloneRepository(repo)
	r.snapMu.Lock()
	r.lastSnapshot = cloned
	r.hasSnapshot = true
	r.lastSnapshotAt = now
	r.lastFingerprint = fp
	r.snapDirty = r.snapGen != startedGen
	r.snapMu.Unlock()
}

func (r *repositoryState) cachedSnapshot(now time.Time, freshTTL time.Duration, currentFP uint64) (repo Repository, fresh bool, ok bool) {
	r.snapMu.Lock()
	defer r.snapMu.Unlock()
	if !r.hasSnapshot {
		return Repository{}, false, false
	}
	fresh = !r.snapDirty && r.lastFingerprint == currentFP && now.Sub(r.lastSnapshotAt) <= freshTTL
	return cloneRepository(r.lastSnapshot), fresh, true
}

func cloneRepository(repo Repository) Repository {
	repo.Worktrees = slices.Clone(repo.Worktrees)
	return repo
}

// repositoriesMateriallyEqual compares two snapshots ignoring CheckedAt so
// background SWR revalidation only pushes /ws/repositories when something the
// user sees actually changed.
func repositoriesMateriallyEqual(a, b Repository) bool {
	a.CheckedAt = time.Time{}
	b.CheckedAt = time.Time{}
	return reflect.DeepEqual(a, b)
}

func (s *Service) markRepositoriesDirty(ids []string) {
	if len(ids) == 0 {
		return
	}
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	s.mu.Lock()
	var targets []*repositoryState
	for _, r := range s.entries {
		if _, ok := set[r.entry.ID]; ok {
			targets = append(targets, r)
		}
	}
	s.mu.Unlock()
	for _, r := range targets {
		r.markSnapshotDirty()
	}
}

func (s *Service) List(ctx context.Context) ([]Repository, error) {
	return s.ListWithOptions(ctx, ListOptions{})
}

func (s *Service) ListWithOptions(ctx context.Context, opts ListOptions) ([]Repository, error) {
	s.mu.Lock()
	if err := s.requireRegistryLocked(); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	entries := append([]*repositoryState(nil), s.entries...)
	s.mu.Unlock()

	now := time.Now()
	freshTTL := s.snapshotFreshTTL()
	// Repositories are independent, so a slow one must not serialize the rest.
	// Each snapshot keeps its own gate and shared-flight semantics.
	snapshots := make([]Repository, len(entries))
	available := make([]bool, len(entries))
	failures := make([]error, len(entries))
	var syncScanned bool
	var wg sync.WaitGroup
	for i, r := range entries {
		if opts.AllowStale {
			fp := repoFingerprint(r.entry)
			if snap, fresh, ok := r.cachedSnapshot(now, freshTTL, fp); ok {
				if s.registered(r) {
					snap.InitInstruction = s.initInstruction(r)
					snapshots[i], available[i] = snap, true
					if !fresh {
						s.triggerRevalidate(r)
					}
				}
				continue
			}
		}
		syncScanned = true
		wg.Add(1)
		go func(i int, r *repositoryState) {
			defer wg.Done()
			snapshot, err := s.snapshotWithCache(ctx, r, true)
			if err != nil {
				failures[i] = err
				return
			}
			if s.registered(r) {
				snapshots[i], available[i] = snapshot, true
			}
		}(i, r)
	}
	wg.Wait()
	out := make([]Repository, 0, len(entries))
	for i := range entries {
		if available[i] {
			out = append(out, snapshots[i])
		} else if failures[i] != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			// A single unreadable repository keeps the rest of the list and
			// its sessions reachable instead of failing the whole response.
			if s.registered(entries[i]) {
				out = append(out, s.unavailableRepository(entries[i], failures[i]))
			}
		}
	}
	if syncScanned {
		s.reassociate(ctx)
	}
	return out, nil
}

// triggerRevalidate starts at most one background refresh per repository.
// When the refresh completes, /ws/repositories subscribers are notified only if
// the repository's material state changed, preventing refresh feedback loops.
func (s *Service) triggerRevalidate(r *repositoryState) {
	s.swrMu.Lock()
	if s.swrClosed || s.swrCtx == nil {
		s.swrMu.Unlock()
		return
	}
	r.snapMu.Lock()
	if r.revalidating {
		r.snapMu.Unlock()
		s.swrMu.Unlock()
		return
	}
	before := cloneRepository(r.lastSnapshot)
	hadBefore := r.hasSnapshot
	r.revalidating = true
	r.snapMu.Unlock()
	ctx := s.swrCtx
	s.swrWG.Add(1)
	s.swrMu.Unlock()

	go func() {
		defer s.swrWG.Done()
		defer func() {
			r.snapMu.Lock()
			r.revalidating = false
			r.snapMu.Unlock()
		}()
		updated, err := s.snapshotWithCache(ctx, r, true)
		if err != nil || !s.registered(r) {
			return
		}
		s.reassociate(ctx)
		if !hadBefore || !repositoriesMateriallyEqual(before, updated) {
			s.notifyWorkspaces()
		}
	}()
}
