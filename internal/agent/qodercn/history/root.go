package history

import "path/filepath"

func Root(home string, _ func(string) string) string {
	return filepath.Join(home, ".qoder-cn", "projects")
}
