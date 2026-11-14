// Package worktreefs performs descriptor-relative regular file writes and
// removals inside a verified Git worktree directory without following symlinks
// or entering Git control directories.
package worktreefs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"golang.org/x/sys/unix"
)

var (
	// ErrInvalid marks a malformed or forbidden worktree-relative path.
	ErrInvalid = errors.New("invalid worktree file path")
	// ErrSymlink marks a symlink or nested Git boundary encountered along the path.
	ErrSymlink = errors.New("worktree path traverses symlink or git boundary")
	// ErrStale marks an unexpected on-disk file state compared to the caller's snapshot.
	ErrStale = errors.New("worktree file changed on disk")
)

func cleanSegments(relPath string) ([]string, string, error) {
	if relPath == "" || strings.ContainsRune(relPath, 0) || strings.HasPrefix(relPath, "/") {
		return nil, "", fmt.Errorf("%w: empty or absolute path", ErrInvalid)
	}
	cleaned := path.Clean(relPath)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != relPath {
		return nil, "", fmt.Errorf("%w: non-canonical path", ErrInvalid)
	}
	segs := strings.Split(cleaned, "/")
	for _, seg := range segs {
		if seg == "" || seg == "." || seg == ".." || strings.EqualFold(seg, ".git") {
			return nil, "", fmt.Errorf("%w: forbidden segment", ErrInvalid)
		}
	}
	return segs[:len(segs)-1], segs[len(segs)-1], nil
}

