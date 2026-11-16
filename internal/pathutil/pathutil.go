// Package pathutil holds path helpers shared by the server, its
// configuration and the local clients.
package pathutil

import (
	"encoding/binary"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SniffContentType names a media type from a file's first bytes, without
// parameters. ISO BMFF AVIF headers are matched before http.DetectContentType,
// which otherwise classifies them as application/octet-stream.
func SniffContentType(head []byte) string {
	if IsAVIF(head) {
		return "image/avif"
	}
	mediaType, _, err := mime.ParseMediaType(http.DetectContentType(head))
	if err != nil {
		return "application/octet-stream"
	}
	return mediaType
}

// IsAVIF checks the leading ISO BMFF ftyp box's major and compatible brands.
func IsAVIF(h []byte) bool {
	if len(h) < 16 || string(h[4:8]) != "ftyp" {
		return false
	}
	size := uint64(binary.BigEndian.Uint32(h[:4]))
	if size < 16 || size > uint64(len(h)) {
		size = uint64(len(h))
	}
	brand := func(b []byte) bool { return string(b) == "avif" || string(b) == "avis" }
	if brand(h[8:12]) {
		return true
	}
	for i := 16; i+4 <= int(size); i += 4 {
		if brand(h[i : i+4]) {
			return true
		}
	}
	return false
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
