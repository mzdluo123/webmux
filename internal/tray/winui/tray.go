//go:build windows

package winui

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/systray"
	"golang.org/x/sys/windows"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/tray"
	"github.com/rainchan/webmux/internal/update"
	"github.com/rainchan/webmux/internal/wslclient"
)

var (
	//go:embed icons/stopped.ico
	iconStopped []byte
	//go:embed icons/starting.ico
	iconStarting []byte
	//go:embed icons/running.ico
	iconRunning []byte
	//go:embed icons/error.ico
	iconError []byte
)

const (
	settingsPoll   = 2 * time.Second
	uiInterval     = 10 * time.Second
	probeInterval  = 30 * time.Second
	invalidSetting = "托盘设置无效，已使用默认值"
)

// Options wire the tray; cmd/webmux-tray builds them.
type Options struct {
	Version string
	// SettingsPath is %APPDATA%\WebMux\tray.json.
	SettingsPath string
	Log          *tray.Log
	// Runner runs wsl.exe for the tray's own probes.
	Runner tray.Runner
	// NewService builds the WSL service for one set of settings.
	NewService func(tray.Settings) *tray.WSLService
	Reach      tray.Reacher
	// Instance signals wake and external quit requests from other tray processes.
	Instance interface {
		Wakes(func())
		Quits(func())
	}
	// Executable is the absolute path of webmux-tray.exe.
	Executable string
	// Installed reports whether Executable runs from the installer's directory.
	Installed bool
	// LaunchInstaller starts a downloaded installer silently and waits for it.
	LaunchInstaller func(path string) (wait func() (int, error), err error)
	// AutostartEnabled reports whether login autostart is enabled for Executable.
	AutostartEnabled func(exe string) bool
	// SetAutostart enables or disables login autostart for Executable.
	SetAutostart func(on bool, exe string) error
	// InspectFirewall reads the Windows Defender Firewall status for exe without administrator rights.
	InspectFirewall func(exe string) tray.FirewallCheck
	// ApplyFirewall adds/repairs or removes the Windows Defender Firewall rule for exe.
	ApplyFirewall func(exe string, allow, clearBlocks bool) tray.FirewallApplyResult
	// AdapterCategories maps Windows interface names to "public", "private", or "domain".
	AdapterCategories func() map[string]tray.NetworkCategory
	// OpenURL opens an http(s) URL in the default browser.
	OpenURL func(url string) error
	// OpenInNotepad opens a Windows path in Notepad.
	OpenInNotepad func(path string) error
	// WSLPath maps a WSL path inside a distribution to its \\wsl.localhost path.
	WSLPath func(distro, path string) (string, error)
	// UpdatesDir is %LOCALAPPDATA%\WebMux\updates, where the tray keeps
	// the installer of its own update.
	UpdatesDir string
	// Releases returns the client reading signed releases of a channel
	// for the tray's own update.
	Releases func(update.Channel) tray.ReleaseClient
}

// App is the running tray.
type App struct {
	o        Options
	log      *tray.Log
	svc      *tray.TargetService
	wslBoot  *tray.WSLBoot
	updater  *tray.SelfUpdater
	menu     *menu
	notifier *Notifier
	clip     *Clipboard
	relays   *relays

	hwnd     windows.HWND
	prevProc atomic.Uintptr

	applyMu    sync.Mutex // serializes settings changes and their dialogs
	firewallMu sync.Mutex // serializes firewall dialogs and UAC elevation

	mu           sync.Mutex
	sup          *tray.Supervisor
	closing      bool
	settings     tray.Settings
	settingsWarn string
	stamp        fileStamp
	distros      []wslclient.Distro
	netMode      string
	detection    tray.Detection
	detected     bool
	lastProbe    time.Time
	probing      bool
	bound        bindings
	journal      string

	lastIcon []byte
	lastTip  string

	kick     chan struct{}
	done     chan struct{}
	quitOnce sync.Once
}

