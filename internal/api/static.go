package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"math"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"

	assets "github.com/rainchan/webmux/web"
)

// The inventory is fixed by the embedded build. Request paths and query strings
// never add entries, and each file's representations are initialized once.
var staticBuild, staticBuildErr = embeddedStaticFiles()

type staticFiles struct {
	root  fs.FS
	files map[string]*staticAsset
}

// Each representation is initialized at most once per embedded asset.
type staticAsset struct {
	once        sync.Once
	name        string
	contentType string
	compressed  bool
	data        []byte
	err         error
	identity    func() ([]byte, error)
	gzip        func() ([]byte, error)
}

func embeddedStaticFiles() (*staticFiles, error) {
	root, err := fs.Sub(assets.Assets, "dist")
	if err != nil {
		return nil, err
	}
	return newStaticFiles(root)
}

func newStaticFiles(root fs.FS) (*staticFiles, error) {
	files := &staticFiles{root: root, files: make(map[string]*staticAsset)}
	err := fs.WalkDir(root, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		base := strings.TrimSuffix(name, ".br")
		compressed := base != name && compressibleStaticType(staticContentType(base))
		if !compressed {
			base = name
		}
		files.files[base] = &staticAsset{name: name, contentType: staticContentType(base), compressed: compressed}
		return nil
	})
	return files, err
}

func (files *staticFiles) get(name string) (*staticAsset, error) {
	asset, ok := files.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	asset.once.Do(func() {
		asset.data, asset.err = fs.ReadFile(files.root, asset.name)
		if asset.compressed && asset.err == nil {
			reader := func() io.Reader { return brotli.NewReader(bytes.NewReader(asset.data)) }
			asset.identity = sync.OnceValues(func() ([]byte, error) { return io.ReadAll(reader()) })
			asset.gzip = sync.OnceValues(func() ([]byte, error) {
				var buf bytes.Buffer
				zw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
				if err != nil {
					return nil, err
				}
				if _, err := io.Copy(zw, reader()); err != nil {
					return nil, err
				}
				if err := zw.Close(); err != nil {
					return nil, err
				}
				return buf.Bytes(), nil
			})
		}
	})
	return asset, asset.err
}

func staticContentType(name string) string {
	switch path.Ext(name) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".webmanifest":
		return "application/manifest+json"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".woff2":
		return "font/woff2"
	case ".wasm":
		// WebAssembly.instantiateStreaming rejects any other type.
		return "application/wasm"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		return ""
	}
}

func compressibleStaticType(contentType string) bool {
	return strings.HasPrefix(contentType, "text/") || contentType == "application/manifest+json" ||
		contentType == "image/svg+xml" || contentType == "application/wasm"
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if staticBuildErr != nil {
		http.Error(w, "frontend build unavailable", http.StatusServiceUnavailable)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	if strings.HasPrefix(name, ".") || strings.Contains(name, "/.") {
		http.NotFound(w, r)
		return
	}
	asset, err := staticBuild.get(name)
	if err != nil {
		if strings.HasPrefix(name, "assets/") || path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		name = "index.html"
		asset, err = staticBuild.get(name)
	}
	if err != nil {
		http.Error(w, "run make web before building", http.StatusServiceUnavailable)
		return
	}
	if asset.contentType != "" {
		w.Header().Set("Content-Type", asset.contentType)
	}
	// Font filenames include a content hash, so they can be cached as immutable.
	if strings.HasPrefix(name, "assets/") || path.Ext(name) == ".woff2" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	asset.write(w, r)
}

func (asset *staticAsset) write(w http.ResponseWriter, r *http.Request) {
	q := staticEncodingQualities(strings.Join(r.Header.Values("Accept-Encoding"), ","))
	body := asset.data
	encoding := ""
	var err error
	if asset.compressed {
		w.Header().Set("Vary", "Accept-Encoding")
		switch {
		case q.br > 0 && q.br >= q.gzip && q.br >= q.identity:
			encoding = "br"
		case q.gzip > 0 && q.gzip >= q.identity:
			encoding = "gzip"
			body, err = asset.gzip()
		case q.identity > 0:
			body, err = asset.identity()
		}
	}
	if encoding == "" && q.identity <= 0 {
		w.Header().Del("Cache-Control")
		http.Error(w, "no acceptable content encoding", http.StatusNotAcceptable)
		return
	}
	if err != nil {
		w.Header().Del("Cache-Control")
		http.Error(w, "corrupt embedded asset", http.StatusInternalServerError)
		return
	}
	if encoding != "" {
		w.Header().Set("Content-Encoding", encoding)
	}
	_, _ = w.Write(body)
}

type encodingQualities struct{ br, gzip, identity float64 }

func staticEncodingQualities(header string) encodingQualities {
	result := encodingQualities{identity: 1}
	qualities := make(map[string]float64, 4)
	for _, item := range strings.Split(header, ",") {
		parts := strings.Split(item, ";")
		coding := strings.ToLower(strings.TrimSpace(parts[0]))
		if coding != "br" && coding != "gzip" && coding != "identity" && coding != "*" {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			key, value, ok := strings.Cut(parameter, "=")
			if strings.EqualFold(strings.TrimSpace(key), "q") {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
				if !ok || err != nil || math.IsNaN(parsed) || parsed < 0 || parsed > 1 {
					quality = 0
				} else {
					quality = parsed
				}
			}
		}
		qualities[coding] = quality
	}
	if wildcard, ok := qualities["*"]; ok {
		result.br, result.gzip = wildcard, wildcard
		if wildcard == 0 {
			result.identity = 0
		}
	}
	if quality, ok := qualities["br"]; ok {
		result.br = quality
	}
	if quality, ok := qualities["gzip"]; ok {
		result.gzip = quality
	}
	if quality, ok := qualities["identity"]; ok {
		result.identity = quality
	}
	return result
}
