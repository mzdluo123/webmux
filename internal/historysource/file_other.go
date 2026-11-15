//go:build !darwin && !linux

package historysource

import "os"

// Native readers run on the supported Linux/macOS server platforms. Keeping
// this fallback permits the separate Windows clients to cross-compile.
func fileIdentity(info os.FileInfo) string { return info.Name() }
