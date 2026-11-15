package transport

import (
	"bytes"
	"context"
	"errors"
	"github.com/rainchan/webmux/internal/streamproto"
	"net/http"
	"net/url"
	"strings"
)

type Bridge struct {
	HTTPHandler http.Handler
	ServeStream streamproto.StreamHandler
	ServeRPC    streamproto.RPCHandler
}

type rpcResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	code   int
}

func (w *rpcResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *rpcResponseWriter) WriteHeader(statusCode int) {
	if w.code == 0 {
		w.code = statusCode
	}
}

func (w *rpcResponseWriter) Write(p []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.body.Write(p)
}

func ServeRPC(ctx context.Context, mux http.Handler, authorized func(*http.Request) bool, in streamproto.RPCRequest) streamproto.RPCResponse {
	rec := &rpcResponseWriter{header: make(http.Header)}
	finish := func() streamproto.RPCResponse {
		code := rec.code
		if code == 0 {
			code = http.StatusOK
		}
		headers := make(map[string]string, len(rec.header))
		for k, vals := range rec.header {
			if len(vals) > 0 {
				headers[strings.ToLower(k)] = vals[0]
			}
		}
		return streamproto.RPCResponse{
			Status:     code,
			StatusText: http.StatusText(code),
			Headers:    headers,
			Body:       rec.body.Bytes(),
		}
	}

	defer BaseHeaders(rec)()

	method := strings.ToUpper(strings.TrimSpace(in.Method))
	if method == "" {
		Failure(rec, http.StatusBadRequest, errors.New("missing method"))
		return finish()
	}
	parsed, err := url.ParseRequestURI(in.Path)
	if err != nil || !strings.HasPrefix(parsed.Path, "/api/") {
		http.NotFound(rec, &http.Request{Method: method, URL: &url.URL{Path: "/api/"}})
		return finish()
	}

	req, err := http.NewRequestWithContext(ctx, method, "http://127.0.0.1"+in.Path, bytes.NewReader(in.Body))
	if err != nil {
		Failure(rec, http.StatusBadRequest, errors.New("invalid request"))
		return finish()
	}
	req.ContentLength = int64(len(in.Body))
	for k, v := range in.Headers {
		if strings.EqualFold(k, "Origin") || strings.EqualFold(k, "Host") {
			continue
		}
		req.Header.Set(k, v)
	}
	if !authorized(req) {
		Failure(rec, http.StatusUnauthorized, errors.New("authentication required"))
		return finish()
	}
	mux.ServeHTTP(rec, req)
	return finish()
}
