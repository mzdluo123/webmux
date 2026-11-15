package conversation

import (
	"context"
	"github.com/rainchan/webmux/internal/codexrollout"
)

func (s *Service) DiscoverCodex(ctx context.Context, root string) (codexrollout.Discovery, error) {
	if !s.HistoryEnabled() {
		return codexrollout.Discovery{}, ErrUnavailable
	}
	return codexrollout.Discover(ctx, root)
}
