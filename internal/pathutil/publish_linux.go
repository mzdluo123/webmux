//go:build linux

package pathutil

import "golang.org/x/sys/unix"

// PublishDirectory atomically moves a sibling directory without replacing any
// existing destination, even an empty directory or a dangling symlink.
func PublishDirectory(from, to string) error {
	return unix.Renameat2(unix.AT_FDCWD, from, unix.AT_FDCWD, to, unix.RENAME_NOREPLACE)
}
