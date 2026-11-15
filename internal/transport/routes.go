package transport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rainchan/webmux/internal/hub"
)

// Exposure is the listener policy enforced before a handler is mounted.
type Exposure uint8

const (
	TCP Exposure = 1 << iota
	Local
	P2P
	All = TCP | Local | P2P
)

type Route struct {
	Pattern  string
	Handler  http.Handler
	Exposure Exposure
	// Shell wraps the host-owned static shell for a contributed page. Exactly
	// one of Handler and Shell must be set; listener exposure applies to both.
	Shell func(http.Handler) http.Handler
}

// Stream opens its source before HTTP upgrade. TCP and P2P run the same source
// and writer; Close releases a source when HTTP upgrade fails as well.
type Stream struct {
	Pattern   string
	Exposure  Exposure
	Open      func(context.Context, *http.Request) (run func(hub.MessageConn), close func(), err error)
	Failure   func(http.ResponseWriter, error)
	CloseCode func(error) hub.StatusCode
}

type Contribution struct {
	Owner   string
	Routes  []Route
	Streams []Stream
}

// Registrar collects prebound routes without constructing a second live mux.
type Registrar interface {
	Handle(string, http.Handler)
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
}

type Collector struct {
	Routes   []Route
	Exposure Exposure
}

func (c *Collector) Handle(pattern string, handler http.Handler) {
	exposure := c.Exposure
	if exposure == 0 {
		exposure = All
	}
	c.Routes = append(c.Routes, Route{Pattern: pattern, Handler: handler, Exposure: exposure})
}
func (c *Collector) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	c.Handle(pattern, http.HandlerFunc(handler))
}

func StreamHTTP(stream Stream) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		run, closeSource, err := stream.Open(r.Context(), r)
		if err != nil {
			if stream.Failure != nil {
				stream.Failure(w, err)
			} else {
				Failure(w, http.StatusServiceUnavailable, err)
			}
			return
		}
		if closeSource != nil {
			defer closeSource()
		}
		if conn := Accept(w, r); conn != nil {
			run(conn)
		}
	})
}

// Mount validates every contribution against all core routes before returning
// any listener. ServeMux detects equivalent patterns and ambiguous overlaps,
// including overlaps that would otherwise be hidden by listener filtering.
func Mount(core func(Registrar), contributions []Contribution, exposure Exposure, shell http.Handler) (_ *http.ServeMux, err error) {
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("route contribution conflict: %v", value)
		}
	}()
	validation := http.NewServeMux()
	core(validation)
	for _, contribution := range contributions {
		if contribution.Owner == "" {
			return nil, errors.New("route contribution requires an owner")
		}
		for _, route := range contribution.Routes {
			if err := validatePattern(route.Pattern, contribution.Owner); err != nil {
				return nil, err
			}
			if (route.Handler == nil) == (route.Shell == nil) || !validExposure(contribution.Owner, route.Pattern, route.Exposure) {
				return nil, errors.New("invalid route contribution")
			}
			handler := route.Handler
			if route.Shell != nil {
				if shell == nil {
					return nil, errors.New("shell route requires the host shell")
				}
				handler = shell
			}
			validation.Handle(route.Pattern, handler)
		}
		for _, stream := range contribution.Streams {
			if err := validatePattern(stream.Pattern, contribution.Owner); err != nil {
				return nil, err
			}
			if stream.Open == nil || !validExposure(contribution.Owner, stream.Pattern, stream.Exposure) || !strings.HasPrefix(stream.Pattern, "GET /ws/") {
				return nil, errors.New("invalid stream contribution")
			}
			validation.Handle(stream.Pattern, StreamHTTP(stream))
		}
	}
	mux := http.NewServeMux()
	core(mux)
	for _, contribution := range contributions {
		for _, route := range contribution.Routes {
			if route.Exposure&exposure != 0 {
				handler := route.Handler
				if route.Shell != nil {
					handler = route.Shell(shell)
				}
				mux.Handle(route.Pattern, handler)
			}
		}
		for _, stream := range contribution.Streams {
			if stream.Exposure&exposure != 0 {
				mux.Handle(stream.Pattern, StreamHTTP(stream))
			}
		}
	}
	return mux, nil
}

func validatePattern(pattern, owner string) error {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		path = pattern
		method = http.MethodGet
	}
	if strings.TrimSpace(path) != path || !strings.HasPrefix(path, "/") {
		return fmt.Errorf("invalid route pattern %q", pattern)
	}
	if owner == "p2p" && path == "/local/v1/p2p" {
		return nil
	}
	// Optional routes may not shadow the core shell or discovery/security routes.
	if path == "/" || path == "/api/" || path == "/ws/" || path == "/api/health" || path == "/api/config" || path == "/api/capabilities" || path == "/api/settings" || path == "/ws/settings" || path == "/api/sessions" || path == "/api/sessions/{id}" || path == "/api/sessions/{id}/input" || path == "/api/sessions/{id}/interrupt" || path == "/api/sessions/{id}/resize" || strings.HasPrefix(path, "/local/") {
		return fmt.Errorf("plugin %s uses reserved route %s", owner, pattern)
	}
	switch {
	case strings.HasPrefix(path, "/api/"), strings.HasPrefix(path, "/ws/"):
	case owner == "mcp" && path == "/mcp":
	case owner == "forward" && (path == "/forward" || strings.HasPrefix(path, "/forward/") || path == "/forward-auth"):
	default:
		return fmt.Errorf("plugin %s uses invalid route %s", owner, pattern)
	}
	// A more general wildcard may legally coexist in ServeMux but still capture
	// core operations. Probe concrete reserved paths before accepting it.
	probe := http.NewServeMux()
	probe.Handle(pattern, http.NotFoundHandler())
	for _, reserved := range []string{"/api/health", "/api/config", "/api/capabilities", "/api/settings", "/api/sessions", "/api/sessions/reserved-session", "/api/sessions/reserved-session/input", "/api/sessions/reserved-session/interrupt", "/api/sessions/reserved-session/resize", "/api/sessions/reserved-session/buffer", "/api/sessions/reserved-session/output", "/api/sessions/reserved-session/details", "/api/sessions/reserved-session/labels", "/api/sessions/reserved-session/attention", "/ws/settings", "/ws/sessions", "/ws/s/reserved-session"} {
		if _, matched := probe.Handler(&http.Request{Method: method, URL: &url.URL{Path: reserved}}); matched != "" {
			return fmt.Errorf("plugin %s overlaps reserved route %s", owner, pattern)
		}
	}
	return nil
}

func validExposure(owner, pattern string, exposure Exposure) bool {
	if exposure == 0 || exposure & ^All != 0 {
		return false
	}
	_, path, hasMethod := strings.Cut(pattern, " ")
	if !hasMethod {
		path = pattern
	}
	if owner == "mcp" || path == "/forward" || strings.HasPrefix(path, "/forward/") || path == "/forward-auth" {
		return exposure == TCP
	}
	if strings.HasPrefix(path, "/local/") {
		return exposure == Local
	}
	return true
}
