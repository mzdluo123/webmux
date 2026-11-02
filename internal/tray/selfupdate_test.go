package tray

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/fetch"
	"github.com/rainchan/webmux/internal/update"
)

// jump moves time at once, firing every timer that falls due.
func (c *fakeClock) jump(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	kept := c.waiters[:0]
	for _, w := range c.waiters {
		if !w.at.After(c.now) {
			w.ch <- c.now
		} else {
			kept = append(kept, w)
		}
	}
	c.waiters = kept
}

func (c *fakeClock) pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.waiters)
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// fakeReleases serves one release; Fetch writes the installer.
type fakeReleases struct {
	mu        sync.Mutex
	latest    string
	latestErr error
	checks    int
	fetches   int
}

func (f *fakeReleases) Latest(context.Context) (update.Manifest, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checks++
	return update.Manifest{Version: f.latest}, "github.com", f.latestErr
}

func (f *fakeReleases) Fetch(_ context.Context, m update.Manifest, asset, target string, _ os.FileMode) (string, error) {
	f.mu.Lock()
	f.fetches++
	f.mu.Unlock()
	if asset != "webmux-setup-windows-amd64.exe" || m.Version != f.latest {
		return "", errors.New("unexpected asset " + asset)
	}
	return "github.com", os.WriteFile(target, []byte("installer"), 0o600)
}

func (f *fakeReleases) counts() (checks, fetches int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.checks, f.fetches
}

type updateHarness struct {
	t        *testing.T
	clock    *fakeClock
	dir      string
	auto     atomic.Bool
	idle     atomic.Bool
	mu       sync.Mutex
	launched []string
	u        *SelfUpdater
}

func newUpdateHarness(t *testing.T, installed bool, client ReleaseClient, updated string) *updateHarness {
	h := &updateHarness{t: t, clock: newFakeClock(), dir: filepath.Join(t.TempDir(), "updates")}
	h.auto.Store(true)
	h.idle.Store(true)
	h.u = NewSelfUpdater(SelfUpdateOptions{
		Version:    "0.4.0",
		Arch:       "amd64",
		Release:    true,
		Installed:  installed,
		Dir:        h.dir,
		Client:     client,
		Clock:      h.clock,
		AutoUpdate: h.auto.Load,
		Idle:       h.idle.Load,
		Launch: func(path string) error {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.launched = append(h.launched, path)
			return nil
		},
		Updated: updated,
	})
	t.Cleanup(h.u.Close)
	return h
}

func (h *updateHarness) launches() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.launched...)
}

// settled waits until no check runs and the schedule waits for its timer.
func (h *updateHarness) settled(timers int) {
	h.t.Helper()
	eventually(h.t, "idle updater", func() bool { return !h.u.View().Busy && h.clock.pending() == timers })
}

// The first check comes two minutes after the start, then one a day;
// with automatic updates off nothing is checked unless the user asks.
func TestSelfUpdateSchedule(t *testing.T) {
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, false, releases, "")
	h.u.Start()
	h.settled(1)
	h.clock.jump(UpdateFirstCheck - time.Second)
	if checks, _ := releases.counts(); checks != 0 {
		t.Fatalf("checked before two minutes: %d", checks)
	}
	h.clock.jump(time.Second)
	eventually(t, "first check", func() bool { c, _ := releases.counts(); return c == 1 })
	h.settled(1)
	if v := h.u.View(); v.Latest != "0.5.0" || v.Status != "有新版本 0.5.0" {
		t.Fatalf("after check %+v", v)
	}
	h.clock.jump(UpdateInterval - time.Second)
	if checks, _ := releases.counts(); checks != 1 {
		t.Fatalf("checked before a day: %d", checks)
	}
	h.clock.jump(time.Second)
	eventually(t, "daily check", func() bool { c, _ := releases.counts(); return c == 2 })
	h.settled(1)

	h.auto.Store(false)
	for range 3 {
		h.clock.jump(UpdateInterval)
		h.settled(1)
	}
	if checks, _ := releases.counts(); checks != 2 {
		t.Fatalf("checked with automatic updates off: %d", checks)
	}
}

