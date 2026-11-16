package hybrid

import (
	"context"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

func (s *Service) runtimeRoot(agent, cwd string, env map[string]string) string {
	if s.nativeRoot == nil {
		return ""
	}
	root := s.nativeRoot(agent, env)
	if root == "" {
		return ""
	}
	if !filepath.IsAbs(root) {
		root = filepath.Join(cwd, root)
	}
	return filepath.Clean(root)
}

// NativeIdentity binds prompt admission to the selected runtime and its options.
func NativeIdentity(r Record) string {
	var b strings.Builder
	for _, value := range []string{r.ID, r.Agent, r.AgentSessionID, r.HistoryRoot, r.Cwd, r.ConfigID, string(r.Side), string(r.State)} {
		b.WriteString(value)
		b.WriteByte(0)
	}
	for _, arg := range r.Cmd {
		b.WriteString(arg)
		b.WriteByte(0)
	}
	for _, key := range slices.Sorted(maps.Keys(r.Config)) {
		b.WriteString(key)
		b.WriteByte(0)
		b.WriteString(r.Config[key])
		b.WriteByte(0)
	}
	return b.String()
}

// ContextUsage returns a copy of the current provider-reported window usage.
func (s *Service) ContextUsage(ctx context.Context, id string) (*Usage, error) {
	a, err := s.actor(id)
	if err != nil {
		return nil, err
	}
	return call(ctx, a, func() (*Usage, error) {
		if a.chat.Usage == nil {
			return nil, nil
		}
		usage := *a.chat.Usage
		return &usage, nil
	})
}
