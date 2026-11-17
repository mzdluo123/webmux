//go:build darwin

package pathutil

import "golang.org/x/sys/unix"

// PublishDirectory atomically moves a sibling directory without replacement.
// Requires kernel and destination-volume support for exclusive renaming.
// Unsupported volumes return the original OS error; no unsafe fallback is used.
// An occupied destination returns EEXIST, distinct from unsupported-operation errors.
func PublishDirectory(from, to string) error { return unix.RenamexNp(from, to, unix.RENAME_EXCL) }
