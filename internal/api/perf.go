package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/perf"
)

const (
	perfBodyMaxBytes  = 64 << 10
	perfMaxBatchItems = 100
)

var allowedFrontendPerfKinds = map[string]bool{
	"loaf":        true,
	"longtask":    true,
	"interaction": true,
	"cls":         true,
	"api":         true,
	"ws":          true,
	"memory":      true,
}

type perfResponseWriter struct {
	http.ResponseWriter
	start     time.Time
	timer     *perf.RequestTimer
	status    int
	bytes     int64
	wroteHead bool
	isUpgrade bool
}

func (w *perfResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *perfResponseWriter) writeTimingHeader() {
	if w.wroteHead || w.isUpgrade {
		return
	}
	w.wroteHead = true
	totalMS := float64(time.Since(w.start)) / float64(time.Millisecond)
	header := fmt.Sprintf("app;dur=%.2f", totalMS)
	if gitD := w.timer.GitDuration(); gitD > 0 {
		header += fmt.Sprintf(", git;dur=%.2f", float64(gitD)/float64(time.Millisecond))
	}
	w.Header().Set("Server-Timing", header)
}

func (w *perfResponseWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
		w.writeTimingHeader()
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *perfResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
		w.writeTimingHeader()
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += int64(n)
	return n, err
}

func (s *Server) wrapPerf(next http.Handler) http.Handler {
	if s.Perf == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip WebSocket handshakes and the perf ingestion route itself to avoid
		// hijacking/feedback loops.
		if strings.HasPrefix(r.URL.Path, "/ws/") || r.URL.Path == "/api/perf/events" {
			next.ServeHTTP(w, r)
			return
		}
		ctx, timer := perf.WithRequestTimer(r.Context())
		r = r.WithContext(ctx)
		pw := &perfResponseWriter{
			ResponseWriter: w,
			start:          time.Now(),
			timer:          timer,
			isUpgrade:      strings.EqualFold(r.Header.Get("Upgrade"), "websocket"),
		}
		next.ServeHTTP(pw, r)
		status := pw.status
		if status == 0 {
			status = http.StatusOK
		}
		op := perfRouteLabel(r)
		s.Perf.RecordHTTP(op, status, time.Since(pw.start), timer.GitDuration(), pw.bytes)
	})
}

func perfRouteLabel(r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}
	p := r.URL.Path
	switch {
	case strings.HasPrefix(p, "/assets/"):
		p = "/assets/*"
	case strings.HasPrefix(p, "/fonts/"):
		p = "/fonts/*"
	}
	return r.Method + " " + p
}

func (s *Server) perfRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/perf/events", s.recordPerfEvents)
}

func (s *Server) recordPerfEvents(w http.ResponseWriter, r *http.Request) {
	if s.Perf == nil {
		http.NotFound(w, r)
		return
	}
	var req contract.PerfEventBatch
	if !decodeStrict(w, r, &req, perfBodyMaxBytes, errors.New("invalid performance event batch")) {
		return
	}
	if len(req.Events) > perfMaxBatchItems {
		failure(w, http.StatusBadRequest, errors.New("too many performance events in one batch"))
		return
	}
	for _, ev := range req.Events {
		if !allowedFrontendPerfKinds[ev.Kind] || strings.TrimSpace(ev.Op) == "" {
			continue
		}
		s.Perf.RecordFrontend(perf.Event{
			TS:         ev.TS,
			Kind:       ev.Kind,
			Op:         ev.Op,
			DurationMS: ev.DurationMS,
			ServerMS:   ev.ServerMS,
			Value:      ev.Value,
			Bytes:      ev.Bytes,
			Status:     ev.Status,
			Route:      ev.Route,
			Detail:     ev.Detail,
		})
	}
	w.WriteHeader(http.StatusNoContent)
}

func mountLocalPprof(mux *http.ServeMux) {
	mux.HandleFunc("GET /local/v1/debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /local/v1/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /local/v1/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /local/v1/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /local/v1/debug/pprof/trace", pprof.Trace)
	mux.Handle("GET /local/v1/debug/pprof/{name}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pprof.Handler(r.PathValue("name")).ServeHTTP(w, r)
	}))
}
