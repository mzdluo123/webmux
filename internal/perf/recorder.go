// Package perf records opt-in local performance bottlenecks (slow HTTP/Git
// operations, periodic CPU/memory/session snapshots, frontend LoAF/interaction
// events, and optional Go FlightRecorder spike traces) as bounded daily JSONL.
// It never records terminal input/output, prompts, or credentials.
package perf

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SessionRef identifies one running session process tree for periodic CPU and
// RSS attribution without coupling perf to internal/session.
type SessionRef struct {
	ID     string
	Name   string
	PID    int
	Status string
}

// Options configures a Recorder.
type Options struct {
	Dir            string
	SlowMS         int
	SampleSeconds  int
	MaxBytesMB     int
	RetentionDays  int
	FlightRecorder bool
	SpikeMS        int
	Sessions       func() []SessionRef
}

// Event is one line in perf-YYYY-MM-DD.jsonl.
type Event struct {
	TS         string         `json:"ts"`
	Source     string         `json:"source"`
	Kind       string         `json:"kind"`
	Op         string         `json:"op"`
	DurationMS float64        `json:"durationMs,omitempty"`
	ServerMS   float64        `json:"serverMs,omitempty"`
	GitMS      float64        `json:"gitMs,omitempty"`
	Value      float64        `json:"value,omitempty"`
	Bytes      int64          `json:"bytes,omitempty"`
	Status     int            `json:"status,omitempty"`
	Route      string         `json:"route,omitempty"`
	Detail     string         `json:"detail,omitempty"`
	Metrics    *SampleMetrics `json:"metrics,omitempty"`
	TopOps     []OpWindow     `json:"topOps,omitempty"`
	TopProcs   []ProcSample   `json:"topProcs,omitempty"`
}

// SampleMetrics summarizes Go runtime and host resource pressure at one tick.
type SampleMetrics struct {
	SelfCPUPct   float64 `json:"selfCpuPct,omitempty"`
	SelfRSSMB    float64 `json:"selfRssMb,omitempty"`
	HeapInuseMB  float64 `json:"heapInuseMb"`
	HeapAllocMB  float64 `json:"heapAllocMb"`
	AllocRateMBs float64 `json:"allocRateMbs,omitempty"`
	Goroutines   int     `json:"goroutines"`
	GCDelta      uint32  `json:"gcDelta,omitempty"`
	MaxGCPauseMS float64 `json:"maxGcPauseMs,omitempty"`
	HostLoad1    float64 `json:"hostLoad1,omitempty"`
	HostAvailMB  float64 `json:"hostAvailMb,omitempty"`
	Sessions     int     `json:"sessions,omitempty"`
	Dropped      int64   `json:"dropped,omitempty"`
}

// OpWindow aggregates all invocations of one route or operation during a tick.
type OpWindow struct {
	Op         string  `json:"op"`
	Count      int     `json:"count"`
	TotalMS    float64 `json:"totalMs"`
	MaxMS      float64 `json:"maxMs"`
	ErrorCount int     `json:"errors,omitempty"`
	Bytes      int64   `json:"bytes,omitempty"`
}

// ProcSample attributes CPU% and RSS MB to one session process tree.
type ProcSample struct {
	SessionID string  `json:"sessionId,omitempty"`
	Name      string  `json:"name"`
	PID       int     `json:"pid"`
	Procs     int     `json:"procs,omitempty"`
	CPUPct    float64 `json:"cpuPct,omitempty"`
	RSSMB     float64 `json:"rssMb"`
}

type requestTimerKey struct{}

// RequestTimer accumulates nested backend sub-operation durations (such as Git
// invocations) for Server-Timing headers and slow-request attribution.
type RequestTimer struct {
	gitNS atomic.Int64
}

// WithRequestTimer attaches a RequestTimer to ctx.
func WithRequestTimer(ctx context.Context) (context.Context, *RequestTimer) {
	t := &RequestTimer{}
	return context.WithValue(ctx, requestTimerKey{}, t), t
}

