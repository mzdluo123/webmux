//go:build !linux && !darwin

package conversation

import "os"

func nativeFileIdentity(info os.FileInfo) string { return "unknown" }
