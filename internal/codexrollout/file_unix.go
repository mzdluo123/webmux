//go:build linux || darwin

package codexrollout

import (
	"os"
	"syscall"
)

// OpenFile never waits for a FIFO writer. Callers must still check the opened
// descriptor is a regular file before reading; O_NONBLOCK has no effect there.
func OpenFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
