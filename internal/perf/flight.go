package perf

import (
	"os"
	"path/filepath"
	"runtime/trace"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	flightCooldown = 30 * time.Second
	flightMaxFiles = 10
)

type flightManager struct {
	dir      string
	maxBytes int64
	mu       sync.Mutex
	fr       *trace.FlightRecorder
	lastDump time.Time
}

func newFlightManager(dir string, totalBudgetBytes int64) *flightManager {
	budget := max(totalBudgetBytes*4/10, 4<<20)
	return &flightManager{dir: dir, maxBytes: budget}
}

func (m *flightManager) start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fr != nil {
		return
	}
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return
	}
	fr := trace.NewFlightRecorder(trace.FlightRecorderConfig{
		MinAge:   10 * time.Second,
		MaxBytes: 4 << 20,
	})
	if err := fr.Start(); err != nil {
		return
	}
	m.fr = fr
}

func (m *flightManager) stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fr != nil {
		m.fr.Stop()
		m.fr = nil
	}
}

func (m *flightManager) capture(reason string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fr == nil || !m.fr.Enabled() {
		return "", false
	}
	now := time.Now().UTC()
	if !m.lastDump.IsZero() && now.Sub(m.lastDump) < flightCooldown {
		return "", false
	}
	m.lastDump = now
	name := "trace-" + now.Format("20060102-150405") + "-" + safeTraceTag(reason) + ".out"
	path := filepath.Join(m.dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", false
	}
	_, writeErr := m.fr.WriteTo(f)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return "", false
	}
	pruneTraces(m.dir, m.maxBytes)
	return path, true
}

func pruneTraces(dir string, maxBytes int64) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type traceFile struct {
		path string
		mod  time.Time
		size int64
	}
	var files []traceFile
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "trace-") || !strings.HasSuffix(entry.Name(), ".out") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, traceFile{
			path: filepath.Join(dir, entry.Name()),
			mod:  info.ModTime(),
			size: info.Size(),
		})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	for i := 0; i < len(files) && (len(files)-i > flightMaxFiles || total > maxBytes); i++ {
		if err := os.Remove(files[i].path); err == nil {
			total -= files[i].size
		}
	}
}

func safeTraceTag(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 32 {
			break
		}
	}
	if b.Len() == 0 {
		return "spike"
	}
	return b.String()
}
