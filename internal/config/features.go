package config

import (
	"fmt"

	"github.com/rainchan/webmux/internal/builtin"
)

// FeatureConfig fixes the enabled business modules for one server lifetime.
// Nil preserves the fixed pre-split feature set; an explicit empty TOML array
// selects terminal core only. Validation never rewrites that distinction.
type FeatureConfig struct {
	Enabled []string `toml:"enabled"`
}

func (p FeatureConfig) Has(id string) bool {
	return builtin.Has(p.Enabled, id)
}

// Validate checks IDs and hard dependencies without I/O or default mutation.
func (p FeatureConfig) Validate() error {
	if _, err := builtin.Resolve(p.Enabled); err != nil {
		return fmt.Errorf("features.enabled: %w", err)
	}
	return nil
}
