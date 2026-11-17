package history

import "path/filepath"

// Root is provisional, like the explicitly shared QoderCN reader. An explicit
// verified source binding remains available when an installation differs.
func Root(home string, _ func(string) string) string {
	return filepath.Join(home, ".qoder", "projects")
}
