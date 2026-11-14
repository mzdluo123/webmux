package pathutil

import (
	"errors"
	"fmt"
	"os"
)

// ErrTemporaryDirectoryUnowned means the handle could not verify the root identity.
var ErrTemporaryDirectoryUnowned = errors.New("temporary directory ownership cannot be verified")

// TemporaryDirectory owns precisely one directory allocated by this handle.
// It cannot be constructed from a client-supplied or persisted deletion path.
type TemporaryDirectory struct {
	path     string
	identity os.FileInfo
}

func MakeTemporaryDirectory(parent, prefix string) (*TemporaryDirectory, error) {
	path, err := os.MkdirTemp(parent, prefix)
	if err != nil {
		return nil, err
	}
	identity, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	return &TemporaryDirectory{path: path, identity: identity}, nil
}
func (d *TemporaryDirectory) Path() string { return d.path }

// Remove refuses a replaced root, including a symlink to someone else's data.
func (d *TemporaryDirectory) Remove() error {
	current, err := os.Lstat(d.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTemporaryDirectoryUnowned, err)
	}
	if !os.SameFile(d.identity, current) {
		return fmt.Errorf("%w: root was replaced", ErrTemporaryDirectoryUnowned)
	}
	return os.RemoveAll(d.path)
}
