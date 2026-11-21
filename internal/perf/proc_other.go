//go:build !linux

package perf

type procStat struct {
	ppid     int
	cpuTicks uint64
	rssBytes uint64
}

func ticksToCPUPct(uint64, float64) float64 { return 0 }
func readProcTable() map[int]procStat       { return nil }
func readProcStat(int) (procStat, bool)     { return procStat{}, false }