// AddGit adds one Git subprocess duration to the request timer in ctx, if any.
func AddGit(ctx context.Context, d time.Duration) {
	if ctx == nil {
		return
	}
	if t, ok := ctx.Value(requestTimerKey{}).(*RequestTimer); ok && t != nil {
		t.gitNS.Add(int64(d))
	}
}

// GitDuration returns the accumulated Git time for this request.
func (t *RequestTimer) GitDuration() time.Duration {
	if t == nil {
		return 0
	}
	return time.Duration(t.gitNS.Load())
}

var defaultRecorder atomic.Pointer[Recorder]

// SetDefault sets the process-wide default recorder used by instrumentation
// hooks such as Git subprocess timing. Pass nil on shutdown.
func SetDefault(r *Recorder) { defaultRecorder.Store(r) }

// Default returns the active default recorder, or nil when profiling is off.
func Default() *Recorder { return defaultRecorder.Load() }

// RecordGit records one Git command duration on the request timer and default
// recorder (if active). subcmd must contain only safe command names, never
// user paths, arguments, or commit text.
func RecordGit(ctx context.Context, subcmd string, d time.Duration, failed bool) {
	AddGit(ctx, d)
	if r := Default(); r != nil {
		r.RecordGitOp(subcmd, d, failed)
	}
}

type opAcc struct {
	count   int
	totalMS float64
	maxMS   float64
	errors  int
	bytes   int64
}

// Recorder buffers and persists performance events without blocking callers.
type Recorder struct {
	opts      Options
	ch        chan Event
	dropped   atomic.Int64
	flight    *flightManager
	stopCh    chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup

	winMu     sync.Mutex
	httpWin   map[string]*opAcc
	gitWin    map[string]*opAcc
	now       func() time.Time
	lastPrune time.Time
}

// New constructs and starts a Recorder writing daily JSONL under opts.Dir.
func New(opts Options) (*Recorder, error) {
	if opts.SlowMS <= 0 {
		opts.SlowMS = 100
	}
	if opts.SampleSeconds <= 0 {
		opts.SampleSeconds = 10
	}
	if opts.MaxBytesMB <= 0 {
		opts.MaxBytesMB = 50
	}
	if opts.RetentionDays <= 0 {
		opts.RetentionDays = 7
	}
	if opts.SpikeMS <= 0 {
		opts.SpikeMS = 500
	}
	if err := os.MkdirAll(opts.Dir, 0o700); err != nil {
		return nil, err
	}
	r := &Recorder{
		opts:    opts,
		ch:      make(chan Event, 2048),
		stopCh:  make(chan struct{}),
		httpWin: make(map[string]*opAcc),
		gitWin:  make(map[string]*opAcc),
		now:     time.Now,
	}
	if opts.FlightRecorder {
		r.flight = newFlightManager(filepath.Join(opts.Dir, "traces"), int64(opts.MaxBytesMB)<<20)
		r.flight.start()
	}
	r.wg.Add(2)
	go r.writeLoop()
	go r.sampleLoop()
	return r, nil
}

// SlowMS returns the configured slow-operation threshold in milliseconds.
func (r *Recorder) SlowMS() int {
	if r == nil {
		return 100
	}
	return r.opts.SlowMS
}

// Dir returns the directory containing JSONL files and spike traces.
func (r *Recorder) Dir() string {
	if r == nil {
		return ""
	}
	return r.opts.Dir
}

// Close flushes buffered events, stops background goroutines, and stops any
// active FlightRecorder. Safe to call more than once.
func (r *Recorder) Close() {
	if r == nil {
		return
	}
	r.closeOnce.Do(func() {
		close(r.stopCh)
		r.wg.Wait()
		if r.flight != nil {
			r.flight.stop()
		}
	})
}

// Emit enqueues an event without blocking; drops silently if saturated.
func (r *Recorder) Emit(ev Event) {
	if r == nil {
		return
	}
	if ev.TS == "" {
		ev.TS = r.now().UTC().Format(time.RFC3339Nano)
	}
	select {
	case r.ch <- ev:
	default:
		r.dropped.Add(1)
	}
}

