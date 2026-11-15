package transport

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/rainchan/webmux/internal/config"
)

// PublicOriginPolicy is the read-only runtime public-entry authorization policy.
type PublicOriginPolicy interface {
	HostAllowed(string) bool
	OriginAllowed(string, string) bool
	SourceOrigin(string) string
}

func OriginAllowed(cfg config.Config, public PublicOriginPolicy, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil || parsed.Host == "" || parsed.Path != "" || strings.ContainsAny(origin, "?#") || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if public != nil && public.HostAllowed(r.Host) {
		if public.OriginAllowed(origin, r.Host) {
			return true
		}
	} else if strings.EqualFold(parsed.Host, r.Host) && parsed.Scheme == scheme {
		return true
	}
	for _, allowed := range cfg.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}
