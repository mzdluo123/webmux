// Package builtin defines the fixed, constructor-free built-in plugin catalog.
package builtin

import (
	"fmt"
	"slices"
	"strings"
)

// Definition declares a plugin's stable ID and hard dependencies.
type Definition struct {
	ID       string
	Requires []string
}

var definitions = []Definition{
	{ID: "launch"},
	{ID: "workspace"},
	{ID: "code"},
	{ID: "files"},
	{ID: "chat", Requires: []string{"launch", "files"}},
	{ID: "notifications"},
	{ID: "quick-commands"},
	{ID: "voice"},
	{ID: "forward"},
	{ID: "cloudflare"},
	{ID: "tailscale"},
	{ID: "p2p"},
	{ID: "mcp"},
	{ID: "immersive"},
}

// Keep this upgrade default fixed: future catalog additions must not silently
// enable new business behavior for configurations that omit plugins.enabled.
var legacyIDs = []string{
	"launch", "workspace", "code", "files", "chat", "notifications",
	"quick-commands", "voice", "forward", "cloudflare", "tailscale",
	"p2p", "mcp", "immersive",
}

// Catalog returns an independent copy in the stable catalog order.
func Catalog() []Definition {
	out := make([]Definition, len(definitions))
	for i, definition := range definitions {
		out[i] = Definition{ID: definition.ID, Requires: slices.Clone(definition.Requires)}
	}
	return out
}

// Has reports membership in the selected set. A nil set means the fixed legacy
// selection; a non-nil empty set means terminal core only. Unknown IDs are never
// enabled, including in legacy mode. Validate the selection with Resolve first.
func Has(enabled []string, id string) bool {
	if enabled == nil {
		return slices.Contains(legacyIDs, id)
	}
	for _, definition := range definitions {
		if definition.ID == id {
			return slices.Contains(enabled, id)
		}
	}
	return false
}

// Resolve validates selection without adding missing dependencies and returns
// dependencies before their consumers. Catalog order, not input order, breaks
// ties. The returned empty selection is non-nil.
func Resolve(enabled []string) ([]string, error) {
	if enabled == nil {
		enabled = legacyIDs
	}
	return resolve(definitions, enabled)
}

func resolve(catalog []Definition, enabled []string) ([]string, error) {
	byID := make(map[string]Definition, len(catalog))
	for _, definition := range catalog {
		if _, exists := byID[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate plugin definition %q", definition.ID)
		}
		byID[definition.ID] = definition
	}
	selected := make(map[string]bool, len(enabled))
	for _, id := range enabled {
		if _, exists := byID[id]; !exists {
			return nil, fmt.Errorf("unknown plugin %q", id)
		}
		if selected[id] {
			return nil, fmt.Errorf("duplicate plugin %q", id)
		}
		selected[id] = true
	}

	// Visiting is distinct from visited so a cycle fails rather than being
	// silently accepted as an already resolved dependency.
	const (
		visiting = 1
		visited  = 2
	)
	state := make(map[string]int, len(enabled))
	ordered := make([]string, 0, len(enabled))
	path := make([]string, 0, len(enabled))
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == visited {
			return nil
		}
		if state[id] == visiting {
			return fmt.Errorf("plugin dependency cycle: %s", strings.Join(append(path, id), " -> "))
		}
		state[id] = visiting
		path = append(path, id)
		for _, required := range byID[id].Requires {
			if !selected[required] {
				return fmt.Errorf("missing plugin dependency: %s", strings.Join(append(path, required), " -> "))
			}
			if err := visit(required); err != nil {
				return err
			}
		}
		path = path[:len(path)-1]
		state[id] = visited
		ordered = append(ordered, id)
		return nil
	}
	for _, definition := range catalog {
		if selected[definition.ID] {
			if err := visit(definition.ID); err != nil {
				return nil, err
			}
		}
	}
	return ordered, nil
}
