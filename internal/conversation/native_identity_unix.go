//go:build linux || darwin

package conversation

import (
	"fmt"
	"os"
	"syscall"
)

func nativeFileIdentity(info os.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "unknown"
	}
	return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino)
}
