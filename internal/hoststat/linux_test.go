//go:build linux

package hoststat

import "testing"

func TestParseMemoryUsesAvailableNotFree(t *testing.T) {
	m, _ := parseMeminfo([]byte("MemTotal:       16000000 kB\nMemFree:          100000 kB\nMemAvailable:    8000000 kB\n"))
	if m == nil || m.Total != 16000000<<10 || m.Available != 8000000<<10 {
		t.Fatalf("got %+v", m)
	}
}

func TestParseMemoryRejectsMissingAvailable(t *testing.T) {
	if m, _ := parseMeminfo([]byte("MemTotal: 1 kB\n")); m != nil {
		t.Fatalf("got %+v", m)
	}
}

func TestParseSwap(t *testing.T) {
	_, s := parseMeminfo([]byte("MemTotal:       16000000 kB\nMemAvailable:    8000000 kB\nSwapTotal:        4000000 kB\nSwapFree:         3000000 kB\n"))
	if s == nil || s.Total != 4000000<<10 || s.Used != 1000000<<10 {
		t.Fatalf("got %+v", s)
	}
}

func TestParseSwapAbsentWhenDisabled(t *testing.T) {
	_, s := parseMeminfo([]byte("MemTotal:       16000000 kB\nMemAvailable:    8000000 kB\nSwapTotal:              0 kB\nSwapFree:               0 kB\n"))
	if s != nil {
		t.Fatalf("got %+v", s)
	}
}

func TestParseLoad(t *testing.T) {
	l := parseLoad("0.52 1.25 2.00 1/300 12345\n")
	if l == nil || l.One != 0.52 || l.Five != 1.25 || l.Fifteen != 2 {
		t.Fatalf("got %+v", l)
	}
	if parseLoad("garbage") != nil {
		t.Fatal("garbage must be unavailable")
	}
}
