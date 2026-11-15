//go:build linux

package hoststat

import (
	"bufio"
	"bytes"
	"os"
	"strconv"
	"strings"
)

func readLoad() *Load {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return nil
	}
	return parseLoad(string(data))
}

func readMemory() *Memory {
	mem, _ := readMeminfo()
	return mem
}

func readSwap() *Swap {
	_, swap := readMeminfo()
	return swap
}

func readMeminfo() (*Memory, *Swap) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, nil
	}
	return parseMeminfo(data)
}

func parseLoad(text string) *Load {
	fields := strings.Fields(text)
	if len(fields) < 3 {
		return nil
	}
	var v [3]float64
	for i := range v {
		f, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return nil
		}
		v[i] = f
	}
	return &Load{One: v[0], Five: v[1], Fifteen: v[2]}
}

// parseMeminfo reads MemTotal, MemAvailable, SwapTotal and SwapFree (kB)
// from /proc/meminfo. A host without swap reports no Swap.
func parseMeminfo(data []byte) (*Memory, *Swap) {
	var memTotal, memAvail, swapTotal, swapFree uint64
	var haveMemTotal, haveMemAvail, haveSwapTotal, haveSwapFree bool
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		name, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		var value *uint64
		var flag *bool
		switch name {
		case "MemTotal":
			value, flag = &memTotal, &haveMemTotal
		case "MemAvailable":
			value, flag = &memAvail, &haveMemAvail
		case "SwapTotal":
			value, flag = &swapTotal, &haveSwapTotal
		case "SwapFree":
			value, flag = &swapFree, &haveSwapFree
		default:
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return nil, nil
		}
		kb, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil, nil
		}
		*value, *flag = kb<<10, true
	}
	var mem *Memory
	if haveMemTotal && haveMemAvail && memTotal > 0 {
		mem = &Memory{Total: memTotal, Available: min(memAvail, memTotal)}
	}
	var swap *Swap
	if haveSwapTotal && haveSwapFree && swapTotal > 0 {
		swap = &Swap{Total: swapTotal, Used: swapTotal - min(swapFree, swapTotal)}
	}
	return mem, swap
}
