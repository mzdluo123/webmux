package perf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecorderWritesSlowEventsSamplesAndReport(t *testing.T) {
	dir := t.TempDir()
	rec, err := New(Options{
		Dir:           dir,
		SlowMS:        50,
		SampleSeconds: 1,
		MaxBytesMB:    5,
		RetentionDays: 7,
		Sessions: func() []SessionRef {
			return []SessionRef{{ID: "s1", Name: "demo", PID: os.Getpid(), Status: "running"}}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	SetDefault(rec)
	defer SetDefault(nil)

	ctx, timer := WithRequestTimer(context.Background())
	RecordGit(ctx, "git status", 80*time.Millisecond, false)
	if timer.GitDuration() != 80*time.Millisecond {
		t.Fatalf("git duration = %v, want 80ms", timer.GitDuration())
	}
	rec.RecordHTTP("GET /api/repositories", 200, 140*time.Millisecond, timer.GitDuration(), 2048)
	rec.RecordHTTP("GET /api/health", 200, 5*time.Millisecond, 0, 128)
	rec.RecordFrontend(Event{
		Kind:       "loaf",
		Op:         "Terminal.svelte:render",
		DurationMS: 92,
		Route:      "/s/:id",
		Detail:     "blocking=42ms",
	})
	rec.RecordFrontend(Event{Kind: "memory", Op: "js_heap_mb", Value: 64.5, Route: "/"})
	rec.RecordFrontend(Event{Kind: "memory", Op: "dom_nodes", Value: 1200, Route: "/"})

	// Wait for at least one periodic sample tick to emit TopOps & TopProcs.
	time.Sleep(1200 * time.Millisecond)
	rec.Close()

	rep, err := Summarize(dir, time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Events < 4 || rep.Samples < 1 {
		t.Fatalf("unexpected counts: events=%d samples=%d", rep.Events, rep.Samples)
	}
	if len(rep.BackendSlowEvents) < 2 {
		t.Fatalf("expected slow http + git events, got %+v", rep.BackendSlowEvents)
	}
	if len(rep.BackendHotOps) < 2 {
		t.Fatalf("expected backend hot ops from sample window, got %+v", rep.BackendHotOps)
	}
	if len(rep.FrontendHotOps) != 1 || rep.FrontendHotOps[0].Op != "Terminal.svelte:render" {
		t.Fatalf("unexpected frontend hot ops: %+v", rep.FrontendHotOps)
	}
	if rep.PeakJSHeapMB != 64.5 || rep.PeakDOMNodes != 1200 {
		t.Fatalf("unexpected frontend peaks: heap=%v dom=%d", rep.PeakJSHeapMB, rep.PeakDOMNodes)
	}
	text := FormatText(rep)
	for _, want := range []string{"Backend Hot Operations", "Backend Slowest Single Events", "Frontend Top Jank", "Terminal.svelte:render"} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q:\n%s", want, text)
		}
	}
}

func TestFlightRecorderCapturesSpikeTrace(t *testing.T) {
	dir := t.TempDir()
	rec, err := New(Options{
		Dir:            dir,
		SlowMS:         50,
		SampleSeconds:  60,
		MaxBytesMB:     10,
		RetentionDays:  7,
		FlightRecorder: true,
		SpikeMS:        100,
	})
	if err != nil {
		t.Fatal(err)
	}
	rec.RecordHTTP("GET /api/slow", 200, 250*time.Millisecond, 0, 512)
	rec.Close()

	rep, err := Summarize(dir, time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.FlightTraces) != 1 {
		t.Fatalf("expected 1 flight trace, got %+v", rep.FlightTraces)
	}
	info, err := os.Stat(rep.FlightTraces[0].Path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("expected non-empty trace file at %s: err=%v", rep.FlightTraces[0].Path, err)
	}
}

func TestPrunePerfDirRespectsAgeAndSize(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	oldFile := filepath.Join(dir, "perf-2020-01-01.jsonl")
	midFile := filepath.Join(dir, "perf-2026-11-18.jsonl")
	newFile := filepath.Join(dir, "perf-2026-11-20.jsonl")
	for _, item := range []struct {
		path string
		age  time.Duration
		size int
	}{
		{oldFile, 10 * 24 * time.Hour, 100},
		{midFile, 2 * 24 * time.Hour, 2048},
		{newFile, time.Hour, 2048},
	} {
		if err := os.WriteFile(item.path, make([]byte, item.size), 0o600); err != nil {
			t.Fatal(err)
		}
		mt := now.Add(-item.age)
		if err := os.Chtimes(item.path, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	prunePerfDir(dir, 3000, 7*24*time.Hour, now)
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expired file should be removed")
	}
	if _, err := os.Stat(midFile); !os.IsNotExist(err) {
		t.Fatalf("oldest file over budget should be removed")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Fatalf("newest file should be retained: %v", err)
	}
}
