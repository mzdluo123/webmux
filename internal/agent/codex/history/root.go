package history

import "path/filepath"

func Root(home string, getenv func(string) string) string {
	if root := getenv("CODEX_HOME"); root != "" {
		return root
	}
	return filepath.Join(home, ".codex")
}
