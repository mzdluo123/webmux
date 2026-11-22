//go:build windows

package winui

import (
	"context"
	"time"

	"github.com/rainchan/webmux/internal/tray"
	"github.com/rainchan/webmux/internal/update"
	"github.com/rainchan/webmux/internal/wslclient"
)

func (a *App) supervisor() *tray.Supervisor {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sup
}

// onWSLReady attaches the supervisor once WSL and at least one distribution
// become available. It is ignored if the tray is already shutting down.
func (a *App) onWSLReady(distros []wslclient.Distro) {
	a.mu.Lock()
	if a.closing || a.sup != nil {
		a.mu.Unlock()
		return
	}
	a.distros = distros
	settings := cloneSettings(a.settings)
	sup := tray.NewSupervisor(tray.Options{
		Service:     a.svc,
		Reach:       a.o.Reach,
		Settings:    settings,
		Publish:     func(tray.Snapshot) { a.kickUI() },
		Version:     a.o.Version,
		AutoUpgrade: update.IsRelease() && tray.BuiltinDigests() != nil,
		Notify:      a.notifier.Show,
		Logf:        a.log.Printf,
	})
	a.sup = sup
	a.mu.Unlock()
	a.probe(true)
	a.kickUI()
}

// probe refreshes the distribution list, the networking mode and the
// systemd detection in the background, at most every probeInterval unless
// forced.
func (a *App) probe(force bool) {
	a.mu.Lock()
	if a.sup == nil || a.probing || (!force && time.Since(a.lastProbe) < probeInterval) {
		a.mu.Unlock()
		return
	}
	a.probing = true
	a.lastProbe = time.Now()
	settings := a.settings
	a.mu.Unlock()
	go func() {
		ctx := context.Background()
		target := tray.TargetOf(settings)
		distros, err := tray.Distros(ctx, a.o.Runner)
		if err != nil {
			a.log.Printf("wsl.exe --list: %v", err)
		}
		mode := tray.NetworkingMode(ctx, a.o.Runner, target)
		detectCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		detection := tray.DetectSystemd(detectCtx, a.o.Runner, target)
		cancel()
		a.mu.Lock()
		a.probing = false
		if err == nil {
			a.distros = distros
		}
		if a.settings.Distro == settings.Distro {
			a.netMode = mode
			a.detection, a.detected = detection, true
		}
		a.mu.Unlock()
		a.kickUI()
	}()
}
