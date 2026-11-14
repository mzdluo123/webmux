package builtin

import (
	"slices"
	"strings"
	"testing"
)

func TestResolveSelectionBoundaries(t *testing.T) {
	legacy := []string{
		"launch", "workspace", "code", "files", "chat", "notifications",
		"quick-commands", "voice", "forward", "cloudflare", "tailscale",
		"p2p", "mcp", "immersive",
	}
	tests := []struct {
		name string
		give []string
		want []string
	}{
		{name: "legacy", give: nil, want: legacy},
		{name: "core", give: []string{}, want: []string{}},
		{name: "single plugin", give: []string{"code"}, want: []string{"code"}},
		{name: "dependencies", give: []string{"chat", "files", "launch"}, want: []string{"launch", "files", "chat"}},
		{name: "input order independent", give: []string{"files", "launch", "chat"}, want: []string{"launch", "files", "chat"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.give)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || !slices.Equal(got, tt.want) {
				t.Fatalf("Resolve(%v) = %v, want non-nil %v", tt.give, got, tt.want)
			}
			for _, definition := range Catalog() {
				if got, want := Has(tt.give, definition.ID), slices.Contains(tt.want, definition.ID); got != want {
					t.Errorf("Has(%q) = %v, want %v", definition.ID, got, want)
				}
			}
		})
	}
	if Has(nil, "unknown") || Has([]string{"unknown"}, "unknown") {
		t.Fatal("unknown IDs must never be treated as enabled")
	}
}

func TestResolveRejectsInvalidSelections(t *testing.T) {
	tests := []struct {
		name string
		give []string
		want []string
	}{
		{name: "unknown", give: []string{"other"}, want: []string{"unknown", "other"}},
		{name: "empty ID", give: []string{""}, want: []string{"unknown"}},
		{name: "duplicate", give: []string{"code", "code"}, want: []string{"duplicate", "code"}},
		{name: "missing launch", give: []string{"files", "chat"}, want: []string{"chat -> launch"}},
		{name: "missing files", give: []string{"launch", "chat"}, want: []string{"chat -> files"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.give)
			if err == nil || got != nil {
				t.Fatalf("invalid selection yielded %v, %v", got, err)
			}
			for _, detail := range tt.want {
				if !strings.Contains(err.Error(), detail) {
					t.Errorf("error %q does not identify %q", err, detail)
				}
			}
		})
	}
}

func TestResolveDetectsDependencyCycles(t *testing.T) {
	tests := []struct {
		name string
		give []Definition
		want string
	}{
		{name: "self", give: []Definition{{ID: "a", Requires: []string{"a"}}}, want: "a -> a"},
		{name: "indirect", give: []Definition{
			{ID: "a", Requires: []string{"b"}},
			{ID: "b", Requires: []string{"c"}},
			{ID: "c", Requires: []string{"a"}},
		}, want: "a -> b -> c -> a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled := make([]string, 0, len(tt.give))
			for _, definition := range tt.give {
				enabled = append(enabled, definition.ID)
			}
			got, err := resolve(tt.give, enabled)
			if err == nil || got != nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("cycle must fail with dependency chain %q, got %v, %v", tt.want, got, err)
			}
		})
	}
}

func TestResolveOrdersLaterDeclaredDependencies(t *testing.T) {
	catalog := []Definition{
		{ID: "consumer", Requires: []string{"dependency"}},
		{ID: "independent"},
		{ID: "dependency"},
	}
	got, err := resolve(catalog, []string{"independent", "consumer", "dependency"})
	want := []string{"dependency", "consumer", "independent"}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("dependency order = %v, %v; want %v", got, err, want)
	}
}
