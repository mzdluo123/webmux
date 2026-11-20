package perf

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Report is the aggregated bottleneck summary generated from local JSONL logs.
type Report struct {
	Dir               string          `json:"dir"`
	Since             string          `json:"since"`
	From              string          `json:"from,omitempty"`
	To                string          `json:"to,omitempty"`
	Events            int             `json:"events"`
	Samples           int             `json:"samples"`
	PeakSelfCPUPct    float64         `json:"peakSelfCpuPct"`
	PeakSelfRSSMB     float64         `json:"peakSelfRssMb"`
	PeakHeapInuseMB   float64         `json:"peakHeapInuseMb"`
	PeakGoroutines    int             `json:"peakGoroutines"`
	PeakGCPauseMS     float64         `json:"peakGcPauseMs"`
	PeakHostLoad1     float64         `json:"peakHostLoad1"`
	PeakJSHeapMB      float64         `json:"peakJsHeapMb"`
	PeakDOMNodes      int             `json:"peakDomNodes"`
	BackendHotOps     []OpRank        `json:"backendHotOps"`
	BackendSlowEvents []Event         `json:"backendSlowEvents"`
	FrontendHotOps    []OpRank        `json:"frontendHotOps"`
	FrontendSlow      []Event         `json:"frontendSlowEvents"`
	PeakSessions      []SessionPeak   `json:"peakSessions"`
	FlightTraces      []FlightSummary `json:"flightTraces"`
}

// OpRank ranks one operation by cumulative and peak duration.
type OpRank struct {
	Kind       string  `json:"kind"`
	Op         string  `json:"op"`
	Detail     string  `json:"detail,omitempty"`
	Count      int     `json:"count"`
	TotalMS    float64 `json:"totalMs"`
	MaxMS      float64 `json:"maxMs"`
	AvgMS      float64 `json:"avgMs"`
	ErrorCount int     `json:"errors,omitempty"`
}

// SessionPeak records the highest CPU% and RSS MB observed for a session.
type SessionPeak struct {
	SessionID  string  `json:"sessionId,omitempty"`
	Name       string  `json:"name"`
	PeakCPUPct float64 `json:"peakCpuPct"`
	PeakRSSMB  float64 `json:"peakRssMb"`
	PeakAt     string  `json:"peakAt"`
}

// FlightSummary points to a captured runtime/trace FlightRecorder file.
type FlightSummary struct {
	TS         string  `json:"ts"`
	Op         string  `json:"op"`
	DurationMS float64 `json:"durationMs"`
	Path       string  `json:"path"`
}

