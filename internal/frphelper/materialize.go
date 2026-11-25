// Package frphelper materializes the release's managed frpc on the local host.
package frphelper

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	binaryName  = "frpc"
	licenseName = "frpc.LICENSE"
	binaryMode  = 0o500
	licenseMode = 0o600
)

// ErrUnavailable means this build does not contain an authorized frpc.
var ErrUnavailable = errors.New("managed frpc unavailable")

//go:embed pins.txt
var pins string

var checkBundle = sync.OnceValue(func() error {
	return verifyBundle(bundledBinary, bundledLicense)
})

// Available reports whether this build contains the pinned frpc and license.
func Available() error {
	return checkBundle()
}

var platformPins = sync.OnceValues(func() (string, string) {
	var binaryPin, licensePin string
	for line := range strings.Lines(pins) {
		fields := strings.Fields(line)
		switch {
		case len(fields) == 4 && fields[0] == runtime.GOOS && fields[1] == runtime.GOARCH:
			binaryPin = fields[3]
		case len(fields) == 2 && fields[0] == "license":
			licensePin = fields[1]
		}
	}
	return binaryPin, licensePin
})

func verifyBundle(binary, license []byte) error {
	if len(binary) == 0 || len(license) == 0 {
		return ErrUnavailable
	}
	binaryPin, licensePin := platformPins()
	// The platform's digest identifies the exact upstream release and version.
	if !matchesPin(binary, binaryPin) || !matchesPin(license, licensePin) {
		return ErrUnavailable
	}
	return nil
}

func matchesPin(data []byte, pin string) bool {
	if len(pin) != sha256.Size*2 {
		return false
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) == pin
}

// Materialize writes the verified frpc and its license to a fresh private
// directory for one scope. The caller owns and removes the directory.
func Materialize(dir string) (string, error) {
	if err := Available(); err != nil {
		return "", err
	}
	root, err := openScopeDir(dir)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }() // The directory handle has no buffered writes.
	if err := writePrivate(root, licenseName, bundledLicense, licenseMode); err != nil {
		return "", fmt.Errorf("materialize managed frpc license: %w", err)
	}
	if err := writePrivate(root, binaryName, bundledBinary, binaryMode); err != nil {
		return "", fmt.Errorf("materialize managed frpc: %w", err)
	}
	return filepath.Join(dir, binaryName), nil
}

// Verify checks materialized files immediately before a managed frpc launch.
func Verify(executable string) error {
	if err := Available(); err != nil {
		return err
	}
	if filepath.Base(executable) != binaryName {
		return ErrUnavailable
	}
	root, err := openScopeDir(filepath.Dir(executable))
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	binaryPin, licensePin := platformPins()
	if err := verifyPrivate(root, binaryName, binaryMode, len(bundledBinary), binaryPin); err != nil {
		return fmt.Errorf("verify managed frpc: %w", err)
	}
	if err := verifyPrivate(root, licenseName, licenseMode, len(bundledLicense), licensePin); err != nil {
		return fmt.Errorf("verify managed frpc license: %w", err)
	}
	return nil
}

func openScopeDir(dir string) (*os.Root, error) {
	if !filepath.IsAbs(dir) {
		return nil, errors.New("managed frpc directory must be absolute")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open managed frpc directory: %w", err)
	}
	info, err := root.Stat(".")
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("stat managed frpc directory: %w", err)
	}
	if info.Mode().Perm() != 0o700 {
		_ = root.Close()
		return nil, errors.New("managed frpc directory must have mode 0700")
	}
	return root, nil
}

func verifyPrivate(root *os.Root, name string, mode os.FileMode, size int, pin string) error {
	entry, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if entry.Mode() != mode || entry.Size() != int64(size) {
		return ErrUnavailable
	}
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }() // Read-only verification.
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(entry, info) || info.Mode() != mode {
		return ErrUnavailable
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(file, int64(size)+1))
	if err != nil {
		return err
	}
	var digest [sha256.Size]byte
	if n != int64(size) || hex.EncodeToString(h.Sum(digest[:0])) != pin {
		return ErrUnavailable
	}
	return nil
}

func writePrivate(root *os.Root, name string, contents []byte, mode os.FileMode) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Chmod(mode); err != nil {
		return errors.Join(err, file.Close())
	}
	return file.Close()
}
