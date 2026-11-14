package codeview

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

// Open each segment relative to an already opened directory. O_NOFOLLOW and
// O_NONBLOCK prevent symlink races and blocking special-file opens on Unix.
func boundary(fd int) bool {
	var stat unix.Stat_t
	return unix.Fstatat(fd, ".git", &stat, unix.AT_SYMLINK_NOFOLLOW) == nil || (unix.Fstatat(fd, "HEAD", &stat, unix.AT_SYMLINK_NOFOLLOW) == nil && unix.Fstatat(fd, "objects", &stat, unix.AT_SYMLINK_NOFOLLOW) == nil)
}
func displayPath(p string) string {
	if utf8.ValidString(p) {
		return p
	}
	return strconv.QuoteToASCII(p)
}
func (b *binding) open(p string) (*os.File, error) {
	if _, err := nodePath(nodeID(p)); err != nil {
		return nil, err
	}
	full := filepath.Join(b.Root, p)
	for _, g := range b.gitDirs {
		if full == g || strings.HasPrefix(full, g+"/") {
			return nil, problemf(ErrInvalid, "Git 管理路径不可读取")
		}
	}
	fd, err := unix.Open(b.Root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), b.Root)
	stat, err := f.Stat()
	if err != nil || !os.SameFile(stat, b.identity) {
		_ = f.Close()
		return nil, problemf(ErrConflict, "浏览根已替换")
	}
	if p == "" {
		return f, nil
	}
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if i > 0 {
			if boundary(int(f.Fd())) {
				_ = f.Close()
				return nil, problemf(ErrInvalid, "嵌套仓库边界不可遍历")
			}
		}
		flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
		if i < len(parts)-1 {
			flags |= unix.O_DIRECTORY
		}
		next, e := unix.Openat(int(f.Fd()), part, flags, 0)
		_ = f.Close()
		if e != nil {
			if os.IsNotExist(e) {
				return nil, os.ErrNotExist
			}
			return nil, problemf(ErrInvalid, "路径不可安全读取: %v", e)
		}
		f = os.NewFile(uintptr(next), full)
	}
	stat, err = f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if stat.IsDir() && p != "" {
		if boundary(int(f.Fd())) {
			_ = f.Close()
			return nil, problemf(ErrInvalid, "嵌套仓库边界不可遍历")
		}
	}
	if !stat.IsDir() && !stat.Mode().IsRegular() {
		_ = f.Close()
		return nil, problemf(ErrInvalid, "不支持读取特殊文件或链接")
	}
	return f, nil
}
