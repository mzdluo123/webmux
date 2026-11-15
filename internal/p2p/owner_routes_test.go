package p2p

import "testing"

func TestOwnerInteractiveAllowlist(t *testing.T) {
	for _, path := range []string{
		"/ws/sessions", "/ws/s/abc?client=c1&generation=g1&offset=123",
		"/ws/s/abc/chat?cursor=c1",
	} {
		if !ownerStreamAllowed(path) {
			t.Errorf("interactive stream rejected: %q", path)
		}
	}
	for _, path := range []string{
		"/ws/settings", "/ws/voice", "/ws/s/abc/voice", "/forward/app",
		"/ws/s/abc/chat/extra", "/ws/s/abc/../settings", "/ws/s/%61bc",
		"https://example.com/ws/sessions", "//example.com/ws/sessions", "/ws/s/",
	} {
		if ownerStreamAllowed(path) {
			t.Errorf("unclassified stream admitted: %q", path)
		}
	}
	for _, request := range [][2]string{
		{"GET", "/api/p2p/ping?seq=1"}, {"GET", "/api/sessions?state=all"},
		{"GET", "/api/sessions/abc"}, {"GET", "/api/sessions/abc/buffer"},
		{"GET", "/api/sessions/abc/output?offset=5"}, {"GET", "/api/chat/search?q=term"},
		{"POST", "/api/sessions/abc/input"}, {"POST", "/api/sessions/abc/interrupt"},
		{"POST", "/api/sessions/abc/resize"}, {"PATCH", "/api/sessions/abc/chat/queue/q1"},
		{"DELETE", "/api/sessions/abc/chat/queue/q1"}, {"POST", "/api/sessions/abc/chat/queue/q1/send-now"},
		{"POST", "/api/sessions/abc/chat/queue/resume"}, {"POST", "/api/sessions/abc/chat/queue/clear"},
		{"POST", "/api/sessions/abc/chat/requests/r1"},
	} {
		if !ownerRPCAllowed(request[0], request[1]) {
			t.Errorf("interactive RPC rejected: %v", request)
		}
	}
	for _, request := range [][2]string{
		{"POST", "/api/sessions"}, {"DELETE", "/api/sessions/abc"},
		{"GET", "/api/settings"}, {"PUT", "/api/settings"}, {"GET", "/api/relay"},
		{"POST", "/api/relay/ice"}, {"POST", "/api/uploads"}, {"GET", "/api/sessions/abc/upload"},
		{"GET", "/api/fs/dirs"}, {"GET", "/forward/app"}, {"POST", "/api/voice"},
		{"PUT", "/api/sessions/abc/chat/config/c1"}, {"PUT", "/api/sessions/abc/chat/permission-mode"},
		{"GET", "/api/sessions/%61bc"}, {"GET", "/api/sessions/../settings"},
		{"GET", "https://example.com/api/sessions"}, {"GET", "/api/unknown"},
		{"POST", "/api/p2p/ping"}, {"get", "/api/sessions"},
	} {
		if ownerRPCAllowed(request[0], request[1]) {
			t.Errorf("unclassified RPC admitted: %v", request)
		}
	}
}