func openParentDir(rootPath string, dirs []string, createMissing bool) (int, error) {
	fd, err := unix.Open(rootPath, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	for _, dir := range dirs {
		next, openErr := unix.Openat(fd, dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if openErr != nil && createMissing && errors.Is(openErr, unix.ENOENT) {
			if mkErr := unix.Mkdirat(fd, dir, 0o755); mkErr != nil && !errors.Is(mkErr, unix.EEXIST) {
				_ = unix.Close(fd)
				return -1, mkErr
			}
			next, openErr = unix.Openat(fd, dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		}
		_ = unix.Close(fd)
		if openErr != nil {
			if errors.Is(openErr, unix.ELOOP) || errors.Is(openErr, unix.ENOTDIR) {
				return -1, fmt.Errorf("%w: %v", ErrSymlink, openErr)
			}
			return -1, openErr
		}
		var dotGit unix.Stat_t
		if unix.Fstatat(next, ".git", &dotGit, unix.AT_SYMLINK_NOFOLLOW) == nil {
			_ = unix.Close(next)
			return -1, fmt.Errorf("%w: nested repository", ErrSymlink)
		}
		fd = next
	}
	return fd, nil
}

func readExistingDigestAt(dirfd int, name string, maxBytes int64) (exists bool, sum string, perm uint32, err error) {
	var st unix.Stat_t
	if sErr := unix.Fstatat(dirfd, name, &st, unix.AT_SYMLINK_NOFOLLOW); sErr != nil {
		if errors.Is(sErr, unix.ENOENT) {
			return false, "", 0o644, nil
		}
		return false, "", 0, sErr
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG {
		return true, "", 0, fmt.Errorf("%w: target is not a regular file", ErrSymlink)
	}
	if maxBytes > 0 && st.Size > maxBytes {
		return true, "", 0, fmt.Errorf("%w: target exceeds size limit", ErrStale)
	}
	rfd, oErr := unix.Openat(dirfd, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if oErr != nil {
		return true, "", 0, oErr
	}
	defer func() { _ = unix.Close(rfd) }()
	h := sha256.New()
	buf := make([]byte, 32*1024)
	for {
		n, rErr := unix.Read(rfd, buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if rErr != nil {
			return true, "", 0, rErr
		}
		if n == 0 {
			break
		}
	}
	mode := uint32(0o644)
	if st.Mode&0o111 != 0 {
		mode = 0o755
	}
	return true, hex.EncodeToString(h.Sum(nil)), mode, nil
}

func writeAllFD(fd int, data []byte) error {
	for len(data) > 0 {
		n, err := unix.Write(fd, data)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

// WriteRegular atomically writes content to relPath inside rootPath using an
// O_NOFOLLOW temporary sibling and Renameat, after verifying expectedSHA256
// (or non-existence when expectedSHA256 == "" and requireMatch is true).
func WriteRegular(rootPath, relPath string, content []byte, executable bool, requireMatch bool, expectedSHA256 string, maxBytes int64) (string, error) {
	dirs, base, err := cleanSegments(relPath)
	if err != nil {
		return "", err
	}
	dirfd, err := openParentDir(rootPath, dirs, true)
	if err != nil {
		return "", err
	}
	defer func() { _ = unix.Close(dirfd) }()

	exists, currentSum, _, err := readExistingDigestAt(dirfd, base, maxBytes)
	if err != nil {
		return "", err
	}
	if requireMatch {
		if expectedSHA256 == "" && exists {
			return "", fmt.Errorf("%w: file unexpectedly appeared on disk", ErrStale)
		}
		if expectedSHA256 != "" && (!exists || currentSum != expectedSHA256) {
			return "", fmt.Errorf("%w: file digest changed on disk", ErrStale)
		}
	}

	perm := uint32(0o644)
	if executable {
		perm = 0o755
	}

	var rnd [8]byte
	if _, rErr := rand.Read(rnd[:]); rErr != nil {
		return "", rErr
	}
	tmpName := ".webmux-merge-" + hex.EncodeToString(rnd[:])
	tfd, err := unix.Openat(dirfd, tmpName, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, perm)
	if err != nil {
		return "", err
	}
	cleanup := true
	defer func() {
		if tfd >= 0 {
			_ = unix.Close(tfd)
		}
		if cleanup {
			_ = unix.Unlinkat(dirfd, tmpName, 0)
		}
	}()

	if err := writeAllFD(tfd, content); err != nil {
		return "", err
	}
	if err := unix.Fchmod(tfd, perm); err != nil {
		return "", err
	}
	if err := unix.Fsync(tfd); err != nil {
		return "", err
	}
	if err := unix.Close(tfd); err != nil {
		tfd = -1
		return "", err
	}
	tfd = -1

	if err := unix.Renameat(dirfd, tmpName, dirfd, base); err != nil {
		return "", err
	}
	cleanup = false
	_ = unix.Fsync(dirfd)

	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}

// RemoveEntry unlinks a regular file or symlink at relPath inside rootPath
// without following symlinks; missing files succeed idempotently.
func RemoveEntry(rootPath, relPath string) error {
	dirs, base, err := cleanSegments(relPath)
	if err != nil {
		return err
	}
	dirfd, err := openParentDir(rootPath, dirs, false)
	if err != nil {
		if errors.Is(err, unix.ENOENT) || errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer func() { _ = unix.Close(dirfd) }()

	var st unix.Stat_t
	if sErr := unix.Fstatat(dirfd, base, &st, unix.AT_SYMLINK_NOFOLLOW); sErr != nil {
		if errors.Is(sErr, unix.ENOENT) {
			return nil
		}
		return sErr
	}
	kind := st.Mode & unix.S_IFMT
	if kind != unix.S_IFREG && kind != unix.S_IFLNK {
		return fmt.Errorf("%w: target is not a regular file or symlink", ErrSymlink)
	}
	if err := unix.Unlinkat(dirfd, base, 0); err != nil && !errors.Is(err, unix.ENOENT) {
		return err
	}
	return nil
}

// RemoveVerified unlinks a regular file or symlink at relPath inside rootPath
// ONLY if its on-disk content digest still matches expectedSHA256.
func RemoveVerified(rootPath, relPath, expectedSHA256 string, maxBytes int64) error {
	dirs, base, err := cleanSegments(relPath)
	if err != nil {
		return err
	}
	dirfd, err := openParentDir(rootPath, dirs, false)
	if err != nil {
		if errors.Is(err, unix.ENOENT) || errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: file already removed", ErrStale)
		}
		return err
	}
	defer func() { _ = unix.Close(dirfd) }()

	var st unix.Stat_t
	if sErr := unix.Fstatat(dirfd, base, &st, unix.AT_SYMLINK_NOFOLLOW); sErr != nil {
		if errors.Is(sErr, unix.ENOENT) {
			return fmt.Errorf("%w: file already removed", ErrStale)
		}
		return sErr
	}
	kind := st.Mode & unix.S_IFMT
	switch kind {
	case unix.S_IFREG:
		exists, sum, _, rErr := readExistingDigestAt(dirfd, base, maxBytes)
		if rErr != nil {
			return rErr
		}
		if !exists || (expectedSHA256 != "" && sum != expectedSHA256) {
			return fmt.Errorf("%w: file digest changed on disk", ErrStale)
		}
	case unix.S_IFLNK:
		buf := make([]byte, 4096)
		n, lErr := unix.Readlinkat(dirfd, base, buf)
		if lErr != nil {
			return lErr
		}
		digest := sha256.Sum256(buf[:n])
		if expectedSHA256 != "" && hex.EncodeToString(digest[:]) != expectedSHA256 {
			return fmt.Errorf("%w: symlink target changed on disk", ErrStale)
		}
	default:
		return fmt.Errorf("%w: target is not a regular file or symlink", ErrSymlink)
	}
	return unix.Unlinkat(dirfd, base, 0)
}
