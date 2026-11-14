package hybrid

import (
	"context"
	"fmt"
	"slices"
)

// Records lists the hybrid session records, most recently updated first;
// recoverable keeps the closed and interrupted ones.
func (s *Service) Records(recoverable bool) ([]Record, error) {
	if err := s.records.unavailable(); err != nil {
		return nil, err
	}
	all := s.records.all()
	out := all[:0]
	for _, rec := range all {
		if !recoverable || rec.State != StateActive {
			out = append(out, rec)
		}
	}
	slices.SortStableFunc(out, func(a, b Record) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	return out, nil
}

// Record returns record id.
func (s *Service) Record(id string) (Record, error) {
	if err := s.records.unavailable(); err != nil {
		return Record{}, err
	}
	rec, ok := s.records.get(id)
	if !ok {
		return Record{}, ErrNotFound
	}
	return rec, nil
}

// Recoverable returns record id when it can be resumed or removed: closed
// or interrupted. An active session fails with ErrSwitching during a
// switch and ErrNotArchived otherwise.
func (s *Service) Recoverable(id string) (Record, error) {
	rec, err := s.Record(id)
	if err != nil || rec.State != StateActive {
		return rec, err
	}
	if metadata := s.Metadata(id); metadata != nil && metadata.To != "" {
		return Record{}, ErrSwitching
	}
	return Record{}, ErrNotArchived
}

// claim marks record id as being resumed or removed; another resume or
// removal of it meanwhile fails with ErrSwitching.
func (s *Service) claim(id string) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.closed:
		return nil, fmt.Errorf("%w: 服务已关闭", ErrUnavailable)
	case s.recovering[id]:
		return nil, ErrSwitching
	}
	s.recovering[id] = true
	return func() {
		s.mu.Lock()
		delete(s.recovering, id)
		s.mu.Unlock()
	}, nil
}

func (s *Service) claimRecordConversation(ctx context.Context, id string) (func(), error) {
	if s.conversations == nil {
		return func() {}, nil
	}
	b, err := s.conversations.Binding(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: conversation identity unavailable", ErrUnavailable)
	}
	return s.ClaimConversation(b.ConversationID, "")
}

func (s *Service) removeRecord(ctx context.Context, id string) error {
	release, err := s.claim(id)
	if err != nil {
		return err
	}
	defer release()
	if _, err := s.Recoverable(id); err != nil {
		return err
	}
	if err := s.clear(ctx, id); err != nil {
		return err
	}
	if s.conversations != nil {
		if err := s.conversations.MarkRuntimeForgetting(ctx, id); err != nil {
			return fmt.Errorf("%w: conversation cleanup unavailable", ErrUnavailable)
		}
	}
	if err := s.records.remove(id); err != nil {
		return err
	}
	if err := s.forgetConversation(context.WithoutCancel(ctx), id); err != nil {
		return err
	}
	s.log.Info("hybrid record removed", "id", id)
	return nil
}
