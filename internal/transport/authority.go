package transport

import (
	"context"
	"net"
	"net/url"
	"strings"
)

func NormalizeAuthority(authority string) (string, bool) {
	parsed, err := url.Parse("http://" + authority)
	if err != nil || parsed.User != nil || parsed.Host != authority || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", false
	}
	port := parsed.Port()
	if port == "443" {
		port = ""
	}
	if port == "" {
		return host, true
	}
	return net.JoinHostPort(host, port), true
}
func AuthorityHostname(authority string) string {
	parsed, err := url.Parse("http://" + authority)
	if err != nil || parsed.User != nil || parsed.Host != authority || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return host
}

type localCallerKey struct{}

func LocalContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, localCallerKey{}, true)
}
func IsLocal(rctx context.Context) bool {
	local, _ := rctx.Value(localCallerKey{}).(bool)
	return local
}
