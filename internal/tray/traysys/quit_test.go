package traysys

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// fakeTray plays the running tray: it exits on the quit event when it
// supports one, and on Kill when it was started from path.
type fakeTray struct {
	alive          atomic.Bool
	supports       bool
	path           string
	killed         []string
	keepaliveStops int
	keepaliveErr   error
}

func (f *fakeTray) quitter() Quitter {
	return Quitter{
		Running: f.alive.Load,
		Signal: func() bool {
			if f.supports {
				f.alive.Store(false)
			}
			return f.supports
		},
		Kill: func(path string) error {
			f.killed = append(f.killed, path)
			if path == f.path {
				f.alive.Store(false)
			}
			return nil
		},
		StopKeepalive: func() error {
			f.keepaliveStops++
			return f.keepaliveErr
		},
		Wait:  20 * time.Millisecond,
		Grace: 20 * time.Millisecond,
		Poll:  time.Millisecond,
	}
}

func TestQuit(t *testing.T) {
	const installed = `C:\Users\u\AppData\Local\Programs\WebMux\webmux-tray.exe`
	tests := []struct {
		name           string
		alive          bool
		supports       bool
		path           string
		opts           QuitOptions
		keepaliveErr   error
		want           int
		kills          int
		keepaliveStops int
	}{
		{name: "no tray", want: QuitNone},
		{name: "no tray stops keepalive when asked", opts: QuitOptions{StopKeepalive: true}, want: QuitNone, keepaliveStops: 1},
		{name: "tray exits on request", alive: true, supports: true, path: installed, opts: QuitOptions{KillPath: installed}, want: QuitDone},
		{name: "tray exits and stops keepalive", alive: true, supports: true, path: installed, opts: QuitOptions{KillPath: installed, StopKeepalive: true}, want: QuitDone, keepaliveStops: 1},
		{name: "keepalive error preserves quit result", alive: true, supports: true, path: installed, opts: QuitOptions{KillPath: installed, StopKeepalive: true}, keepaliveErr: errors.New("gone"), want: QuitDone, keepaliveStops: 1},
		{name: "old tray without kill path", alive: true, path: installed, want: QuitRunning},
		{name: "old tray at kill path", alive: true, path: installed, opts: QuitOptions{KillPath: installed}, want: QuitDone, kills: 1},
		{name: "old tray elsewhere survives and keeps keepalive", alive: true, path: `C:\Users\u\Downloads\webmux-tray-windows-amd64.exe`, opts: QuitOptions{KillPath: installed, StopKeepalive: true}, want: QuitRunning, kills: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeTray{supports: tt.supports, path: tt.path, keepaliveErr: tt.keepaliveErr}
			f.alive.Store(tt.alive)
			if got := f.quitter().Quit(tt.opts); got != tt.want {
				t.Fatalf("Quit = %d, want %d", got, tt.want)
			}
			if len(f.killed) != tt.kills {
				t.Fatalf("Kill calls %v, want %d", f.killed, tt.kills)
			}
			if f.keepaliveStops != tt.keepaliveStops {
				t.Fatalf("StopKeepalive calls = %d, want %d", f.keepaliveStops, tt.keepaliveStops)
			}
		})
	}
}

func TestParseQuit(t *testing.T) {
	tests := []struct {
		args    []string
		want    QuitOptions
		wantErr bool
	}{
		{args: nil},
		{args: []string{"--kill-path", `C:\a b\webmux-tray.exe`}, want: QuitOptions{KillPath: `C:\a b\webmux-tray.exe`}},
		{args: []string{`--kill-path=C:\x.exe`}, want: QuitOptions{KillPath: `C:\x.exe`}},
		{args: []string{"--stop-keepalive"}, want: QuitOptions{StopKeepalive: true}},
		{args: []string{"--kill-path", `C:\a b\webmux-tray.exe`, "--stop-keepalive"}, want: QuitOptions{KillPath: `C:\a b\webmux-tray.exe`, StopKeepalive: true}},
		{args: []string{"--kill-path"}, wantErr: true},
		{args: []string{"--force"}, wantErr: true},
		{args: []string{"extra"}, wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParseQuit(tt.args)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseQuit(%q) = %+v, %v", tt.args, got, err)
		}
	}
}
