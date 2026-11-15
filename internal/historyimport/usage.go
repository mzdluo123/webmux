package historyimport

import (
	"context"

	"github.com/rainchan/webmux/internal/historysource"
)

// ProviderUsage is a conservative visible-byte estimate, not provider telemetry.
// Private reasoning and opaque image costs remain unknown to this estimate.
func (s *Service) ProviderUsage(ctx context.Context, conversation, provider string) (int64, error) {
	if !s.enabled() {
		return 0, historysource.ErrUnavailable
	}
	bindings, err := s.store.NativeBindings(ctx, conversation)
	if err != nil {
		return 0, err
	}
	for _, binding := range bindings {
		if binding.ProviderThreadID != provider {
			continue
		}
		source, err := s.resolve(ctx, binding)
		if err != nil {
			return 0, err
		}
		doc, err := s.read(ctx, historysource.Request{Source: source})
		if err != nil {
			return 0, err
		}
		var used int64
		for _, record := range doc.Records {
			used += int64(len(record.Text) + len(record.ID) + len(record.Role) + len(record.Kind) + 256)
			used += int64(len(record.Attachments)) * 8192
		}
		return used, nil
	}
	return 0, historysource.ErrUnavailable
}