// With automatic updates off a manual check only reports the release; the
// installed tray installs it when the user picks "更新到".
func TestSelfUpdateManual(t *testing.T) {
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, true, releases, "")
	h.auto.Store(false)
	h.u.Check()
	eventually(t, "check", func() bool { c, _ := releases.counts(); return c == 1 && !h.u.View().Busy })
	v := h.u.View()
	if _, fetches := releases.counts(); fetches != 0 || len(h.launches()) != 0 || v.Status != "有新版本 0.5.0" {
		t.Fatalf("manual check %+v", v)
	}
	if entry := UpdateEntry(Snapshot{}, v); entry.Title != "更新到 0.5.0" || entry.Action != UpdateInstall || !entry.Enabled {
		t.Fatalf("entry %+v", entry)
	}
	h.u.Install()
	eventually(t, "installer", func() bool { return len(h.launches()) == 1 })
	if got := h.launches()[0]; got != filepath.Join(h.dir, "webmux-setup-windows-amd64.exe") {
		t.Fatalf("launched %q", got)
	}
	if v := h.u.View(); !v.Busy || v.Status != "正在安装托盘 0.5.0…" {
		t.Fatalf("after launch %+v", v)
	}
	// The installer closes this tray; nothing starts a second one.
	h.u.Check()
	time.Sleep(10 * time.Millisecond)
	if checks, _ := releases.counts(); checks != 2 || len(h.launches()) != 1 {
		t.Fatalf("checked after launch: %d %v", checks, h.launches())
	}
}

// A downloaded installer waits while the tray restarts WebMux, installs a
// server or shows a dialog.
func TestSelfUpdateWaitsForIdle(t *testing.T) {
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, true, releases, "")
	h.idle.Store(false)
	h.u.Check()
	eventually(t, "download", func() bool { return h.clock.pending() == 1 })
	if v := h.u.View(); v.Status != "托盘 0.5.0 已下载，当前操作完成后安装" || !v.Busy {
		t.Fatalf("waiting %+v", v)
	}
	for range 3 {
		h.clock.jump(UpdateIdlePoll)
		eventually(t, "next poll", func() bool { return h.clock.pending() == 1 })
	}
	if len(h.launches()) != 0 {
		t.Fatalf("installer ran while busy: %v", h.launches())
	}
	h.idle.Store(true)
	h.clock.jump(UpdateIdlePoll)
	eventually(t, "installer", func() bool { return len(h.launches()) == 1 })
}

// A portable tray reports the release and links its page, but never
// downloads or runs an installer, even when asked.
func TestSelfUpdatePortableNeverDownloads(t *testing.T) {
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, false, releases, "")
	h.u.Check()
	eventually(t, "check", func() bool { c, _ := releases.counts(); return c == 1 && !h.u.View().Busy })
	h.u.Install()
	eventually(t, "second check", func() bool { c, _ := releases.counts(); return c == 2 && !h.u.View().Busy })
	if _, fetches := releases.counts(); fetches != 0 || len(h.launches()) != 0 {
		t.Fatalf("portable tray downloaded %d, launched %v", fetches, h.launches())
	}
	v := h.u.View()
	entry := UpdateEntry(Snapshot{}, v)
	if v.Status != "有新版本 0.5.0" || entry.Title != "下载托盘 0.5.0" || entry.URL != ReleasePage("0.5.0") || entry.Action != UpdateOpenRelease {
		t.Fatalf("portable %+v %+v", v, entry)
	}
	if _, err := os.Stat(h.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("updates directory created: %v", err)
	}
}

// signedRelease serves a release whose checksum list is signed by key.
type signedRelease struct {
	key       update.SecretKey
	sums      string
	installer []byte
	checks    atomic.Int32
}