// Summarize reads perf-*.jsonl under dir within the past since duration and
// builds a ranked bottleneck report.
func Summarize(dir string, since time.Duration, top int) (Report, error) {
	if since <= 0 {
		since = 2 * time.Hour
	}
	if top <= 0 {
		top = 10
	}
	rep := Report{
		Dir:               dir,
		Since:             since.String(),
		BackendHotOps:     []OpRank{},
		BackendSlowEvents: []Event{},
		FrontendHotOps:    []OpRank{},
		FrontendSlow:      []Event{},
		PeakSessions:      []SessionPeak{},
		FlightTraces:      []FlightSummary{},
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return rep, nil
		}
		return rep, err
	}
	cutoff := time.Now().UTC().Add(-since)
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "perf-") || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)

	type rankAcc struct {
		kind, op, detail string
		count            int
		totalMS, maxMS   float64
		errors           int
	}
	backendOps := map[string]*rankAcc{}
	frontendOps := map[string]*rankAcc{}
	sessions := map[string]*SessionPeak{}
	addRank := func(m map[string]*rankAcc, kind, op, detail string, count int, totalMS, maxMS float64, errs int) {
		key := kind + "\x00" + op
		acc, ok := m[key]
		if !ok {
			acc = &rankAcc{kind: kind, op: op, detail: detail}
			m[key] = acc
		}
		acc.count += count
		acc.totalMS += totalMS
		if maxMS > acc.maxMS {
			acc.maxMS = maxMS
			if detail != "" {
				acc.detail = detail
			}
		}
		acc.errors += errs
	}

	var firstTS, lastTS time.Time
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64<<10), 2<<20)
		for sc.Scan() {
			var ev Event
			if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
				continue
			}
			ts, err := time.Parse(time.RFC3339Nano, ev.TS)
			if err != nil || ts.Before(cutoff) {
				continue
			}
			rep.Events++
			if firstTS.IsZero() || ts.Before(firstTS) {
				firstTS = ts
			}
			if ts.After(lastTS) {
				lastTS = ts
			}
			switch {
			case ev.Source == "backend" && ev.Kind == "sample":
				rep.Samples++
				if ev.Metrics != nil {
					rep.PeakSelfCPUPct = max(rep.PeakSelfCPUPct, ev.Metrics.SelfCPUPct)
					rep.PeakSelfRSSMB = max(rep.PeakSelfRSSMB, ev.Metrics.SelfRSSMB)
					rep.PeakHeapInuseMB = max(rep.PeakHeapInuseMB, ev.Metrics.HeapInuseMB)
					rep.PeakGoroutines = max(rep.PeakGoroutines, ev.Metrics.Goroutines)
					rep.PeakGCPauseMS = max(rep.PeakGCPauseMS, ev.Metrics.MaxGCPauseMS)
					rep.PeakHostLoad1 = max(rep.PeakHostLoad1, ev.Metrics.HostLoad1)
				}
				for _, op := range ev.TopOps {
					kind := "http"
					if strings.HasPrefix(op.Op, "git ") {
						kind = "git"
					}
					addRank(backendOps, kind, op.Op, "", op.Count, op.TotalMS, op.MaxMS, op.ErrorCount)
				}
				for _, proc := range ev.TopProcs {
					key := proc.SessionID
					if key == "" {
						key = proc.Name
					}
					p, ok := sessions[key]
					if !ok {
						p = &SessionPeak{SessionID: proc.SessionID, Name: proc.Name, PeakAt: ev.TS}
						sessions[key] = p
					}
					if proc.CPUPct > p.PeakCPUPct {
						p.PeakCPUPct = proc.CPUPct
						p.PeakAt = ev.TS
					}
					if proc.RSSMB > p.PeakRSSMB {
						p.PeakRSSMB = proc.RSSMB
					}
				}
			case ev.Source == "backend" && (ev.Kind == "http" || ev.Kind == "git"):
				rep.BackendSlowEvents = append(rep.BackendSlowEvents, ev)
			case ev.Source == "backend" && ev.Kind == "flight_dump":
				rep.FlightTraces = append(rep.FlightTraces, FlightSummary{
					TS:         ev.TS,
					Op:         ev.Op,
					DurationMS: ev.DurationMS,
					Path:       filepath.Join(dir, "traces", ev.Detail),
				})
			case ev.Source == "frontend":
				if ev.Kind == "memory" {
					switch ev.Op {
					case "js_heap_mb":
						rep.PeakJSHeapMB = max(rep.PeakJSHeapMB, ev.Value)
					case "dom_nodes":
						rep.PeakDOMNodes = max(rep.PeakDOMNodes, int(ev.Value))
					}
					continue
				}
				if ev.DurationMS > 0 {
					addRank(frontendOps, ev.Kind, ev.Op, ev.Detail, 1, ev.DurationMS, ev.DurationMS, 0)
					rep.FrontendSlow = append(rep.FrontendSlow, ev)
				} else if ev.Kind == "cls" && ev.Value > 0 {
					addRank(frontendOps, ev.Kind, ev.Op, ev.Detail, 1, ev.Value*1000, ev.Value*1000, 0)
					rep.FrontendSlow = append(rep.FrontendSlow, ev)
				}
			}
		}
		_ = f.Close()
	}

	if !firstTS.IsZero() {
		rep.From = firstTS.Format(time.RFC3339)
		rep.To = lastTS.Format(time.RFC3339)
	}

	toRanks := func(m map[string]*rankAcc) []OpRank {
		out := make([]OpRank, 0, len(m))
		for _, acc := range m {
			avg := 0.0
			if acc.count > 0 {
				avg = round2(acc.totalMS / float64(acc.count))
			}
			out = append(out, OpRank{
				Kind:       acc.kind,
				Op:         acc.op,
				Detail:     acc.detail,
				Count:      acc.count,
				TotalMS:    round2(acc.totalMS),
				MaxMS:      round2(acc.maxMS),
				AvgMS:      avg,
				ErrorCount: acc.errors,
			})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].TotalMS != out[j].TotalMS {
				return out[i].TotalMS > out[j].TotalMS
			}
			return out[i].MaxMS > out[j].MaxMS
		})
		if len(out) > top {
			out = out[:top]
		}
		return out
	}

	rep.BackendHotOps = toRanks(backendOps)
	rep.FrontendHotOps = toRanks(frontendOps)

	sort.Slice(rep.BackendSlowEvents, func(i, j int) bool {
		return rep.BackendSlowEvents[i].DurationMS > rep.BackendSlowEvents[j].DurationMS
	})
	if len(rep.BackendSlowEvents) > top {
		rep.BackendSlowEvents = rep.BackendSlowEvents[:top]
	}

	sort.Slice(rep.FrontendSlow, func(i, j int) bool {
		if rep.FrontendSlow[i].DurationMS != rep.FrontendSlow[j].DurationMS {
			return rep.FrontendSlow[i].DurationMS > rep.FrontendSlow[j].DurationMS
		}
		return rep.FrontendSlow[i].Value > rep.FrontendSlow[j].Value
	})
	if len(rep.FrontendSlow) > top {
		rep.FrontendSlow = rep.FrontendSlow[:top]
	}

	for _, s := range sessions {
		rep.PeakSessions = append(rep.PeakSessions, *s)
	}
	sort.Slice(rep.PeakSessions, func(i, j int) bool {
		scoreI := rep.PeakSessions[i].PeakCPUPct*20 + rep.PeakSessions[i].PeakRSSMB
		scoreJ := rep.PeakSessions[j].PeakCPUPct*20 + rep.PeakSessions[j].PeakRSSMB
		return scoreI > scoreJ
	})
	if len(rep.PeakSessions) > top {
		rep.PeakSessions = rep.PeakSessions[:top]
	}
	return rep, nil
}

