//go:build !linux && !darwin

package pathutil

import "errors"

// PublishDirectory fails closed on unsupported server platforms.
func PublishDirectory(from, to string) error {
	return errors.New("atomic directory publication is unsupported on this platform")
}
