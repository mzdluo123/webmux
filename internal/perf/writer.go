package perf

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (r *Recorder) writeLoop() {
	defer r.wg.Done()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var (
		fileDay string
		file    *os.File
		writer  *bufio.Writer
		pending int
	)
	flush := func() {
		if writer != nil && pending > 0 {
			_ = writer.Flush()
			pending = 0
		}
	}
	closeFile := func() {
		flush()
		if file != nil {
			_ = file.Close()
			file = nil
			writer = nil
			fileDay = ""
		}
	}
	defer closeFile()

	writeOne := func(ev Event) {
		day := r.now().UTC().Format("2006-01-02")
		if day != fileDay || file == nil {
			closeFile()
			path := filepath.Join(r.opts.Dir, "perf-"+day+".jsonl")
			f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				return
			}
			file = f
			writer = bufio.NewWriterSize(f, 16<<10)
			fileDay = day
			r.pruneIfDue()
		}
		line, err := json.Marshal(ev)
		if err != nil {
			return
		}
		_, _ = writer.Write(line)
		_ = writer.WriteByte('\n')
		pending++
		if pending >= 64 {
			flush()
			r.pruneIfDue()
		}
	}

	for {
		select {
		case ev := <-r.ch:
			writeOne(ev)
		case <-ticker.C:
			flush()
			r.pruneIfDue()
		case <-r.stopCh:
			for {
				select {
				case ev := <-r.ch:
					writeOne(ev)
				default:
					return
				}
			}
		}
	}
}

func (r *Recorder) pruneIfDue() {
	now := r.now()
	if !r.lastPrune.IsZero() && now.Sub(r.lastPrune) < time.Minute {
		return
	}
	r.lastPrune = now
	prunePerfDir(r.opts.Dir, int64(r.opts.MaxBytesMB)<<20, time.Duration(r.opts.RetentionDays)*24*time.Hour, now)
}

func prunePerfDir(dir string, maxBytes int64, maxAge time.Duration, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type fileInfo struct {
		path string
		mod  time.Time
		size int64
	}
	var files []fileInfo
	var total int64
	cutoff := now.Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "perf-") || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		p := filepath.Join(dir, entry.Name())
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(p)
			continue
		}
		files = append(files, fileInfo{path: p, mod: info.ModTime(), size: info.Size()})
		total += info.Size()
	}
	if total <= maxBytes {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	// Keep at least the newest active file even if it briefly exceeds the cap.
	for i := 0; i < len(files)-1 && total > maxBytes; i++ {
		if err := os.Remove(files[i].path); err == nil {
			total -= files[i].size
		}
	}
}
