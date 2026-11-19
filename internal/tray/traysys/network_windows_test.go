//go:build windows

package traysys

import (
	"path/filepath"
	"testing"
)

func TestInspectFirewallAndAdapterCategoriesWindows(t *testing.T) {
	st, err := InspectFirewall(filepath.Join(t.TempDir(), "webmux-tray.exe"))
	if err != nil {
		t.Fatalf("InspectFirewall: %v", err)
	}
	if st.RulePresent || st.RuleValid {
		t.Fatalf("non-existent temp executable unexpectedly matched a rule: %+v", st)
	}

	cats := AdapterCategories()
	for name, cat := range cats {
		switch cat {
		case "public", "private", "domain":
		default:
			t.Errorf("adapter %q reported unexpected category %q", name, cat)
		}
	}
}
