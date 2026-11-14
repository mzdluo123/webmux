package transport

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/streamproto"
)

type streamConnectionKey struct{}

func RPCHandler(routes http.Handler, authorized func(*http.Request) bool) streamproto.RPCHandler {
	return func(ctx context.Context, request streamproto.RPCRequest) streamproto.RPCResponse {
		return ServeRPC(ctx, routes, authorized, request)
	}
}

func StreamRouter(streams []Stream) streamproto.StreamHandler {
	mux := http.NewServeMux()
	for _, stream := range streams {
		mux.HandleFunc(stream.Pattern, func(_ http.ResponseWriter, r *http.Request) {
			conn := r.Context().Value(streamConnectionKey{}).(hub.MessageConn)
			run, closeSource, err := stream.Open(r.Context(), r)
			if err != nil {
				code := hub.StatusNotFound
				if stream.CloseCode != nil {
					code = stream.CloseCode(err)
				}
				_ = conn.Close(code, "stream unavailable")
				return
			}
			if closeSource != nil {
				defer closeSource()
			}
			run(conn)
		})
	}
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		_ = r.Context().Value(streamConnectionKey{}).(hub.MessageConn).Close(hub.StatusNotFound, "stream not found")
	})
	return func(ctx context.Context, path string, raw streamproto.StreamConn) {
		conn := NewP2PStreamConn(raw)
		if conn == nil {
			return
		}
		parsed, err := url.ParseRequestURI(path)
		if err != nil || !strings.HasPrefix(parsed.Path, "/ws/") {
			_ = conn.Close(hub.StatusNotFound, "invalid stream path")
			return
		}
		req, err := http.NewRequestWithContext(context.WithValue(ctx, streamConnectionKey{}, conn), http.MethodGet, "http://127.0.0.1"+path, nil)
		if err != nil {
			_ = conn.Close(hub.StatusNotFound, "invalid stream path")
			return
		}
		mux.ServeHTTP(&streamResponse{header: make(http.Header), conn: conn}, req)
	}
}

type streamResponse struct {
	header http.Header
	conn   hub.MessageConn
}

func (w *streamResponse) Header() http.Header            { return w.header }
func (w *streamResponse) WriteHeader(int)                { _ = w.conn.Close(hub.StatusNotFound, "stream not found") }
func (w *streamResponse) Write(data []byte) (int, error) { return len(data), nil }
