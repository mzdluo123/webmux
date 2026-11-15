//go:build !linux && !darwin

package codexrollout

import "os"

func OpenFile(path string) (*os.File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrFormat
	}
	return os.Open(path)
}
