//go:build darwin

package pathutil

import "golang.org/x/sys/unix"

// PublishDirectory atomically moves a sibling directory without replacement.
func PublishDirectory(from, to string) error { return unix.RenamexNp(from, to, unix.RENAME_EXCL) }
