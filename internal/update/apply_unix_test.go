//go:build unix

package update

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/rainchan/webmux/internal/fetch"
)

func asRelease(t *testing.T) {
	t.Helper()
	old := release
	release = "1"
	t.Cleanup(func() { release = old })
}

type call struct {
	name string
	args []string
}

// fakeSystem answers ownership queries for one package manager and records
// installs; installErr fails the install; held is what `dpkg-query -W` or
// `dnf versionlock list` prints.
type fakeSystem struct {
	owner      Method
	installErr error
	held       string
	calls      []call
	installed  []byte // package bytes seen at install time
}

func (f *fakeSystem) run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, call{name, args})
	switch {
	case name == "dpkg-query" && args[0] == "-W" && f.held != "":
		return []byte(f.held), nil
	case name == "dnf" && f.held != "":
		return []byte(f.held), nil
	case name == "dpkg-query" && args[0] == "-S" && f.owner == MethodDeb:
		return []byte("webmux:amd64: /usr/bin/webmux\n"), nil
	case name == "rpm" && args[0] == "-qf" && f.owner == MethodRPM:
		return []byte("webmux\n"), nil
	case name == "dpkg" || (name == "rpm" && args[0] == "-U"):
		f.installed, _ = os.ReadFile(args[len(args)-1])
		if f.installErr != nil {
			return []byte("dpkg: error: dpkg frontend lock is locked by another process"), f.installErr
		}
		return nil, nil
	}
	return nil, errors.New("not owned")
}

// applyRelease serves a signed release with deb and rpm packages.
func applyRelease(t *testing.T, latest string) (Client, *atomic.Int32) {
	t.Helper()
	k := testKey(t)
	sums := sumLine("deb-bytes", "webmux_"+latest+"-1_amd64.deb") + sumLine("rpm-bytes", "webmux-"+latest+"-1.aarch64.rpm")
	files := map[string][]byte{
		"SHA256SUMS":                          []byte(sums),
		"SHA256SUMS.minisig":                  signed(t, k, sums, "webmux "+latest),
		"webmux_" + latest + "-1_amd64.deb":   []byte("deb-bytes"),
		"webmux-" + latest + "-1.aarch64.rpm": []byte("rpm-bytes"),
	}
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		b, ok := files[filepath.Base(r.URL.Path)]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return Client{Downloader: fetch.Downloader{GitHubBase: srv.URL, Mirrors: []string{}}, Keys: []PublicKey{k.Public()}}, &hits
}

func applyOptions(t *testing.T, c Client, sys *fakeSystem, current, arch string) (ApplyOptions, *bytes.Buffer) {
	dir := t.TempDir()
	var out bytes.Buffer
	return ApplyOptions{
		Current: current, Exe: "/usr/bin/webmux", GOARCH: arch,
		LockPath: filepath.Join(dir, "lock"), CacheDir: filepath.Join(dir, "cache"),
		Client: c, Run: sys.run, Out: &out,
	}, &out
}

func TestApplyInstallsNewerPackage(t *testing.T) {
	asRelease(t)
	for _, tc := range []struct {
		owner      Method
		arch, want string
		cmd        string
		flag       string
	}{
		{MethodDeb, "amd64", "deb-bytes", "dpkg", "-i"},
		{MethodRPM, "arm64", "rpm-bytes", "rpm", "-U"},
	} {
		c, _ := applyRelease(t, "0.5.0")
		sys := &fakeSystem{owner: tc.owner}
		o, out := applyOptions(t, c, sys, "0.4.0", tc.arch)
		if err := Apply(context.Background(), o); err != nil {
			t.Fatalf("%s: %v", tc.owner, err)
		}
		last := sys.calls[len(sys.calls)-1]
		if last.name != tc.cmd || last.args[0] != tc.flag || last.args[1] != "--" || filepath.Dir(last.args[2]) != o.CacheDir {
			t.Fatalf("%s install argv %+v", tc.owner, last)
		}
		if string(sys.installed) != tc.want {
			t.Fatalf("%s installed %q", tc.owner, sys.installed)
		}
		if entries, _ := os.ReadDir(o.CacheDir); len(entries) != 0 {
			t.Fatalf("%s left %v in cache", tc.owner, entries)
		}
		if !strings.Contains(out.String(), "已安装 0.5.0") {
			t.Fatalf("%s log %q", tc.owner, out.String())
		}
	}
}