// RecordHTTP records one completed HTTP request into the rolling window and
// immediately emits a slow event (and optional FlightRecorder dump) if needed.
func (r *Recorder) RecordHTTP(op string, status int, d time.Duration, gitD time.Duration, bytes int64) {
	if r == nil {
		return
	}
	ms := round2(float64(d) / float64(time.Millisecond))
	gitMS := round2(float64(gitD) / float64(time.Millisecond))
	r.addWindow(r.httpWin, op, ms, status >= 500, bytes)
	if ms >= float64(r.opts.SlowMS) || status >= 500 {
		r.Emit(Event{
			Source:     "backend",
			Kind:       "http",
			Op:         op,
			DurationMS: ms,
			GitMS:      gitMS,
			Status:     status,
			Bytes:      bytes,
		})
	}
	if ms >= float64(r.opts.SpikeMS) {
		r.triggerFlight("http_"+op, ms)
	}
}

// RecordGitOp records one Git subprocess invocation into the rolling window and
// emits a slow event if it meets SlowMS or failed slowly.
func (r *Recorder) RecordGitOp(subcmd string, d time.Duration, failed bool) {
	if r == nil {
		return
	}
	ms := round2(float64(d) / float64(time.Millisecond))
	r.addWindow(r.gitWin, subcmd, ms, failed, 0)
	if ms >= float64(r.opts.SlowMS) {
		detail := ""
		if failed {
			detail = "failed"
		}
		r.Emit(Event{
			Source:     "backend",
			Kind:       "git",
			Op:         subcmd,
			DurationMS: ms,
			Detail:     detail,
		})
	}
	if ms >= float64(r.opts.SpikeMS) {
		r.triggerFlight("git_"+subcmd, ms)
	}
}

// RecordFrontend stores one validated, sanitized browser event.
func (r *Recorder) RecordFrontend(ev Event) {
	if r == nil {
		return
	}
	ev.Source = "frontend"
	ev.Kind = clampStr(ev.Kind, 32)
	ev.Op = clampStr(ev.Op, 160)
	ev.Route = clampStr(ev.Route, 128)
	ev.Detail = clampStr(ev.Detail, 256)
	ev.DurationMS = round2(sanitizeFloat(ev.DurationMS))
	ev.ServerMS = round2(sanitizeFloat(ev.ServerMS))
	ev.Value = round2(sanitizeFloat(ev.Value))
	r.Emit(ev)
}

func (r *Recorder) addWindow(m map[string]*opAcc, op string, ms float64, isErr bool, bytes int64) {
	r.winMu.Lock()
	defer r.winMu.Unlock()
	acc, ok := m[op]
	if !ok {
		if len(m) >= 256 {
			op = "(other)"
			acc = m[op]
			if acc == nil {
				acc = &opAcc{}
				m[op] = acc
			}
		} else {
			acc = &opAcc{}
			m[op] = acc
		}
	}
	acc.count++
	acc.totalMS += ms
	if ms > acc.maxMS {
		acc.maxMS = ms
	}
	if isErr {
		acc.errors++
	}
	acc.bytes += bytes
}

func (r *Recorder) drainTopOps(m map[string]*opAcc, limit int) []OpWindow {
	r.winMu.Lock()
	if len(m) == 0 {
		r.winMu.Unlock()
		return nil
	}
	items := make([]OpWindow, 0, len(m))
	for op, acc := range m {
		items = append(items, OpWindow{
			Op:         op,
			Count:      acc.count,
			TotalMS:    round2(acc.totalMS),
			MaxMS:      round2(acc.maxMS),
			ErrorCount: acc.errors,
			Bytes:      acc.bytes,
		})
	}
	clear(m)
	r.winMu.Unlock()
	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalMS != items[j].TotalMS {
			return items[i].TotalMS > items[j].TotalMS
		}
		return items[i].MaxMS > items[j].MaxMS
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (r *Recorder) triggerFlight(reason string, ms float64) {
	if r.flight == nil {
		return
	}
	if path, ok := r.flight.capture(reason); ok {
		r.Emit(Event{
			Source:     "backend",
			Kind:       "flight_dump",
			Op:         clampStr(reason, 120),
			DurationMS: ms,
			Detail:     filepath.Base(path),
		})
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func sanitizeFloat(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0
	}
	return v
}

func clampStr(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