// FormatText renders a compact human-readable bottleneck report.
func FormatText(rep Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "WebMux Perf Report (%s, dir: %s)\n", rep.Since, rep.Dir)
	if rep.Events == 0 {
		b.WriteString("No performance events recorded in this window.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "Window: %s .. %s (%d events, %d samples)\n", rep.From, rep.To, rep.Events, rep.Samples)
	fmt.Fprintf(&b, "Backend Peaks: selfCPU=%.1f%% selfRSS=%.1fMB heap=%.1fMB goroutines=%d gcPause=%.1fms hostLoad1=%.2f\n",
		rep.PeakSelfCPUPct, rep.PeakSelfRSSMB, rep.PeakHeapInuseMB, rep.PeakGoroutines, rep.PeakGCPauseMS, rep.PeakHostLoad1)
	if rep.PeakJSHeapMB > 0 || rep.PeakDOMNodes > 0 {
		fmt.Fprintf(&b, "Frontend Peaks: jsHeap=%.1fMB domNodes=%d\n", rep.PeakJSHeapMB, rep.PeakDOMNodes)
	}
	if len(rep.BackendHotOps) > 0 {
		b.WriteString("\nBackend Hot Operations (by cumulative time):\n")
		for _, op := range rep.BackendHotOps {
			fmt.Fprintf(&b, "  %-4s %-42s count=%-5d total=%8.1fms avg=%6.1fms max=%7.1fms\n",
				op.Kind, op.Op, op.Count, op.TotalMS, op.AvgMS, op.MaxMS)
		}
	}
	if len(rep.BackendSlowEvents) > 0 {
		b.WriteString("\nBackend Slowest Single Events:\n")
		for _, ev := range rep.BackendSlowEvents {
			extra := ""
			if ev.GitMS > 0 {
				extra = fmt.Sprintf(" git=%.1fms", ev.GitMS)
			}
			fmt.Fprintf(&b, "  %s %-4s %-38s %7.1fms%s status=%d\n",
				shortTS(ev.TS), ev.Kind, ev.Op, ev.DurationMS, extra, ev.Status)
		}
	}
	if len(rep.FrontendHotOps) > 0 {
		b.WriteString("\nFrontend Top Jank / Slow Operations:\n")
		for _, op := range rep.FrontendHotOps {
			detail := ""
			if op.Detail != "" {
				detail = " [" + op.Detail + "]"
			}
			fmt.Fprintf(&b, "  %-11s %-34s count=%-4d total=%8.1fms max=%7.1fms%s\n",
				op.Kind, op.Op, op.Count, op.TotalMS, op.MaxMS, detail)
		}
	}
	if len(rep.PeakSessions) > 0 {
		b.WriteString("\nTop Resource-Consuming Sessions:\n")
		for _, s := range rep.PeakSessions {
			fmt.Fprintf(&b, "  %-10s %-28s peakCPU=%6.1f%% peakRSS=%7.1fMB (at %s)\n",
				s.SessionID, s.Name, s.PeakCPUPct, s.PeakRSSMB, shortTS(s.PeakAt))
		}
	}
	if len(rep.FlightTraces) > 0 {
		b.WriteString("\nCaptured FlightRecorder Traces (inspect with `go tool trace <path>`):\n")
		for _, tr := range rep.FlightTraces {
			fmt.Fprintf(&b, "  %s %-28s %7.1fms  %s\n", shortTS(tr.TS), tr.Op, tr.DurationMS, tr.Path)
		}
	}
	return b.String()
}

func shortTS(ts string) string {
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t.Local().Format("15:04:05")
	}
	return ts
}