func TestApplyUpToDateDownloadsNothing(t *testing.T) {
	asRelease(t)
	c, hits := applyRelease(t, "0.5.0")
	sys := &fakeSystem{owner: MethodDeb}
	o, out := applyOptions(t, c, sys, "0.5.0", "amd64")
	if err := Apply(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 || !strings.Contains(out.String(), "已是最新版本 0.5.0") {
		t.Fatalf("hits %d log %q", hits.Load(), out.String())
	}
	for _, c := range sys.calls {
		if c.name == "dpkg" {
			t.Fatal("installed while up to date")
		}
	}
}

func TestApplyRefusesBeforeNetwork(t *testing.T) {
	asRelease(t)
	c, hits := applyRelease(t, "0.5.0")

	o, _ := applyOptions(t, c, &fakeSystem{owner: MethodDeb}, "0.4.0", "amd64")
	o.Euid = 1000
	if err := Apply(context.Background(), o); !errors.Is(err, ErrNotRoot) {
		t.Fatalf("non-root: %v", err)
	}

	o, _ = applyOptions(t, c, &fakeSystem{owner: MethodOther}, "0.4.0", "amd64")
	if err := Apply(context.Background(), o); !errors.Is(err, ErrUnsupportedInstall) {
		t.Fatalf("unowned binary: %v", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("refusals made %d requests", hits.Load())
	}
}

func TestApplyExitsWhileLocked(t *testing.T) {
	asRelease(t)
	c, hits := applyRelease(t, "0.5.0")
	o, _ := applyOptions(t, c, &fakeSystem{owner: MethodDeb}, "0.4.0", "amd64")
	held, err := os.OpenFile(o.LockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Close() }()
	if err := syscall.Flock(int(held.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	if err := Apply(context.Background(), o); !errors.Is(err, ErrBusy) {
		t.Fatalf("locked: %v", err)
	}
	if hits.Load() != 0 {
		t.Fatal("locked apply used the network")
	}
}

func TestApplyPackageManagerFailure(t *testing.T) {
	asRelease(t)
	c, _ := applyRelease(t, "0.5.0")
	sys := &fakeSystem{owner: MethodDeb, installErr: errors.New("exit status 2")}
	o, _ := applyOptions(t, c, sys, "0.4.0", "amd64")
	err := Apply(context.Background(), o)
	if err == nil || !strings.Contains(err.Error(), "lock is locked") {
		t.Fatalf("failure: %v", err)
	}
	if entries, _ := os.ReadDir(o.CacheDir); len(entries) != 0 {
		t.Fatalf("left %v in cache", entries)
	}
}

func TestApplyRequiresReleaseBuild(t *testing.T) {
	c, hits := applyRelease(t, "0.5.0")
	o, _ := applyOptions(t, c, &fakeSystem{owner: MethodDeb}, "0.4.0", "amd64")
	if err := Apply(context.Background(), o); !errors.Is(err, ErrNotRelease) || hits.Load() != 0 {
		t.Fatalf("dev build: %v, %d requests", err, hits.Load())
	}
}

func TestApplySkipsHeldPackage(t *testing.T) {
	asRelease(t)
	for _, tc := range []struct {
		owner Method
		arch  string
		held  string
	}{
		{MethodDeb, "amd64", "hold ok installed"},
		{MethodRPM, "arm64", "Last metadata expiration check: 0:01:02 ago.\nwebmux-0:0.4.0-1.*\n"},
		{MethodRPM, "arm64", "# Added lock on ...\nPackage name: webmux\nevr = 0.4.0-1\n"},
	} {
		c, hits := applyRelease(t, "0.5.0")
		sys := &fakeSystem{owner: tc.owner, held: tc.held}
		o, out := applyOptions(t, c, sys, "0.4.0", tc.arch)
		if err := Apply(context.Background(), o); err != nil {
			t.Fatalf("%s held: %v", tc.owner, err)
		}
		if hits.Load() != 0 || !strings.Contains(out.String(), "已被锁定版本") {
			t.Fatalf("%s held: %d requests, log %q", tc.owner, hits.Load(), out.String())
		}
	}
	// Other packages' locks and an unheld dpkg status do not stop updates.
	for _, held := range []string{"webmux-tray-0:1.0-1.*\n", "install ok installed"} {
		owner := MethodRPM
		if held == "install ok installed" {
			owner = MethodDeb
		}
		c, _ := applyRelease(t, "0.5.0")
		sys := &fakeSystem{owner: owner, held: held}
		o, out := applyOptions(t, c, sys, "0.4.0", map[Method]string{MethodDeb: "amd64", MethodRPM: "arm64"}[owner])
		if err := Apply(context.Background(), o); err != nil || !strings.Contains(out.String(), "已安装 0.5.0") {
			t.Fatalf("not held (%q): %v, log %q", held, err, out.String())
		}
	}
}
