// Package hoststat reads the machine's load average, memory and swap so the
// UI can show how busy the host is. Platforms without a reader report nothing.
package hoststat

import "runtime"

// Load is the 1, 5 and 15 minute load averages and the logical CPU count
// they are measured against.
type Load struct {
	One, Five, Fifteen float64
	CPUs               int
}

// Memory is physical memory in bytes. Available is what new work can use
// without swapping: free memory plus reclaimable cache.
type Memory struct{ Total, Available uint64 }

// Swap is swap space in bytes; a nil result means the platform cannot report
// it or the host has none configured.
type Swap struct{ Total, Used uint64 }

// Read returns what the platform reports; a nil result is unavailable.
func Read() (*Load, *Memory, *Swap) {
	load := readLoad()
	if load != nil {
		load.CPUs = runtime.NumCPU()
	}
	return load, readMemory(), readSwap()
}