func (r *signedRelease) serve(t *testing.T) string {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		base := "/" + update.Repo + "/releases/"
		switch req.URL.Path {
		case base + "latest/download/SHA256SUMS":
			r.checks.Add(1)
			_, _ = w.Write([]byte(r.sums))
		case base + "latest/download/SHA256SUMS.minisig":
			sig, err := r.key.Sign([]byte(r.sums), "webmux 0.5.0")
			if err != nil {
				t.Error(err)
			}
			_, _ = w.Write(sig)
		case base + "download/v0.5.0/webmux-setup-windows-amd64.exe":
			_, _ = w.Write(r.installer)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func testReleaseKey(t *testing.T) update.SecretKey {
	t.Helper()
	k, err := update.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// Nothing unverified is ever run: a checksum list signed by an unknown key
// and an installer that does not match the signed list both fail, the
// status line shows why, and the next scheduled check retries.
func TestSelfUpdateRunsOnlyVerifiedInstallers(t *testing.T) {
	trusted := testReleaseKey(t)
	installer := []byte("MZ installer")
	sum := sha256.Sum256(installer)
	sums := hex.EncodeToString(sum[:]) + "  webmux-setup-windows-amd64.exe\n"
	for _, tc := range []struct {
		name   string
		signer update.SecretKey
		served []byte
		reason string
	}{
		{"unknown key", testReleaseKey(t), installer, "更新失败："},
		{"tampered installer", trusted, []byte("MZ evil"), "更新失败：下载 webmux-setup-windows-amd64.exe 0.5.0："},
	} {
		t.Run(tc.name, func(t *testing.T) {
			release := &signedRelease{key: tc.signer, sums: sums, installer: tc.served}
			client := update.Client{
				Downloader: fetch.Downloader{GitHubBase: release.serve(t), Mirrors: []string{}},
				Keys:       []update.PublicKey{trusted.Public()},
			}
			h := newUpdateHarness(t, true, client, "")
			h.u.Start()
			h.settled(1)
			h.clock.jump(UpdateFirstCheck)
			eventually(t, "failed check", func() bool {
				return release.checks.Load() == 1 && strings.HasPrefix(h.u.View().Status, "更新失败")
			})
			h.settled(1)
			if v := h.u.View(); !strings.HasPrefix(v.Status, tc.reason) || len(h.launches()) != 0 {
				t.Fatalf("status %q, launched %v", v.Status, h.launches())
			}
			if entries, _ := os.ReadDir(h.dir); len(entries) != 0 {
				t.Fatalf("unverified download kept: %v", entries)
			}
			h.clock.jump(UpdateInterval)
			eventually(t, "retry", func() bool { return release.checks.Load() == 2 })
			h.settled(1)
			if len(h.launches()) != 0 {
				t.Fatalf("launched %v", h.launches())
			}
		})
	}
	// The same release, correctly signed, installs.
	release := &signedRelease{key: trusted, sums: sums, installer: installer}
	client := update.Client{
		Downloader: fetch.Downloader{GitHubBase: release.serve(t), Mirrors: []string{}},
		Keys:       []update.PublicKey{trusted.Public()},
	}
	h := newUpdateHarness(t, true, client, "")
	h.u.Check()
	eventually(t, "installer", func() bool { return len(h.launches()) == 1 })
	if data, err := os.ReadFile(h.launches()[0]); err != nil || string(data) != string(installer) {
		t.Fatalf("installer %q %v", data, err)
	}
}

// The new tray announces its version until the menu opened twice, reports
// a manual check that found nothing, and empties the updates directory.
func TestSelfUpdateNotices(t *testing.T) {
	releases := &fakeReleases{latest: "0.4.0"}
	h := newUpdateHarness(t, true, releases, "0.4.0")
	if err := os.MkdirAll(h.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.dir, "webmux-setup-windows-amd64.exe"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.u.Start()
	eventually(t, "cleanup", func() bool { entries, _ := os.ReadDir(h.dir); return len(entries) == 0 })
	if v := h.u.View(); v.Status != "已更新到 0.4.0" {
		t.Fatalf("notice %+v", v)
	}
	h.u.MenuOpened()
	if v := h.u.View(); v.Status != "已更新到 0.4.0" {
		t.Fatalf("first menu %+v", v)
	}
	h.u.MenuOpened()
	if v := h.u.View(); v.Status != "" {
		t.Fatalf("second menu %+v", v)
	}
	h.u.Check()
	eventually(t, "check", func() bool { c, _ := releases.counts(); return c == 1 && !h.u.View().Busy })
	if v := h.u.View(); v.Status != "托盘已是最新版本 0.4.0" || v.Latest != "" {
		t.Fatalf("current %+v", v)
	}
	if _, fetches := releases.counts(); fetches != 0 {
		t.Fatal("downloaded the current version")
	}
}

// A newer WSL webmux makes an installed tray with automatic updates check
// once per version; portable trays and disabled updates never check.
func TestSelfUpdateFollowsNewerServer(t *testing.T) {
	newer := Snapshot{Tray: BuildInfo{Version: "0.4.0", Contract: 1}, Installed: BuildInfo{Version: "0.5.0", Contract: 1}, Compat: CompatServerNewer}
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, true, releases, "")
	h.u.Observe(newer)
	eventually(t, "installer", func() bool { return len(h.launches()) == 1 })
	h.u.Observe(newer)
	time.Sleep(10 * time.Millisecond)
	if checks, _ := releases.counts(); checks != 1 {
		t.Fatalf("checks %d", checks)
	}

	for name, installed := range map[string]bool{"portable": false, "disabled": true} {
		releases := &fakeReleases{latest: "0.5.0"}
		h := newUpdateHarness(t, installed, releases, "")
		h.auto.Store(name != "disabled")
		h.u.Observe(newer)
		time.Sleep(10 * time.Millisecond)
		if checks, _ := releases.counts(); checks != 0 {
			t.Errorf("%s: checked %d", name, checks)
		}
	}
}

// A download the starting tray cannot remove yet (its installer is still
// finishing) is removed by a later attempt.
func TestSelfUpdateRetriesCleanup(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	h := newUpdateHarness(t, true, &fakeReleases{latest: "0.4.0"}, "")
	if err := os.MkdirAll(h.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.dir, "webmux-setup-windows-amd64.exe"), []byte("running"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(h.dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(h.dir, 0o700) })
	h.u.Start()
	h.settled(2) // first check and cleanup retry
	if err := os.Chmod(h.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	h.clock.jump(updateCleanRetry)
	eventually(t, "cleanup", func() bool { entries, _ := os.ReadDir(h.dir); return len(entries) == 0 })
	h.settled(1)
}
