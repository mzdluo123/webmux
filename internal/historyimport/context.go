package historyimport

import (
	"context"

	"github.com/rainchan/webmux/internal/historysource"
)

// ProviderContext observes only the exact runtime-bound native session. It does
// not persist a body, create a snapshot, or substitute another provider's state.
func (s *Service) ProviderContext(ctx context.Context, conversation, provider string) (historysource.ContextState, error) {
	var empty historysource.ContextState
	if !s.enabled() {
		return empty, historysource.ErrUnavailable
	}
	bindings, err := s.store.NativeBindings(ctx, conversation)
	if err != nil {
		return empty, err
	}
	for _, binding := range bindings {
		if binding.ProviderThreadID != provider {
			continue
		}
		source, err := s.resolve(ctx, binding)
		if err != nil {
			return empty, err
		}
		doc, err := s.read(ctx, historysource.Request{Source: source})
		if err != nil {
			return empty, err
		}
		snapshot := historysource.Snapshot{Source: doc.Source, Generation: doc.Generation}
		for index, record := range doc.Records {
			if record.Kind != "compaction" && index != len(doc.Records)-1 {
				continue
			}
			hash, err := hashValue(record)
			if err != nil {
				return empty, err
			}
			snapshot.Records = append(snapshot.Records, historysource.RecordRef{ID: record.ID, Kind: record.Kind, Hash: hash})
		}
		return snapshot.Context(), nil
	}
	return empty, historysource.ErrUnavailable
}

// RecordDispatchCheckpoint runs after provider acknowledgement. A compaction
// during the call cannot prove that the delivered ranges remain in context.
func (s *Service) RecordDispatchCheckpoint(ctx context.Context, execution string) error {
	scope, err := s.store.DispatchSourceScope(ctx, execution)
	if err != nil {
		return err
	}
	state, err := s.ProviderContext(ctx, scope.ConversationID, scope.ProviderThreadID)
	if err != nil {
		return err
	}
	if state.NativeID != scope.NativeID || state.Key == "" || state.HeadID == "" || (scope.BeforeContextKey == "" && state.Compacted) || (scope.BeforeContextKey != "" && scope.BeforeContextKey != state.Key) {
		return historysource.ErrChanged
	}
	return s.store.RecordSourceContext(ctx, scope, state)
}