// Run shows the tray until the user quits or Windows ends the session. It
// must run on the main goroutine.
func Run(o Options) {
	a := &App{o: o, log: o.Log, relays: newRelays(o.Log.Printf), kick: make(chan struct{}, 1), done: make(chan struct{})}
	a.notifier = &Notifier{Window: func() windows.HWND { return a.hwnd }, Logf: a.log.Printf}
	a.clip = &Clipboard{Owner: func() windows.HWND { return a.hwnd }}
	a.stamp = statFile(o.SettingsPath)
	settings, err := tray.LoadSettings(o.SettingsPath)
	if err != nil {
		a.log.Printf("tray settings: %v", err)
		a.settingsWarn = invalidSetting
	}
	if _, warn := settings.Channel(); warn != nil {
		a.log.Printf("tray settings: %v", warn)
	}
	updated, failed := "", tray.UpdateAttempt{}
	if err == nil {
		// An invalid file is never rewritten without a menu action.
		settings, updated, failed = a.recordVersion(settings)
	}
	a.settings = settings
	a.svc = tray.NewTargetService(o.NewService, settings)
	a.log.Printf("webmux-tray %s starting", o.Version)
	installed := o.Installed
	a.log.Printf("tray copy: installed %v, release build %v", installed, update.IsRelease())
	a.updater = tray.NewSelfUpdater(tray.SelfUpdateOptions{
		Version:       o.Version,
		Arch:          runtime.GOARCH,
		Release:       update.IsRelease(),
		Installed:     installed,
		Dir:           o.UpdatesDir,
		Client:        o.Releases,
		Channel:       a.channel,
		SetChannel:    a.setChannel,
		AutoUpdate:    a.autoUpdate,
		Idle:          a.updateIdle,
		RecordAttempt: a.recordAttempt,
		Launch:        o.LaunchInstaller,
		Publish:       a.kickUI,
		Logf:          a.log.Printf,
		Updated:       updated,
		FailedAttempt: failed,
	})
	systray.Run(a.onReady, a.onExit)
}

// recordVersion saves this tray's version as lastVersion, clears an update
// attempt it completed, and returns the version to announce as updated and
// the failed update attempt, if any.
func (a *App) recordVersion(s tray.Settings) (tray.Settings, string, tray.UpdateAttempt) {
	next, updated, failed, changed := tray.RecordVersion(s, a.o.Version)
	if !changed {
		return s, "", failed
	}
	if err := tray.SaveSettings(a.o.SettingsPath, next); err != nil {
		a.log.Printf("save tray settings: %v", err)
		return s, updated, failed
	}
	a.stamp = statFile(a.o.SettingsPath)
	return next, updated, failed
}

// recordAttempt saves the release whose installer is about to run, so the
// tray it restarts after a failure knows. An invalid settings file is not
// replaced.
func (a *App) recordAttempt(attempt tray.UpdateAttempt) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.settingsWarn != "" {
		return errors.New("托盘设置文件无效")
	}
	next := cloneSettings(a.settings)
	next.UpdateAttempt = attempt
	if err := tray.SaveSettings(a.o.SettingsPath, next); err != nil {
		return err
	}
	a.settings = next
	a.stamp = statFile(a.o.SettingsPath)
	return nil
}

func (a *App) autoUpdate() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.AutoUpdate
}

func (a *App) channel() (update.Channel, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.Channel()
}

// setChannel saves the update channel the user picked.
func (a *App) setChannel(ch update.Channel) {
	a.changeSettings(func(s *tray.Settings) { s.UpdateChannel = ch })
}

// updateIdle reports that the installer may close the tray now: no server
// install, start, stop or restart runs and no dialog is open.
func (a *App) updateIdle() bool {
	snap := a.snapshot()
	busy := snap.Installing || snap.State == tray.StateStarting || snap.State == tray.StateStopping || snap.State == tray.StateRestarting
	return !busy && openDialogs.Load() == 0
}

func (a *App) onReady() {
	systray.SetIcon(iconStarting)
	a.lastIcon = iconStarting
	systray.SetTooltip("WebMux · 正在检查 WSL")
	a.hwnd = findTrayWindow()
	if a.hwnd == 0 {
		a.log.Printf("tray window not found; notifications and menu refresh are unavailable")
	}
	a.menu = buildMenu()
	a.listen()
	systray.SetOnTapped(func() { go a.openLocal() })
	a.wslBoot = tray.NewWSLBoot(tray.WSLBootOptions{
		Runner:  a.o.Runner,
		Publish: func(tray.WSLStatus) { a.kickUI() },
		OnReady: a.onWSLReady,
		Logf:    a.log.Printf,
	})
	if a.hwnd != 0 {
		subclass(a.hwnd, windows.NewCallback(a.wndProc), &a.prevProc)
	}
	if a.o.Instance != nil {
		a.o.Instance.Wakes(a.onWake)
		a.o.Instance.Quits(a.externalQuit)
	}
	go a.uiLoop()
	go a.watchSettings()
	a.updater.Start()
	a.kickUI()
}

func (a *App) supervisor() *tray.Supervisor {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sup
}

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

// onExit runs when the tray quits or Windows ends the session; it never
// asks anything.
func (a *App) onExit() {
	a.shutdown(false)
}

// wndProc refreshes the status whenever the menu is about to open; the
// menu itself always shows the latest snapshot without waiting.
func (a *App) wndProc(hwnd windows.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == systrayMessage && lParam == wmRButtonUp {
		go a.onMenuOpen()
	}
	return callWindowProc(a.prevProc.Load(), hwnd, msg, wParam, lParam)
}

