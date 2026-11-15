package history

import "path/filepath"

func Root(home string, getenv func(string) string) string {
	if root := getenv("CLAUDE_CONFIG_DIR"); root != "" {
		return filepath.Join(root, "projects")
	}
	return filepath.Join(home, ".claude", "projects")
}
