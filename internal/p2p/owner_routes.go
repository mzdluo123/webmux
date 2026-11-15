package p2p

import (
	"net/url"
	"strings"
)

func ownerRoute(raw string) ([]string, bool) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.Fragment != "" ||
		parsed.EscapedPath() != parsed.Path || !strings.HasPrefix(parsed.Path, "/") {
		return nil, false
	}
	parts := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, false
		}
		for i := range len(part) {
			c := part[i]
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' {
				return nil, false
			}
		}
	}
	return parts, true
}

// Owner channels preserve the existing terminal generation/offset and chat/list
// snapshot protocols. Unclassified operations never become a generic proxy.
func ownerStreamAllowed(path string) bool {
	parts, ok := ownerRoute(path)
	if !ok || parts[0] != "ws" {
		return false
	}
	return len(parts) == 2 && parts[1] == "sessions" ||
		len(parts) == 3 && parts[1] == "s" ||
		len(parts) == 4 && parts[1] == "s" && parts[3] == "chat"
}

func ownerRPCAllowed(method, path string) bool {
	parts, ok := ownerRoute(path)
	if !ok || parts[0] != "api" {
		return false
	}
	if method == "GET" {
		if len(parts) == 3 && (parts[1] == "p2p" && parts[2] == "ping" || parts[1] == "chat" && parts[2] == "search") {
			return true
		}
		if len(parts) == 2 && parts[1] == "sessions" || len(parts) == 3 && parts[1] == "sessions" {
			return true
		}
		return len(parts) == 4 && parts[1] == "sessions" && (parts[3] == "buffer" || parts[3] == "output")
	}
	if len(parts) < 4 || parts[1] != "sessions" {
		return false
	}
	if method == "POST" && len(parts) == 4 {
		return parts[3] == "input" || parts[3] == "interrupt" || parts[3] == "resize"
	}
	if parts[3] != "chat" {
		return false
	}
	if len(parts) == 6 && parts[4] == "queue" {
		return method == "PATCH" || method == "DELETE" ||
			method == "POST" && (parts[5] == "resume" || parts[5] == "clear")
	}
	if method == "POST" {
		return len(parts) == 7 && parts[4] == "queue" && parts[6] == "send-now" ||
			len(parts) == 6 && parts[4] == "requests"
	}
	return false
}
