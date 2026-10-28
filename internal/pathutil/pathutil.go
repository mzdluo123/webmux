// Package pathutil holds path helpers shared by the server, its
// configuration and the local clients.
package pathutil

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// UploadDirPrefix names the private temporary directories the upload
// endpoint stores files in.
const UploadDirPrefix = "webmux-upload-"

// UploadPath validates that path names a regular, non-symlink file directly
// inside a private upload directory under the system temporary directory,
// and returns its canonical path. A path that is not an upload cannot be
// read as an attachment.
func UploadPath(path string) (string, error) {
	invalid := errors.New("路径不是有效的上传文件")
	if path == "" || !filepath.IsAbs(path) {
		return "", invalid
	}
	clean := filepath.Clean(path)
	tmp := filepath.Clean(os.TempDir())
	dir := filepath.Dir(clean)
	if filepath.Dir(dir) != tmp || !strings.HasPrefix(filepath.Base(dir), UploadDirPrefix) {
		return "", invalid
	}
	info, err := os.Lstat(clean)
	if err != nil || !info.Mode().IsRegular() {
		return "", invalid
	}
	dirInfo, err := os.Lstat(dir)
	if err != nil || !dirInfo.IsDir() || dirInfo.Mode().Perm() != 0o700 {
		return "", invalid
	}
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return "", invalid
	}
	wantDir, err := filepath.EvalSymlinks(dir)
	if err != nil || filepath.Dir(resolved) != wantDir {
		return "", invalid
	}
	return resolved, nil
}

// SniffContentType names a media type from a file's first bytes, without
// parameters.
func SniffContentType(head []byte) string {
	mediaType, _, err := mime.ParseMediaType(http.DetectContentType(head))
	if err != nil {
		return "application/octet-stream"
	}
	return mediaType
}

// Expand expands a leading "~/" and makes the path absolute.
func Expand(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	return filepath.Abs(path)
}
