package launch

import (
	"maps"
	"slices"
	"strings"
	"time"
)

// Preset describes a built-in launch config template that can be restored.
type Preset struct {
	Name     string   `json:"name"`
	Icon     string   `json:"icon,omitempty"`
	Cmd      []string `json:"cmd"`
	EnvNames []string `json:"envNames"`
	Hybrid   bool     `json:"hybrid"`
}

func (in Input) preset() Preset {
	names := slices.Sorted(maps.Keys(in.Env))
	if names == nil {
		names = []string{}
	}
	return Preset{
		Name:     in.Name,
		Icon:     in.Icon,
		Cmd:      slices.Clone(in.Cmd),
		EnvNames: names,
		Hybrid:   in.Hybrid,
	}
}

func (s *Service) normalizePresets(ins []Input) ([]Input, error) {
	out := make([]Input, 0, len(ins))
	for _, in := range ins {
		in, err := s.normalize(in)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(out, func(p Input) bool { return p.Name == in.Name }) {
			return nil, ErrDuplicate
		}
		out = append(out, in)
	}
	return out, nil
}

// Presets returns the system launch presets configured on the service.
func (s *Service) Presets() ([]Preset, error) {
	if err := s.lockAvailable(); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	out := make([]Preset, 0, len(s.presets))
	for _, p := range s.presets {
		out = append(out, p.preset())
	}
	return out, nil
}

// RestorePresets adds missing system presets without overwriting existing
// configs of the same name. When name is non-empty, only that preset is
// restored (returning ErrNotFound for an unknown preset name or ErrDuplicate
// if a config with that name already exists). When name is empty, every
// missing preset is appended in preset order.
func (s *Service) RestorePresets(name string) ([]Summary, error) {
	name = strings.TrimSpace(name)
	if err := s.lockAvailable(); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	presets := s.presets
	var toAdd []Input
	if name != "" {
		i := slices.IndexFunc(presets, func(p Input) bool { return p.Name == name })
		if i < 0 {
			return nil, ErrNotFound
		}
		if s.nameTakenLocked(name, "") {
			return nil, ErrDuplicate
		}
		toAdd = []Input{presets[i]}
	} else {
		for _, p := range presets {
			if !s.nameTakenLocked(p.Name, "") {
				toAdd = append(toAdd, p)
			}
		}
	}
	if len(toAdd) == 0 {
		return s.summariesLocked(), nil
	}
	if len(s.configs)+len(toAdd) > MaxConfigs {
		return nil, invalidf("最多保存 %d 条启动配置", MaxConfigs)
	}
	now := time.Now().UTC()
	next := slices.Clone(s.configs)
	for _, in := range toAdd {
		id, err := newID(func(id string) bool {
			return slices.ContainsFunc(next, func(c Config) bool { return c.ID == id })
		})
		if err != nil {
			return nil, err
		}
		c := Config{ID: id, Name: in.Name, Icon: in.Icon, Cmd: in.Cmd, Env: in.Env, Preview: in.Preview, Attention: in.Attention, Hybrid: in.Hybrid, CreatedAt: now, UpdatedAt: now}
		next = append(next, c.clone())
	}
	if err := s.saveLocked(next); err != nil {
		return nil, err
	}
	s.configs = next
	return s.summariesLocked(), nil
}
