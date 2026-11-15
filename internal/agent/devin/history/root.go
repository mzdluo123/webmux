package history

import "path/filepath"

func Root(home string, getenv func(string) string) string {
	if root := getenv("XDG_DATA_HOME"); root != "" {
		return filepath.Join(root, "devin", "cli")
	}
	return filepath.Join(home, ".local", "share", "devin", "cli")
}
