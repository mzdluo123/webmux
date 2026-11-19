package workspace

import (
	"log/slog"
	"slices"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	maxKnownWorktreesPerRepo = 1000
	maxHomeWorktreeIDUnits   = 128
)

// validHomeWorktreeID matches internal/settings's UTF-16 code unit limit for
// repository and worktree IDs stored in homeWorktrees.
func validHomeWorktreeID(s string) bool {
	if s == "" || !utf8.ValidString(s) {
		return false
	}
	units := 0
	for _, r := range s {
		units += utf16.RuneLen(r)
		if units > maxHomeWorktreeIDUnits {
			return false
		}
	}
	return true
}

// UseHomeWorktrees installs, once at assembly before serving, the callback
// that pins externally discovered linked worktrees onto the shared
// homeWorktrees setting.
func (s *Service) UseHomeWorktrees(pin func(repoID, mainWorktreeID string, added []string) error) {
	s.pinHomeWorktrees = pin
}

// UseWorktreePins installs, once at assembly before serving, the callbacks
// that prune removed worktrees on healthy snapshots and drop unregistered
// repositories from the shared worktreePins setting.
func (s *Service) UseWorktreePins(retain func(repoID string, liveIDs []string) error, dropRepository func(repoID string) error) {
	s.retainWorktreePins = retain
	s.dropRepositoryWorktreePins = dropRepository
}

func (s *Service) purgeRepositoryWorktreePins(repoID string) {
	if s.dropRepositoryWorktreePins == nil {
		return
	}
	if err := s.dropRepositoryWorktreePins(repoID); err != nil {
		slog.Warn("drop worktree pins of unregistered repository failed", "repository", repoID, "error", err)
	}
}

func observedWorktreeIDs(trees []Worktree) []string {
	if len(trees) == 0 {
		return nil
	}
	out := make([]string, 0, len(trees))
	seen := make(map[string]bool, len(trees))
	for _, tree := range trees {
		if !validHomeWorktreeID(tree.ID) || seen[tree.ID] {
			continue
		}
		seen[tree.ID] = true
		out = append(out, tree.ID)
		if len(out) >= maxKnownWorktreesPerRepo {
			break
		}
	}
	return out
}

func mainWorktreeID(repo Repository) string {
	for _, tree := range repo.Worktrees {
		if tree.Kind == "main" && validHomeWorktreeID(tree.ID) {
			return tree.ID
		}
	}
	return ""
}

// recordManagedWorktree records a worktree created or rebuilt through WebMux
// while the caller still holds r.gate, so callers that opt out of pinning
// (such as `webmux worktree create --no-pin`) are not treated as external
// additions by a subsequent snapshot.
func (s *Service) recordManagedWorktree(r *repositoryState, actual Repository, createdID string) {
	if !validHomeWorktreeID(createdID) {
		return
	}
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.Lock()
	if err := s.requireRegistryLocked(); err != nil || !s.containsLocked(r) {
		s.mu.Unlock()
		return
	}
	seeded := r.knownSeeded
	previous := slices.Clone(r.knownWorktrees)
	s.mu.Unlock()

	var nextKnown []string
	if !seeded {
		nextKnown = observedWorktreeIDs(actual.Worktrees)
		if !slices.Contains(nextKnown, createdID) && len(nextKnown) < maxKnownWorktreesPerRepo {
			nextKnown = append(nextKnown, createdID)
		}
	} else {
		if slices.Contains(previous, createdID) || len(previous) >= maxKnownWorktreesPerRepo {
			return
		}
		nextKnown = append(previous, createdID)
	}
	s.commitKnownLocked(r, true, nextKnown)
}

// reconcileKnownWorktrees updates r's observed worktree baseline on a settled
// available snapshot and pins externally added linked worktrees onto the
// shared homeWorktrees setting before advancing their baseline entries.
func (s *Service) reconcileKnownWorktrees(r *repositoryState, repo Repository) {
	if repo.Availability != "available" || repo.Error != "" {
		return
	}
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.Lock()
	if err := s.requireRegistryLocked(); err != nil || !s.containsLocked(r) {
		s.mu.Unlock()
		return
	}
	seeded := r.knownSeeded
	previous := slices.Clone(r.knownWorktrees)
	s.mu.Unlock()

	if s.retainWorktreePins != nil && validHomeWorktreeID(repo.ID) {
		liveIDs := make([]string, 0, len(repo.Worktrees))
		for _, tree := range repo.Worktrees {
			if validHomeWorktreeID(tree.ID) {
				liveIDs = append(liveIDs, tree.ID)
			}
		}
		if err := s.retainWorktreePins(repo.ID, liveIDs); err != nil {
			slog.Warn("prune worktree pins of repository failed", "repository", repo.ID, "error", err)
		}
	}

	currentIDs := observedWorktreeIDs(repo.Worktrees)
	if !seeded {
		s.commitKnownLocked(r, true, currentIDs)
		return
	}

	seen := make(map[string]bool, len(previous))
	for _, id := range previous {
		seen[id] = true
	}
	var added []string
	addedSet := map[string]bool{}
	for _, tree := range repo.Worktrees {
		if tree.Kind == "linked" && tree.Available && !tree.Prunable &&
			validHomeWorktreeID(tree.ID) && !seen[tree.ID] && !addedSet[tree.ID] {
			added = append(added, tree.ID)
			addedSet[tree.ID] = true
		}
	}

	nextKnown := currentIDs
	if len(added) > 0 && s.pinHomeWorktrees != nil && validHomeWorktreeID(repo.ID) {
		if err := s.pinHomeWorktrees(repo.ID, mainWorktreeID(repo), added); err != nil {
			// Retain unpinned additions as unknown so the next healthy read
			// retries them instead of dropping the auto-pin permanently.
			retained := make([]string, 0, len(currentIDs))
			for _, id := range currentIDs {
				if !addedSet[id] {
					retained = append(retained, id)
				}
			}
			nextKnown = retained
		}
	}

	if slices.Equal(previous, nextKnown) {
		return
	}
	s.commitKnownLocked(r, true, nextKnown)
}

func (s *Service) containsLocked(r *repositoryState) bool {
	for _, candidate := range s.entries {
		if candidate == r {
			return true
		}
	}
	return false
}

func (s *Service) commitKnownLocked(r *repositoryState, nextSeeded bool, nextKnown []string) {
	if s.store == nil {
		s.mu.Lock()
		if s.containsLocked(r) {
			r.knownSeeded = nextSeeded
			r.knownWorktrees = slices.Clone(nextKnown)
		}
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	if err := s.requireRegistryLocked(); err != nil || !s.containsLocked(r) {
		s.mu.Unlock()
		return
	}
	stored := registry{Version: 1, Repositories: make([]registration, 0, len(s.entries))}
	for _, entry := range s.entries {
		item := entry.registrationLocked()
		if entry == r {
			item.KnownSeeded = nextSeeded
			if len(nextKnown) > 0 {
				item.KnownWorktrees = slices.Clone(nextKnown)
			} else {
				item.KnownWorktrees = nil
			}
		}
		stored.Repositories = append(stored.Repositories, item)
	}
	s.mu.Unlock()
	if !s.store.Fits(stored) {
		return
	}
	if err := s.commitRegistry(stored); err != nil {
		return
	}
	s.mu.Lock()
	if s.containsLocked(r) {
		r.knownSeeded = nextSeeded
		r.knownWorktrees = slices.Clone(nextKnown)
	}
	s.mu.Unlock()
}
