package agentcheck_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/agentcatalog"
	"github.com/rainchan/webmux/internal/agentcheck"
	"github.com/rainchan/webmux/internal/agentprofile"
)

func writeExecutable(t *testing.T, dir, name, script string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestInspectStartupShellCandidatesTimeoutAndRefresh(t *testing.T) {
	home := t.TempDir()
	userBin := filepath.Join(home, ".user-bin")
	if err := os.MkdirAll(userBin, 0o755); err != nil {
		t.Fatal(err)
	}

	// Only ~/.bashrc adds userBin to PATH; it also prints greeting noise and sets an alias.
	bashrc := "echo 'welcome from bashrc'\nalias tool-alpha='tool-alpha --flag'\nexport PATH=\"" + userBin + ":$PATH\"\n"
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte(bashrc), 0o644); err != nil {
		t.Fatal(err)
	}

	alphaPath := writeExecutable(t, userBin, "tool-alpha", "#!/bin/sh\necho '2.1.300 (Alpha CLI)'\n")
	betaPath := writeExecutable(t, userBin, "tool-beta-alt", "#!/bin/sh\necho 'tool-beta 0.9.0'\n")
	connPath := writeExecutable(t, userBin, "alpha-acp", "#!/bin/sh\necho 'v0.10.2'\n")
	hungPath := writeExecutable(t, userBin, "tool-hung", "#!/bin/sh\nsleep 30 &\nwait\n")

	var httpHits atomic.Int32
	forbidSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpHits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer forbidSrv.Close()

	entries := []agentcatalog.Entry{
		{
			ID:              "alpha",
			Title:           "Alpha Agent",
			Icon:            "terminal",
			Kind:            agentcatalog.KindHybrid,
			Executables:     []string{"tool-alpha"},
			VerifiedVersion: "2.1.310",
			Distribution: agentprofile.Distribution{
				InstallCommand: "npm install -g @example/alpha",
				UpgradeCommand: "npm install -g @example/alpha@latest",
				Upstream:       agentprofile.NPMUpstream("@example/alpha"),
			},
			Connector: &agentcatalog.ConnectorEntry{
				Executable:      "alpha-acp",
				Title:           "alpha-acp",
				VerifiedVersion: "0.10.0",
				Distribution: agentprofile.Distribution{
					InstallCommand: "npm install -g @example/alpha-acp",
					Upstream:       agentprofile.NPMUpstream("@example/alpha-acp"),
				},
			},
		},
		{
			ID:          "beta",
			Title:       "Beta Agent",
			Icon:        "terminal",
			Kind:        agentcatalog.KindHybrid,
			Executables: []string{"tool-beta-primary", "tool-beta-alt"},
			Distribution: agentprofile.Distribution{
				DocsURL: "https://example.com/beta",
			},
		},
		{
			ID:          "hung",
			Title:       "Hung Agent",
			Icon:        "terminal",
			Kind:        agentcatalog.KindTerminal,
			Executables: []string{"tool-hung"},
			Distribution: agentprofile.Distribution{
				Upstream: agentprofile.NPMUpstream("hung-pkg"),
			},
		},
		{
			ID:          "gamma",
			Title:       "Gamma Agent",
			Icon:        "terminal",
			Kind:        agentcatalog.KindTerminal,
			Executables: []string{"tool-gamma"},
			Distribution: agentprofile.Distribution{
				InstallCommand: "npm install -g gamma-cli",
				Upstream:       agentprofile.NPMUpstream("gamma-cli"),
			},
		},
	}

	now := time.Date(2026, 11, 16, 4, 0, 0, 0, time.UTC)
	svc := agentcheck.New(agentcheck.Options{
		Entries:        entries,
		ShellPath:      "/bin/bash",
		Env:            []string{"HOME=" + home, "PATH=/usr/bin:/bin", "SHELL=/bin/bash"},
		NPMBases:       []string{forbidSrv.URL},
		PyPIBase:       forbidSrv.URL,
		GitHubAPIBase:  forbidSrv.URL,
		LocalTTL:       5 * time.Minute,
		LookupTimeout:  4 * time.Second,
		VersionTimeout: 250 * time.Millisecond,
		Now:            func() time.Time { return now },
	})

	start := time.Now()
	rep, err := svc.Inspect(context.Background(), false)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if elapsed > 4*time.Second {
		t.Fatalf("Inspect took %s; hung child should time out promptly", elapsed)
	}
	if httpHits.Load() != 0 {
		t.Fatalf("Inspect made %d HTTP requests, want 0", httpHits.Load())
	}
	if rep.Shell != "bash" {
		t.Errorf("Shell = %q, want bash", rep.Shell)
	}
	if len(rep.Tools) != 4 {
		t.Fatalf("len(Tools) = %d, want 4", len(rep.Tools))
	}

	alpha := rep.Tools[0]
	if !alpha.Installed || alpha.Path != alphaPath || alpha.InstalledVersion != "2.1.300" {
		t.Errorf("alpha = %+v, want installed at %s v2.1.300", alpha, alphaPath)
	}
	if !alpha.BelowVerified {
		t.Errorf("alpha.BelowVerified = false, want true (2.1.300 < 2.1.310)")
	}
	if alpha.UpdateStatus != agentcheck.StatusUnchecked {
		t.Errorf("alpha.UpdateStatus = %q, want unchecked", alpha.UpdateStatus)
	}
	if alpha.Connector == nil || !alpha.Connector.Installed || alpha.Connector.Path != connPath || alpha.Connector.InstalledVersion != "0.10.2" {
		t.Fatalf("alpha.Connector = %+v, want installed v0.10.2", alpha.Connector)
	}
	if alpha.Connector.BelowVerified {
		t.Errorf("alpha.Connector.BelowVerified = true, want false (0.10.2 >= 0.10.0)")
	}

	beta := rep.Tools[1]
	if !beta.Installed || beta.Executable != "tool-beta-alt" || beta.Path != betaPath || beta.InstalledVersion != "0.9.0" {
		t.Errorf("beta = %+v, want matched second executable tool-beta-alt v0.9.0", beta)
	}
	if beta.UpdateStatus != agentcheck.StatusSourceUnavailable {
		t.Errorf("beta.UpdateStatus = %q, want source_unavailable", beta.UpdateStatus)
	}

	hung := rep.Tools[2]
	if !hung.Installed || hung.Path != hungPath || hung.InstalledVersion != "" {
		t.Errorf("hung = %+v, want installed=true with empty version after timeout", hung)
	}

	gamma := rep.Tools[3]
	if gamma.Installed || gamma.UpdateStatus != agentcheck.StatusNotInstalled {
		t.Errorf("gamma = %+v, want not_installed", gamma)
	}

	// Install gamma on disk; cached Inspect(refresh=false) keeps old snapshot, refresh=true picks it up.
	gammaPath := writeExecutable(t, userBin, "tool-gamma", "#!/bin/sh\necho '1.4.2'\n")
	cachedRep, err := svc.Inspect(context.Background(), false)
	if err != nil {
		t.Fatalf("Inspect cached: %v", err)
	}
	if cachedRep.Tools[3].Installed {
		t.Errorf("cached Inspect unexpectedly rescanned gamma")
	}

	freshRep, err := svc.Inspect(context.Background(), true)
	if err != nil {
		t.Fatalf("Inspect refresh: %v", err)
	}
	if !freshRep.Tools[3].Installed || freshRep.Tools[3].Path != gammaPath || freshRep.Tools[3].InstalledVersion != "1.4.2" {
		t.Errorf("fresh gamma = %+v, want installed at %s v1.4.2", freshRep.Tools[3], gammaPath)
	}
}

