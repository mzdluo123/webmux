package history

import "path/filepath"

func Root(home string, getenv func(string) string) string {
	if root := getenv("KIMI_CODE_HOME"); root != "" {
		return filepath.Join(root, "sessions")
	}
	return filepath.Join(home, ".kimi-code", "sessions")
}
