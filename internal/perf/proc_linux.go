//go:build linux

package perf

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type procStat struct {
	ppid     int
	cpuTicks uint64
	rssBytes uint64
}

// USER_HZ is 100 on all standard Linux architectures supported by WebMux.
const linuxClockTicksPerSec = 100.0

func ticksToCPUPct(deltaTicks uint64, elapsedSec float64) float64 {
	if elapsedSec <= 0 || deltaTicks == 0 {
		return 0
	}
	return (float64(deltaTicks) / linuxClockTicksPerSec / elapsedSec) * 100.0
}

func readProcTable() map[int]procStat {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	pageSize := uint64(os.Getpagesize())
	table := make(map[int]procStat, 128)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		st, ok := readProcStatWithPageSize(pid, pageSize)
		if ok {
			table[pid] = st
		}
	}
	return table
}

func readProcStat(pid int) (procStat, bool) {
	return readProcStatWithPageSize(pid, uint64(os.Getpagesize()))
}

func readProcStatWithPageSize(pid int, pageSize uint64) (procStat, bool) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return procStat{}, false
	}
	return parseLinuxProcStat(string(data), pageSize)
}

func parseLinuxProcStat(text string, pageSize uint64) (procStat, bool) {
	end := strings.LastIndexByte(text, ')')
	if end < 0 || end+2 >= len(text) {
		return procStat{}, false
	}
	fields := strings.Fields(text[end+1:])
	// After ") ": [0]=state, [1]=ppid, ..., [11]=utime, [12]=stime, ..., [21]=rss(pages).
	if len(fields) < 22 {
		return procStat{}, false
	}
	ppid, err1 := strconv.Atoi(fields[1])
	utime, err2 := strconv.ParseUint(fields[11], 10, 64)
	stime, err3 := strconv.ParseUint(fields[12], 10, 64)
	rssPages, err4 := strconv.ParseInt(fields[21], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || rssPages < 0 {
		return procStat{}, false
	}
	return procStat{
		ppid:     ppid,
		cpuTicks: utime + stime,
		rssBytes: uint64(rssPages) * pageSize,
	}, true
}