func TestCheckUpdatesMirrorFallbackIsolationAndRedirectGuard(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, binDir, "alpha-cli", "#!/bin/sh\necho '2.1.300'\n")
	writeExecutable(t, binDir, "alpha-acp", "#!/bin/sh\necho '0.11.0'\n")
	writeExecutable(t, binDir, "py-agent", "#!/bin/sh\necho 'py-agent, version 1.5.0'\n")
	writeExecutable(t, binDir, "bad-redirect-cli", "#!/bin/sh\necho '1.0.0'\n")
	writeExecutable(t, binDir, "broken-cli", "#!/bin/sh\necho '1.0.0'\n")

	evilSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"version":"9.9.9"}`))
	}))
	defer evilSrv.Close()

	// Primary npm fails on @scope/alpha so mirror must serve it; redirect-pkg tries redirecting to evilSrv.
	primaryNPM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/@scope/alpha/latest":
			w.WriteHeader(http.StatusBadGateway)
		case "/redirect-pkg/latest":
			http.Redirect(w, r, evilSrv.URL+"/redirect-pkg/latest", http.StatusFound)
		case "/broken-pkg/latest":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer primaryNPM.Close()

	mirrorNPM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/@scope/alpha/latest":
			_, _ = w.Write([]byte(`{"version":"2.1.320"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mirrorNPM.Close()

	var pypiHits atomic.Int32
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pypiHits.Add(1)
		if r.URL.Path == "/pypi/py-agent/json" {
			_, _ = w.Write([]byte(`{"info":{"version":"1.5.0"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer pypiSrv.Close()

	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/acme/alpha-acp/releases/latest" {
			_, _ = w.Write([]byte(`{"tag_name":"v0.12.0"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ghSrv.Close()

	entries := []agentcatalog.Entry{
		{
			ID:          "alpha",
			Title:       "Alpha",
			Kind:        agentcatalog.KindHybrid,
			Executables: []string{"alpha-cli"},
			Distribution: agentprofile.Distribution{
				Upstream: agentprofile.NPMUpstream("@scope/alpha"),
			},
			Connector: &agentcatalog.ConnectorEntry{
				Executable: "alpha-acp",
				Title:      "alpha-acp",
				Distribution: agentprofile.Distribution{
					Upstream: agentprofile.GitHubReleaseUpstream("acme/alpha-acp"),
				},
			},
		},
		{
			ID:          "py",
			Title:       "Py Agent",
			Kind:        agentcatalog.KindHybrid,
			Executables: []string{"py-agent"},
			Distribution: agentprofile.Distribution{
				Upstream: agentprofile.PyPIUpstream("py-agent"),
			},
		},
		{
			ID:          "redir",
			Title:       "Redirect Agent",
			Kind:        agentcatalog.KindTerminal,
			Executables: []string{"bad-redirect-cli"},
			Distribution: agentprofile.Distribution{
				Upstream: agentprofile.NPMUpstream("redirect-pkg"),
			},
		},
		{
			ID:          "broken",
			Title:       "Broken Agent",
			Kind:        agentcatalog.KindTerminal,
			Executables: []string{"broken-cli"},
			Distribution: agentprofile.Distribution{
				Upstream: agentprofile.NPMUpstream("broken-pkg"),
			},
		},
		{
			ID:          "uninstalled",
			Title:       "Uninstalled Agent",
			Kind:        agentcatalog.KindTerminal,
			Executables: []string{"missing-cli"},
			Distribution: agentprofile.Distribution{
				Upstream: agentprofile.PyPIUpstream("should-not-query"),
			},
		},
	}

	svc := agentcheck.New(agentcheck.Options{
		Entries:       entries,
		ShellPath:     "/bin/sh", // non-wrapped shell -> direct PATH lookup
		Env:           []string{"PATH=" + binDir, "SHELL=/bin/sh"},
		NPMBases:      []string{primaryNPM.URL, mirrorNPM.URL},
		PyPIBase:      pypiSrv.URL,
		GitHubAPIBase: ghSrv.URL,
	})

	rep, err := svc.CheckUpdates(context.Background(), false)
	if err != nil {
		t.Fatalf("CheckUpdates: %v", err)
	}
	if rep.RemoteCheckedAt == nil {
		t.Fatal("RemoteCheckedAt = nil, want timestamp")
	}
	if pypiHits.Load() != 1 {
		t.Errorf("pypiHits = %d, want 1 (uninstalled tool must not be queried)", pypiHits.Load())
	}

	alpha := rep.Tools[0]
	if alpha.UpdateStatus != agentcheck.StatusUpdateAvailable || alpha.LatestVersion != "2.1.320" {
		t.Errorf("alpha = %+v, want update_available 2.1.320 via npm mirror", alpha)
	}
	if alpha.Connector == nil || alpha.Connector.UpdateStatus != agentcheck.StatusUpdateAvailable || alpha.Connector.LatestVersion != "0.12.0" {
		t.Errorf("alpha.Connector = %+v, want update_available 0.12.0 via GitHub release", alpha.Connector)
	}

	py := rep.Tools[1]
	if py.UpdateStatus != agentcheck.StatusUpToDate || py.LatestVersion != "1.5.0" {
		t.Errorf("py = %+v, want up_to_date 1.5.0", py)
	}

	redir := rep.Tools[2]
	if redir.UpdateStatus != agentcheck.StatusCheckFailed || redir.CheckError == "" {
		t.Errorf("redir = %+v, want check_failed on untrusted redirect", redir)
	}

	broken := rep.Tools[3]
	if broken.UpdateStatus != agentcheck.StatusCheckFailed || broken.CheckError == "" {
		t.Errorf("broken = %+v, want check_failed isolated to broken tool", broken)
	}

	uninstalled := rep.Tools[4]
	if uninstalled.UpdateStatus != agentcheck.StatusNotInstalled {
		t.Errorf("uninstalled = %+v, want not_installed", uninstalled)
	}

	// Subsequent Inspect merges unexpired remote cache without hitting servers again.
	cachedInspect, err := svc.Inspect(context.Background(), false)
	if err != nil {
		t.Fatalf("Inspect after CheckUpdates: %v", err)
	}
	if pypiHits.Load() != 1 {
		t.Errorf("Inspect hit PyPI server (%d), want 1", pypiHits.Load())
	}
	if cachedInspect.Tools[0].UpdateStatus != agentcheck.StatusUpdateAvailable {
		t.Errorf("cachedInspect alpha status = %q, want update_available", cachedInspect.Tools[0].UpdateStatus)
	}
}

func TestStartupShellPATHUsedByEnvShimVersionProbe(t *testing.T) {
	home := t.TempDir()
	nvmBin := filepath.Join(home, ".nvm-bin")
	if err := os.MkdirAll(nvmBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("export PATH=\""+nvmBin+":$PATH\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Both the interpreter and the #!/usr/bin/env shim exist ONLY on ~/.bashrc PATH, not service PATH.
	writeExecutable(t, nvmBin, "fake-node-runtime", "#!/bin/sh\necho 'shim-agent 4.5.6'\n")
	shimPath := writeExecutable(t, nvmBin, "shim-agent", "#!/usr/bin/env fake-node-runtime\n")

	svc := agentcheck.New(agentcheck.Options{
		Entries: []agentcatalog.Entry{
			{
				ID:          "shim",
				Title:       "Shim Agent",
				Kind:        agentcatalog.KindHybrid,
				Executables: []string{"shim-agent"},
			},
		},
		ShellPath: "/bin/bash",
		Env:       []string{"HOME=" + home, "PATH=/usr/bin:/bin", "SHELL=/bin/bash"},
	})

	rep, err := svc.Inspect(context.Background(), false)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(rep.Tools) != 1 || !rep.Tools[0].Installed || rep.Tools[0].Path != shimPath || rep.Tools[0].InstalledVersion != "4.5.6" {
		t.Fatalf("shim tool = %+v, want installed at %s v4.5.6", rep.Tools, shimPath)
	}
}

func TestSingleflightSurvivesLeaderCancellationAndPEP440Upstream(t *testing.T) {
	binDir := t.TempDir()
	// Slow --version slightly so leader cancellation happens mid-scan while waiter is joined.
	writeExecutable(t, binDir, "slow-kimi", "#!/bin/sh\nsleep 0.15\necho 'kimi, version 2.5.2'\n")

	startedUpstream := make(chan struct{})
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-startedUpstream:
		default:
			close(startedUpstream)
		}
		time.Sleep(150 * time.Millisecond)
		if r.URL.Path == "/pypi/kimi-cli/json" {
			_, _ = w.Write([]byte(`{"info":{"version":"3.0.0rc1"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer pypiSrv.Close()

	svc := agentcheck.New(agentcheck.Options{
		Entries: []agentcatalog.Entry{
			{
				ID:          "kimi",
				Title:       "Kimi CLI",
				Kind:        agentcatalog.KindHybrid,
				Executables: []string{"slow-kimi"},
				Distribution: agentprofile.Distribution{
					Upstream: agentprofile.PyPIUpstream("kimi-cli"),
				},
			},
		},
		ShellPath: "/bin/sh",
		Env:       []string{"PATH=" + binDir + ":/usr/bin:/bin", "SHELL=/bin/sh"},
		PyPIBase:  pypiSrv.URL,
	})

	// 1. Cold local scan: leader cancels after 30ms; healthy waiter still succeeds.
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	time.AfterFunc(30*time.Millisecond, cancelLeader)

	waiterDone := make(chan struct {
		rep agentcheck.Report
		err error
	}, 1)
	go func() {
		time.Sleep(10 * time.Millisecond)
		r, e := svc.Inspect(context.Background(), false)
		waiterDone <- struct {
			rep agentcheck.Report
			err error
		}{r, e}
	}()

	if _, err := svc.Inspect(leaderCtx, false); err == nil {
		t.Fatal("expected cancelled leader Inspect to return error")
	}
	wRes := <-waiterDone
	if wRes.err != nil {
		t.Fatalf("healthy waiter Inspect failed after leader cancel: %v", wRes.err)
	}
	if len(wRes.rep.Tools) != 1 || wRes.rep.Tools[0].InstalledVersion != "2.5.2" {
		t.Fatalf("healthy waiter Tools = %+v, want v2.5.2", wRes.rep.Tools)
	}

	// 2. Remote check: leader cancels mid-HTTP; healthy waiter gets PEP 440 3.0.0rc1 update_available and cache is populated.
	remoteLeaderCtx, cancelRemoteLeader := context.WithCancel(context.Background())
	remoteWaiterDone := make(chan struct {
		rep agentcheck.Report
		err error
	}, 1)
	go func() {
		<-startedUpstream
		go func() {
			r, e := svc.CheckUpdates(context.Background(), false)
			remoteWaiterDone <- struct {
				rep agentcheck.Report
				err error
			}{r, e}
		}()
		time.Sleep(20 * time.Millisecond)
		cancelRemoteLeader()
	}()

	if _, err := svc.CheckUpdates(remoteLeaderCtx, false); err == nil {
		t.Fatal("expected cancelled leader CheckUpdates to return error")
	}
	rwRes := <-remoteWaiterDone
	if rwRes.err != nil {
		t.Fatalf("healthy waiter CheckUpdates failed after leader cancel: %v", rwRes.err)
	}
	if rwRes.rep.Tools[0].UpdateStatus != agentcheck.StatusUpdateAvailable || rwRes.rep.Tools[0].LatestVersion != "3.0.0rc1" {
		t.Fatalf("healthy waiter remote status = %+v, want update_available 3.0.0rc1", rwRes.rep.Tools[0])
	}
}

func TestFlightBudgetExpiryPreservesPartialResults(t *testing.T) {
	binDir := t.TempDir()
	for i := range 5 {
		name := "fast-agent-" + string(rune('a'+i))
		writeExecutable(t, binDir, name, "#!/bin/sh\necho '1.0.0'\n")
	}
	writeExecutable(t, binDir, "hung-agent", "#!/bin/sh\nsleep 2\necho '1.0.0'\n")

	npmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pkg-a/latest":
			_, _ = w.Write([]byte(`{"version":"1.0.0.post1"}`))
		case "/pkg-b/latest", "/pkg-c/latest", "/pkg-d/latest", "/pkg-e/latest":
			_, _ = w.Write([]byte(`{"version":"1.0.0"}`))
		case "/pkg-hung/latest":
			select {
			case <-r.Context().Done():
			case <-time.After(2 * time.Second):
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer npmSrv.Close()

	entries := []agentcatalog.Entry{
		{ID: "a", Title: "A", Kind: agentcatalog.KindHybrid, Executables: []string{"fast-agent-a"}, Distribution: agentprofile.Distribution{Upstream: agentprofile.NPMUpstream("pkg-a")}},
		{ID: "b", Title: "B", Kind: agentcatalog.KindHybrid, Executables: []string{"fast-agent-b"}, Distribution: agentprofile.Distribution{Upstream: agentprofile.NPMUpstream("pkg-b")}},
		{ID: "c", Title: "C", Kind: agentcatalog.KindHybrid, Executables: []string{"fast-agent-c"}, Distribution: agentprofile.Distribution{Upstream: agentprofile.NPMUpstream("pkg-c")}},
		{ID: "d", Title: "D", Kind: agentcatalog.KindHybrid, Executables: []string{"fast-agent-d"}, Distribution: agentprofile.Distribution{Upstream: agentprofile.NPMUpstream("pkg-d")}},
		{ID: "e", Title: "E", Kind: agentcatalog.KindHybrid, Executables: []string{"fast-agent-e"}, Distribution: agentprofile.Distribution{Upstream: agentprofile.NPMUpstream("pkg-e")}},
		{ID: "hung", Title: "Hung", Kind: agentcatalog.KindHybrid, Executables: []string{"hung-agent"}, Distribution: agentprofile.Distribution{Upstream: agentprofile.NPMUpstream("pkg-hung")}},
	}

	svc := agentcheck.New(agentcheck.Options{
		Entries:        entries,
		ShellPath:      "/bin/sh",
		Env:            []string{"PATH=" + binDir + ":/usr/bin:/bin", "SHELL=/bin/sh"},
		NPMBases:       []string{npmSrv.URL, npmSrv.URL},
		HTTPClient:     &http.Client{Timeout: 500 * time.Millisecond},
		VersionTimeout: 500 * time.Millisecond,
		LocalBudget:    120 * time.Millisecond,
		RemoteBudget:   120 * time.Millisecond,
	})

	rep, err := svc.CheckUpdates(context.Background(), false)
	if err != nil {
		t.Fatalf("CheckUpdates should degrade to per-tool statuses on flight expiry, got err: %v", err)
	}
	if rep.RemoteCheckedAt == nil {
		t.Fatal("RemoteCheckedAt = nil, want timestamp even on partial remote expiry")
	}
	if rep.Tools[0].UpdateStatus != agentcheck.StatusUpdateAvailable || rep.Tools[0].LatestVersion != "1.0.0.post1" {
		t.Fatalf("tool a = %+v, want update_available 1.0.0.post1", rep.Tools[0])
	}
	for i := 1; i <= 4; i++ {
		if rep.Tools[i].UpdateStatus != agentcheck.StatusUpToDate {
			t.Fatalf("tool %d = %+v, want up_to_date preserved", i, rep.Tools[i])
		}
	}
	if !rep.Tools[5].Installed || rep.Tools[5].UpdateStatus != agentcheck.StatusCheckFailed || rep.Tools[5].CheckError == "" {
		t.Fatalf("hung tool = %+v, want installed with check_failed", rep.Tools[5])
	}
}

func TestVersionCacheReusesUnchangedAndTimedOutBinaries(t *testing.T) {
	binDir := t.TempDir()
	hitsFile := filepath.Join(t.TempDir(), "hits")
	fastPath := writeExecutable(t, binDir, "fast-tool", "#!/bin/sh\necho x >> "+hitsFile+"\necho '1.2.3'\n")
	writeExecutable(t, binDir, "hung-tool", "#!/bin/sh\necho h >> "+hitsFile+"\nsleep 10\n")

	now := time.Date(2026, 11, 20, 16, 0, 0, 0, time.UTC)
	svc := agentcheck.New(agentcheck.Options{
		Entries: []agentcatalog.Entry{
			{ID: "fast", Title: "Fast", Kind: agentcatalog.KindHybrid, Executables: []string{"fast-tool"}},
			{ID: "hung", Title: "Hung", Kind: agentcatalog.KindHybrid, Executables: []string{"hung-tool"}},
		},
		ShellPath:      "/bin/sh",
		Env:            []string{"PATH=" + binDir + ":/usr/bin:/bin", "SHELL=/bin/sh"},
		LocalTTL:       30 * time.Second,
		VersionTimeout: 200 * time.Millisecond,
		Now:            func() time.Time { return now },
	})

	// Cold scan probes both binaries once (fast succeeds, hung times out).
	rep, err := svc.Inspect(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Tools[0].InstalledVersion != "1.2.3" || rep.Tools[1].InstalledVersion != "" {
		t.Fatalf("cold scan = %+v", rep.Tools)
	}
	_ = os.Remove(hitsFile)

	// Advance past LocalTTL (30s) but within timeout cache TTL (5m):
	// neither unchanged fast-tool nor timed-out hung-tool should re-exec --version.
	now = now.Add(45 * time.Second)
	start := time.Now()
	rep2, err := svc.Inspect(context.Background(), false)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if rep2.Tools[0].InstalledVersion != "1.2.3" || rep2.Tools[1].InstalledVersion != "" {
		t.Fatalf("warm scan = %+v", rep2.Tools)
	}
	if elapsed >= 150*time.Millisecond {
		t.Fatalf("warm scan took %s; timed-out binary was not cached", elapsed)
	}
	if data, err := os.ReadFile(hitsFile); err == nil && len(data) > 0 {
		t.Fatalf("warm scan unexpectedly re-probed binaries: %q", string(data))
	}

	// Updating the binary on disk changes its size/mtime and invalidates its entry.
	writeExecutable(t, binDir, "fast-tool", "#!/bin/sh\necho x >> "+hitsFile+"\necho '2.0.0-updated'\n")
	now = now.Add(45 * time.Second)
	rep3, err := svc.Inspect(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if rep3.Tools[0].InstalledVersion != "2.0.0-updated" {
		t.Fatalf("updated binary version = %q, want 2.0.0-updated", rep3.Tools[0].InstalledVersion)
	}

	// Explicit refresh=true bypasses version cache and re-probes.
	_ = os.Remove(hitsFile)
	if _, err := svc.Inspect(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(hitsFile); err != nil || len(data) == 0 {
		t.Fatalf("explicit refresh should re-probe binaries")
	}
	_ = fastPath
}

func TestTimedOutBinaryReprobesAsynchronouslyAfterTTL(t *testing.T) {
	binDir := t.TempDir()
	gateFile := filepath.Join(t.TempDir(), "fast-mode")
	// Starts hanging (> VersionTimeout); once gateFile is created, prints 3.4.5 immediately
	// without changing the binary's own stat (realPath, modNS, size).
	writeExecutable(t, binDir, "flaky-tool", "#!/bin/sh\nif [ -f '"+gateFile+"' ]; then echo '3.4.5'; exit 0; fi\nsleep 10\n")

	var curUnix atomic.Int64
	curUnix.Store(time.Date(2026, 11, 20, 16, 0, 0, 0, time.UTC).Unix())

	svc := agentcheck.New(agentcheck.Options{
		Entries: []agentcatalog.Entry{
			{ID: "flaky", Title: "Flaky", Kind: agentcatalog.KindHybrid, Executables: []string{"flaky-tool"}},
		},
		ShellPath:      "/bin/sh",
		Env:            []string{"PATH=" + binDir + ":/usr/bin:/bin", "SHELL=/bin/sh"},
		LocalTTL:       30 * time.Second,
		VersionTimeout: 200 * time.Millisecond,
		Now:            func() time.Time { return time.Unix(curUnix.Load(), 0).UTC() },
	})

	// Cold scan times out on flaky-tool.
	rep, err := svc.Inspect(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Tools[0].InstalledVersion != "" {
		t.Fatalf("cold scan version = %q, want empty", rep.Tools[0].InstalledVersion)
	}

	// Advance past both LocalTTL (30s) and versionTimeoutCacheTTL (5m) while still hanging:
	// Inspect must return promptly without blocking on VersionTimeout.
	curUnix.Add(int64((6 * time.Minute).Seconds()))
	start := time.Now()
	rep2, err := svc.Inspect(context.Background(), false)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if rep2.Tools[0].InstalledVersion != "" {
		t.Fatalf("stale-timeout scan version = %q, want empty", rep2.Tools[0].InstalledVersion)
	}
	if elapsed >= 150*time.Millisecond {
		t.Fatalf("stale-timeout scan blocked for %s, want async re-probe", elapsed)
	}

	// Wait for the background re-probe (200ms timeout) to finish, then enable fast-mode
	// and advance past TTL again so the next async re-probe populates version 3.4.5.
	time.Sleep(300 * time.Millisecond)
	if err := os.WriteFile(gateFile, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	curUnix.Add(int64((6 * time.Minute).Seconds()))
	if _, err := svc.Inspect(context.Background(), false); err != nil {
		t.Fatal(err)
	}

	// Poll until the background re-probe stores 3.4.5 and a fresh LocalTTL window observes it.
	deadline := time.Now().Add(2 * time.Second)
	for {
		curUnix.Add(int64((45 * time.Second).Seconds()))
		rep3, err := svc.Inspect(context.Background(), false)
		if err != nil {
			t.Fatal(err)
		}
		if rep3.Tools[0].InstalledVersion == "3.4.5" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("background re-probe did not populate recovered version; got %q", rep3.Tools[0].InstalledVersion)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
