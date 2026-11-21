package perf

import (
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/rainchan/webmux/internal/hoststat"
)

func (r *Recorder) sampleLoop() {
	defer r.wg.Done()
	interval := time.Duration(r.opts.SampleSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var (
		prevMem      runtime.MemStats
		prevGC       uint32
		prevTotal    uint64
		prevAt       = r.now()
		prevCPU      = map[int]uint64{}
		prevSelfCPU  uint64
		haveSelfPrev bool
		selfPID      = os.Getpid()
	)
	runtime.ReadMemStats(&prevMem)
	prevGC = prevMem.NumGC
	prevTotal = prevMem.TotalAlloc
	if selfStat, ok := readProcStat(selfPID); ok {
		prevSelfCPU = selfStat.cpuTicks
		haveSelfPrev = true
	}

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			now := r.now()
			elapsed := now.Sub(prevAt).Seconds()
			if elapsed <= 0 {
				elapsed = interval.Seconds()
			}
			prevAt = now

			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			gcDelta := mem.NumGC - prevGC
			maxPauseMS := maxRecentGCPauseMS(&mem, gcDelta)
			allocDelta := uint64(0)
			if mem.TotalAlloc >= prevTotal {
				allocDelta = mem.TotalAlloc - prevTotal
			}
			prevGC = mem.NumGC
			prevTotal = mem.TotalAlloc

			metrics := &SampleMetrics{
				HeapInuseMB:  round2(float64(mem.HeapInuse) / (1 << 20)),
				HeapAllocMB:  round2(float64(mem.HeapAlloc) / (1 << 20)),
				AllocRateMBs: round2((float64(allocDelta) / (1 << 20)) / elapsed),
				Goroutines:   runtime.NumGoroutine(),
				GCDelta:      gcDelta,
				MaxGCPauseMS: round2(maxPauseMS),
				Dropped:      r.dropped.Load(),
			}
			if selfStat, ok := readProcStat(selfPID); ok {
				metrics.SelfRSSMB = round2(float64(selfStat.rssBytes) / (1 << 20))
				if haveSelfPrev && selfStat.cpuTicks >= prevSelfCPU {
					metrics.SelfCPUPct = round2(ticksToCPUPct(selfStat.cpuTicks-prevSelfCPU, elapsed))
				}
				prevSelfCPU = selfStat.cpuTicks
				haveSelfPrev = true
			}
			load, hostMem, _ := hoststat.Read()
			if load != nil {
				metrics.HostLoad1 = round2(load.One)
			}
			if hostMem != nil {
				metrics.HostAvailMB = round2(float64(hostMem.Available) / (1 << 20))
			}

			var topProcs []ProcSample
			if r.opts.Sessions != nil {
				refs := r.opts.Sessions()
				metrics.Sessions = len(refs)
				topProcs, prevCPU = sampleSessions(refs, prevCPU, elapsed)
			}

			httpTop := r.drainTopOps(r.httpWin, 8)
			gitTop := r.drainTopOps(r.gitWin, 8)
			topOps := mergeTopOps(httpTop, gitTop, 8)

			r.Emit(Event{
				TS:       now.UTC().Format(time.RFC3339Nano),
				Source:   "backend",
				Kind:     "sample",
				Op:       "runtime",
				Metrics:  metrics,
				TopOps:   topOps,
				TopProcs: topProcs,
			})

			if maxPauseMS >= 50 {
				r.triggerFlight("gc_pause", maxPauseMS)
			} else if metrics.SelfCPUPct >= 80 {
				r.triggerFlight("cpu_spike", metrics.SelfCPUPct)
			}
		}
	}
}

func maxRecentGCPauseMS(mem *runtime.MemStats, delta uint32) float64 {
	if delta == 0 {
		return 0
	}
	n := min(int(delta), len(mem.PauseNs))
	var maxNS uint64
	for i := 0; i < n; i++ {
		idx := (mem.NumGC - 1 - uint32(i)) % uint32(len(mem.PauseNs))
		if mem.PauseNs[idx] > maxNS {
			maxNS = mem.PauseNs[idx]
		}
	}
	return float64(maxNS) / float64(time.Millisecond)
}

func sampleSessions(refs []SessionRef, prevCPU map[int]uint64, elapsedSec float64) ([]ProcSample, map[int]uint64) {
	if len(refs) == 0 {
		return nil, map[int]uint64{}
	}
	snap := readProcTable()
	if len(snap) == 0 {
		return nil, map[int]uint64{}
	}
	children := make(map[int][]int, len(snap))
	for pid, st := range snap {
		children[st.ppid] = append(children[st.ppid], pid)
	}
	nextCPU := make(map[int]uint64, len(snap))
	samples := make([]ProcSample, 0, len(refs))
	for _, ref := range refs {
		if ref.PID <= 0 {
			continue
		}
		if _, ok := snap[ref.PID]; !ok {
			continue
		}
		queue := []int{ref.PID}
		seen := map[int]bool{ref.PID: true}
		var rssBytes uint64
		var cpuDelta uint64
		for len(queue) > 0 {
			pid := queue[0]
			queue = queue[1:]
			if st, ok := snap[pid]; ok {
				rssBytes += st.rssBytes
				nextCPU[pid] = st.cpuTicks
				if old, had := prevCPU[pid]; had && st.cpuTicks >= old {
					cpuDelta += st.cpuTicks - old
				}
			}
			for _, child := range children[pid] {
				if !seen[child] {
					seen[child] = true
					queue = append(queue, child)
				}
			}
		}
		name := ref.Name
		if name == "" {
			name = ref.ID
		}
		samples = append(samples, ProcSample{
			SessionID: ref.ID,
			Name:      clampStr(name, 48),
			PID:       ref.PID,
			Procs:     len(seen),
			CPUPct:    round2(ticksToCPUPct(cpuDelta, elapsedSec)),
			RSSMB:     round2(float64(rssBytes) / (1 << 20)),
		})
	}
	sort.Slice(samples, func(i, j int) bool {
		// Prioritize active CPU hotspots first, then largest memory holders.
		scoreI := samples[i].CPUPct*20 + samples[i].RSSMB
		scoreJ := samples[j].CPUPct*20 + samples[j].RSSMB
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		return samples[i].PID < samples[j].PID
	})
	if len(samples) > 5 {
		samples = samples[:5]
	}
	return samples, nextCPU
}

func mergeTopOps(a, b []OpWindow, limit int) []OpWindow {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	merged := make([]OpWindow, 0, len(a)+len(b))
	merged = append(merged, a...)
	merged = append(merged, b...)
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].TotalMS != merged[j].TotalMS {
			return merged[i].TotalMS > merged[j].TotalMS
		}
		return merged[i].MaxMS > merged[j].MaxMS
	})
	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}
