package workspace

import (
	"context"
	"fmt"
	"path/filepath"
)

// Register adds the repository at path. A repository already registered is
// returned as it is. Its initialization instruction is set afterwards with
// SetInitInstruction.
func (s *Service) Register(ctx context.Context, path string) (Repository, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	r, created, err := s.registerRepository(ctx, path)
	if err != nil {
		return Repository{}, false, err
	}
	snapshot, err := s.snapshot(ctx, r)
	s.reassociate(ctx)
	return snapshot, created, err
}

// registerRepository commits identity without a post-commit Git snapshot. The
// caller supplies a deadline and decides whether it needs expensive Git state.
func (s *Service) registerRepository(ctx context.Context, path string) (*repositoryState, bool, error) {
	if err := s.requireRegistry(); err != nil {
		return nil, false, err
	}
	if path == "" {
		return nil, false, problemf(ErrInvalid, "请选择仓库目录")
	}
	common, anchor, err := s.locate(ctx, path)
	if err != nil {
		return nil, false, err
	}
	s.saveMu.Lock()
	s.mu.Lock()
	if err := s.requireRegistryLocked(); err != nil {
		s.mu.Unlock()
		s.saveMu.Unlock()
		return nil, false, err
	}
	for _, r := range s.entries {
		if r.entry.CommonDir == common {
			s.mu.Unlock()
			s.saveMu.Unlock()
			return r, false, nil
		}
	}
	if len(s.entries) >= MaxRepositories {
		s.mu.Unlock()
		s.saveMu.Unlock()
		return nil, false, problemf(ErrInvalid, "最多登记 %d 个仓库，请先移除不再使用的仓库", MaxRepositories)
	}
	id, err := randomID()
	if err != nil {
		s.mu.Unlock()
		s.saveMu.Unlock()
		return nil, false, err
	}
	r := newRepository(registration{ID: id, CommonDir: common, AnchorPath: anchor, DisplayName: filepath.Base(anchor)})
	next := append(append([]*repositoryState(nil), s.entries...), r)
	s.mu.Unlock()
	err = s.save(next)
	if err == nil {
		s.mu.Lock()
		s.entries = next
		s.mu.Unlock()
	}
	s.saveMu.Unlock()
	if err != nil {
		return nil, false, fmt.Errorf("保存仓库清单: %w", err)
	}
	s.notifyWorkspaces()
	return r, true, nil
}