func (a *App) onMenuOpen() {
	a.updater.MenuOpened()
	if sup := a.supervisor(); sup != nil {
		sup.Refresh()
		// A blocked or incompatible tray never polls the installed file;
		// opening the menu is the user's retry.
		if snap := sup.Snapshot(); snap.Blocker != "" || snap.Compat.Incompatible() {
			sup.Recheck()
		}
		a.probe(false)
		return
	}
	if a.wslBoot != nil {
		a.wslBoot.Retry()
	}
}

func (a *App) kickUI() {
	select {
	case a.kick <- struct{}{}:
	default:
	}
}

func (a *App) snapshot() tray.Snapshot {
	a.mu.Lock()
	sup := a.sup
	settings := cloneSettings(a.settings)
	a.mu.Unlock()
	return a.snapshotFor(sup, settings)
}

func (a *App) snapshotFor(sup *tray.Supervisor, settings tray.Settings) tray.Snapshot {
	if sup != nil {
		return sup.Snapshot()
	}
	st := tray.WSLStatus{State: tray.WSLChecking, Reason: "正在检查 WSL"}
	if a.wslBoot != nil {
		st = a.wslBoot.Status()
	}
	state := tray.StateFailed
	if st.State != tray.WSLUnavailable {
		state = tray.StateStarting
	}
	return tray.Snapshot{State: state, Reason: st.Reason, Mode: settings.Mode, Settings: settings}
}

func (a *App) uiLoop() {
	ticker := time.NewTicker(uiInterval)
	defer ticker.Stop()
	for {
		select {
		case <-a.kick:
		case <-ticker.C:
		case <-a.done:
			return
		}
		a.update()
	}
}

// idle reports that no instance runs and no start or stop is in flight, so
// the service may switch to new paths or another distribution.
func idle(snap tray.Snapshot) bool {
	running := snap.HasStatus && snap.Status.State == contract.StateRunning
	busy := snap.State == tray.StateStarting || snap.State == tray.StateStopping || snap.State == tray.StateRestarting
	return !running && !busy
}

// update reconciles forwarding with the latest state and redraws.
func (a *App) update() {
	a.mu.Lock()
	sup := a.sup
	v := view{
		settings:     cloneSettings(a.settings),
		settingsWarn: a.settingsWarn,
		noWSL:        sup == nil,
		distros:      a.distros,
		detection:    a.detection,
		detected:     a.detected,
	}
	netMode := a.netMode
	a.mu.Unlock()
	snap := a.snapshotFor(sup, v.settings)
	if sup != nil && idle(snap) && a.svc.Settle() {
		go sup.Refresh()
	}
	if !v.noWSL && netMode != "" {
		v.plan = tray.PlanForward(netMode, snap, a.windowsInterfaces(snap.LANEnabled()))
		if snap.LANEnabled() && v.plan.Needed {
			v.firewall = a.inspectFirewall()
		}
	}
	a.reconcileForward(snap, v)
	v.forwarding = a.relays.listening()
	if a.o.AutostartEnabled != nil {
		v.autostart = a.o.AutostartEnabled(a.o.Executable)
	}
	v.installSupported = a.svc.Current().Digests != nil
	if sup != nil {
		a.updater.Observe(snap)
	}
	v.selfUpdate = a.updater.View()

	select {
	case <-a.done:
		return
	default:
	}
	b := a.menu.render(snap, v)
	a.mu.Lock()
	a.bound = b
	a.mu.Unlock()
	if icon := iconFor(snap.State); !bytes.Equal(icon, a.lastIcon) {
		systray.SetIcon(icon)
		a.lastIcon = icon
	}
	if tip := tooltip(snap); tip != a.lastTip {
		systray.SetTooltip(truncateUTF16(tip, 127))
		a.lastTip = tip
	}
}

func truncateUTF16(s string, limit int) string {
	u := windows.StringToUTF16(s)
	if len(u)-1 <= limit {
		return s
	}
	return windows.UTF16ToString(u[:limit])
}

// reconcileForward listens on all candidate Windows addresses while LAN access
// is enabled and NAT forwarding is needed, following address, port and
// condition changes.
func (a *App) reconcileForward(snap tray.Snapshot, v view) {
	want := map[string]string{}
	target := ""
	if host, port, ok := tray.ForwardTarget(snap.Status); ok && snap.LANEnabled() && v.plan.Needed && v.plan.Blocked == "" && snap.HasStatus {
		target = net.JoinHostPort(host, port)
		for _, c := range v.plan.Candidates {
			want[c.Address] = net.JoinHostPort(c.Address, port)
		}
	}
	a.relays.reconcile(want, target)
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

func (a *App) onWake() {
	snap := a.snapshot()
	message := tooltip(snap)
	if snap.Reason != "" {
		message += "\n" + snap.Reason
	}
	a.notifier.Show("WebMux 托盘已在运行", message)
}
