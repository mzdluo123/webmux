package historyimport

import (
	"context"
	"crypto/rand"
	"errors"

	"github.com/rainchan/webmux/internal/historysource"
)

// Import creates a history-only conversation from an exact frozen source range.
// It neither starts a provider nor copies source bodies into Activity.
func (s *Service) Import(ctx context.Context, source historysource.Source, retain bool) (historysource.Binding, historysource.Snapshot, error) {
	var binding historysource.Binding
	pendingOwner := "import-pending:" + rand.Text()
	snapshot, err := s.Freeze(ctx, source, pendingOwner, retain)
	if err != nil {
		return binding, historysource.Snapshot{}, err
	}
	binding, err = s.store.ImportFrozenSource(ctx, snapshot.ID, pendingOwner)
	if err != nil {
		// Cancellation must not strand the temporary lifetime reference. The
		// original context may already be cancelled when the transaction fails.
		cleanupErr := s.store.ReleaseFrozen(context.WithoutCancel(ctx), snapshot.ID, pendingOwner)
		return historysource.Binding{}, historysource.Snapshot{}, errors.Join(err, cleanupErr)
	}
	return binding, snapshot, nil
}
