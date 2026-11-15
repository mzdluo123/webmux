package conversation

import "github.com/rainchan/webmux/internal/codexrollout"

func (s *Service) DiscoverCodex(root string) ([]codexrollout.File, error) {
	if !s.HistoryEnabled() {
		return nil, ErrUnavailable
	}
	return codexrollout.Discover(root)
}
