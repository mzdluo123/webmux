package workspace

import (
	"context"
	"path/filepath"
	"slices"
	"sort"

	"github.com/rainchan/webmux/internal/session"
)

func (s *Service) notifyLocked() { close(s.changed); s.changed = make(chan struct{}) }
func (s *Service) beginStart(path string) error {
	s.coord.Lock()
	defer s.coord.Unlock()
	for reserved := range s.reservations {
		if within(reserved, path) {
			return problemf(ErrConflict, "目标工作树正在删除，请稍后刷新")
		}
	}
	s.starts[path]++
	return nil
}
func (s *Service) endStart(path string) {
	s.coord.Lock()
	defer s.coord.Unlock()
	s.starts[path]--
	if s.starts[path] == 0 {
		delete(s.starts, path)
	}
	s.notifyLocked()
}
func (s *Service) reserve(ctx context.Context, path string) error {
	s.coord.Lock()
	for reserved := range s.reservations {
		if overlap(reserved, path) {
			s.coord.Unlock()
			return problemf(ErrConflict, "重叠工作树已有删除操作")
		}
	}
	s.reservations[path] = true
	for {
		starting := false
		for cwd := range s.starts {
			if within(path, cwd) {
				starting = true
				break
			}
		}
		if !starting {
			s.coord.Unlock()
			return nil
		}
		changed := s.changed
		s.coord.Unlock()
		select {
		case <-ctx.Done():
			s.unreserve(path)
			return ctx.Err()
		case <-changed:
		}
		s.coord.Lock()
	}
}
func (s *Service) unreserve(path string) {
	s.coord.Lock()
	defer s.coord.Unlock()
	delete(s.reservations, path)
	s.notifyLocked()
}

// Association belongs to the workspace plugin, not the terminal host.
type Association struct {
	RepositoryID string
	WorktreeID   string
	WorktreePath string
}

// Association returns a caller-owned cached association without Git I/O.
func (s *Service) Association(id string) *Association {
	s.associationCacheMu.RLock()
	defer s.associationCacheMu.RUnlock()
	value, ok := s.associations[id]
	if !ok {
		return nil
	}
	return &value
}

func (s *Service) association(ctx context.Context, cwd string) *Association {
	if s.gitPath == "" {
		return nil
	}
	common, err := s.common(ctx, cwd)
	if err != nil {
		return nil
	}
	root, err := s.git(ctx, cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil
	}
	path, err := canonical(outputLine(root))
	if err != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.capability.RegistryAvailable {
		return nil
	}
	for _, r := range s.entries {
		if r.entry.CommonDir == common {
			return &Association{RepositoryID: r.entry.ID, WorktreeID: worktreeID(r.entry.ID, path), WorktreePath: path}
		}
	}
	return nil
}

// Association commits are ordered with registry removal. Git resolution never
// holds the registry/session locks, and a removed registration cannot reappear.
func (s *Service) commitAssociation(info session.Info, association *Association, preserve bool) {
	s.associationMu.Lock()
	defer s.associationMu.Unlock()
	item, err := s.manager.Get(info.ID)
	if err != nil || item.Info().Cwd != info.Cwd {
		return
	}
	s.mu.Lock()
	s.associationCacheMu.Lock()
	current, exists := s.associations[info.ID]
	registered := false
	id := ""
	if association != nil {
		id = association.RepositoryID
	} else if preserve && exists {
		id = current.RepositoryID
	}
	for _, r := range s.entries {
		if r.entry.ID == id {
			registered = true
			break
		}
	}
	if !registered {
		association = nil
	} else if association == nil && preserve {
		association = &current
	}
	changed := exists != (association != nil) || association != nil && current != *association
	if association == nil {
		delete(s.associations, info.ID)
	} else {
		if s.associations == nil {
			s.associations = make(map[string]Association)
		}
		s.associations[info.ID] = *association
	}
	s.associationCacheMu.Unlock()
	s.mu.Unlock()
	// Removal can win after the initial lookup and before this cache commit.
	// Its observer deletes older entries; this lookup covers the late writer.
	if _, err := s.manager.Get(info.ID); err != nil {
		s.forgetAssociation(info.ID)
		return
	}
	if changed {
		s.manager.Notify(info.ID)
	}
}
func (s *Service) reassociate(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	for _, info := range s.manager.List() {
		if ctx.Err() != nil {
			return
		}
		association := s.association(ctx, info.Cwd)
		// If the original checkout disappeared, retain its identity instead of
		// silently rebinding the record to a newly created checkout at that path.
		if current := s.Association(info.ID); current != nil && association != nil && *association != *current {
			association = nil
		}
		s.commitAssociation(info, association, true)
	}
}

// PrepareResume preserves worktree-removal exclusion and commits an optional
// repository association only after the chat coordinator restores its runtime.
func (s *Service) PrepareResume(ctx context.Context, cwd string) (func(*session.Session), func(), error) {
	if err := s.beginStart(cwd); err != nil {
		return nil, nil, handoffLocationError(err)
	}
	lookup, cancel := context.WithTimeout(ctx, readTimeout)
	association := s.association(lookup, cwd)
	cancel()
	if err := ctx.Err(); err != nil {
		s.endStart(cwd)
		return nil, nil, err
	}
	return func(item *session.Session) {
		s.commitAssociation(item.Info(), association, false)
	}, func() { s.endStart(cwd) }, nil
}

func (s *Service) targetSessions(path string) []session.Info {
	result := make([]session.Info, 0)
	for _, info := range s.manager.List() {
		cwd, err := canonicalTarget(info.Cwd)
		if err != nil {
			cwd = filepath.Clean(info.Cwd)
		}
		if within(path, cwd) || within(path, info.Cwd) {
			result = append(result, info)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// sameSessions reports whether a and b hold the same sessions in the same
// directories, each exited in both or in neither.
func sameSessions(a, b []session.Info) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Cwd != b[i].Cwd {
			return false
		}
		if (a[i].Status == session.StatusExited) != (b[i].Status == session.StatusExited) {
			return false
		}
	}
	return true
}

// remainingSessions reports whether after holds only sessions of before, in
// the same directories. Sessions may leave: a closed hybrid shell is removed
// once its driver records the close.
func remainingSessions(before, after []session.Info) bool {
	for _, info := range after {
		i := slices.IndexFunc(before, func(old session.Info) bool { return old.ID == info.ID })
		if i < 0 || before[i].Cwd != info.Cwd {
			return false
		}
	}
	return true
}

type associationCleanup struct{ service *Service }

func (observer associationCleanup) Observe(event session.Event) {
	if event.Type == "removed" {
		observer.service.forgetAssociation(event.ID)
	}
}

func (s *Service) forgetAssociation(id string) {
	s.associationCacheMu.Lock()
	delete(s.associations, id)
	s.associationCacheMu.Unlock()
}
