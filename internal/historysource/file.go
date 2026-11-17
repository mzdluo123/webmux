package historysource

import "os"

// FileIdentity distinguishes replaced native stores without depending on paths
// or modification times. A rename preserves identity; a replacement does not.
func FileIdentity(info os.FileInfo) string { return fileIdentity(info) }
