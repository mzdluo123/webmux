//go:build linux || darwin

package session

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/procpeek"
)

func waitSidecarDone(t *testing.T, c *Sidecar) {
	t.Helper()
	select {
	case <-c.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("sidecar did not finish")
	}
}

func TestSidecarPipesAndStop(t *testing.T) {
	t.Parallel()
	m := shellManager(t)
	s := createShell(t, m, nil, "/bin/sleep", "30")
	sidecar, err := s.StartSidecar([]string{"/bin/sh", "-c", `echo "$WEBMUX_SESSION_ID $PWD" >&2; exec cat`}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(sidecar.Stdin(), "ping\n"); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(sidecar.Stdout()).ReadString('\n')
	if err != nil || line != "ping\n" {
		t.Fatalf("stdout %q, %v", line, err)
	}
	if code := sidecar.ExitCode(); code != -1 {
		t.Fatalf("exit code %d before Done", code)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = sidecar.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if code := sidecar.ExitCode(); code != 0 {
		t.Fatalf("cat after stdin closed: exit %d", code)
	}
	info := s.Info()
	cwd, _ := filepath.EvalSymlinks(info.Cwd)
	if stderr := strings.TrimSpace(string(sidecar.Stderr())); stderr != info.ID+" "+info.Cwd && stderr != info.ID+" "+cwd {
		t.Fatalf("sidecar environment %q, want session %s in %s", stderr, info.ID, info.Cwd)
	}
	// Stopping a sidecar follows its own tree, not the shared session marker.
	if info.Status == StatusExited {
		t.Fatal("stopping the sidecar ended the PTY run")
	}
	if err = m.Remove(info.ID); err != ErrRunning {
		t.Fatalf("remove with a live run: %v", err)
	}
}

func TestShutdownKillsSidecars(t *testing.T) {
	t.Parallel()
	m := NewManager(Options{DefaultCwd: t.TempDir()})
	s := createShell(t, m, nil)
	sidecar, err := s.StartSidecar([]string{"/bin/sh", "-c", "sleep 60 & echo $!; wait"}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(sidecar.Stdout()).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	child, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Remove(s.Info().ID); err != ErrRunning {
		t.Fatalf("remove with a live sidecar: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m.Shutdown(ctx)
	waitSidecarDone(t, sidecar)
	for _, pid := range []int{sidecar.pid, child} {
		deadline := time.Now().Add(2 * time.Second)
		for !processGone(pid) {
			if time.Now().After(deadline) {
				t.Fatalf("sidecar process %d survived shutdown", pid)
			}
			reapAdopted(procpeek.Processes())
			time.Sleep(20 * time.Millisecond)
		}
	}
	if _, err = s.StartSidecar([]string{"/bin/true"}, false, nil); err != ErrClosed {
		t.Fatalf("sidecar after shutdown: %v", err)
	}
}

func TestSidecarStartupFileOutputStaysOffProtocol(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			t.Parallel()
			path, err := exec.LookPath(shell)
			if err != nil {
				t.Skip(shell + " is not installed")
			}
			home := t.TempDir()
			rc := filepath.Join(home, "."+shell+"rc")
			if err = os.WriteFile(rc, []byte("echo STARTUP_NOISE\nexport FROM_STARTUP=1\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			m := shellManager(t)
			s, err := m.CreateShell(ShellInput{CreateInput: CreateInput{Env: map[string]string{"HOME": home}}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			sidecar, err := s.StartSidecar([]string{path, "-ic", `exec -- "$@" 1>&3 3>&-`, "webmux", "/bin/sh", "-c", `printf "PROTOCOL:$FROM_STARTUP"`}, true, nil)
			if err != nil {
				t.Fatal(err)
			}
			protocol, err := io.ReadAll(sidecar.Stdout())
			if err != nil {
				t.Fatal(err)
			}
			waitSidecarDone(t, sidecar)
			if string(protocol) != "PROTOCOL:1" {
				t.Fatalf("protocol stdout %q", protocol)
			}
			if !strings.Contains(string(sidecar.Stderr()), "STARTUP_NOISE") {
				t.Fatalf("%s did not run or its output was lost: %q", rc, sidecar.Stderr())
			}
			if code := sidecar.ExitCode(); code != 0 {
				t.Fatalf("exit %d", code)
			}
		})
	}
}

func TestSidecarStderrIsBounded(t *testing.T) {
	t.Parallel()
	m := shellManager(t)
	s := createShell(t, m, nil)
	sidecar, err := s.StartSidecar([]string{"/bin/sh", "-c", "head -c 200000 /dev/zero | tr '\\0' e >&2; printf END >&2"}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	waitSidecarDone(t, sidecar)
	stderr := sidecar.Stderr()
	if len(stderr) != sidecarStderrCapacity || !strings.HasSuffix(string(stderr), "END") {
		t.Fatalf("stderr tail: %d bytes, suffix %q", len(stderr), stderr[max(0, len(stderr)-3):])
	}
}

func TestSetsidHelper(t *testing.T) {
	if os.Getenv("WEBMUX_TEST_SETSID") != "1" {
		return
	}
	_, _ = syscall.Setsid()
	var child *exec.Cmd
	if os.Getenv("WEBMUX_TEST_SPAWN_UNMARKED") == "1" {
		child = exec.Command("/bin/sleep", "60")
		child.Env = []string{}
		if err := child.Start(); err == nil {
			if unmarkedFile := os.Getenv("WEBMUX_TEST_UNMARKED_PIDFILE"); unmarkedFile != "" {
				_ = os.WriteFile(unmarkedFile, []byte(strconv.Itoa(child.Process.Pid)), 0o600)
			}
		}
	}
	if pidFile := os.Getenv("WEBMUX_TEST_PIDFILE"); pidFile != "" {
		_ = os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600)
	}
	time.Sleep(60 * time.Second)
}

func waitForPIDFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if raw, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(raw))) > 0 {
			pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
			if err == nil && pid > 0 {
				return pid
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("pid file %s was never written", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestOutputReturnsWhenDetachedSetsidHoldsStdout(t *testing.T) {
	t.Parallel()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(t.TempDir(), "detached.pid")
	m := shellManager(t)
	s := createShell(t, m, nil)
	t.Cleanup(func() {
		if raw, readErr := os.ReadFile(pidFile); readErr == nil {
			if pid, convErr := strconv.Atoi(strings.TrimSpace(string(raw))); convErr == nil && pid > 0 {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	script := `(WEBMUX_TEST_SETSID=1 WEBMUX_TEST_PIDFILE="$1" "$2" -test.run=^TestSetsidHelper$ &) ; while [ ! -s "$1" ]; do sleep 0.01; done; printf DETACHED_OK`
	out, code, err := s.Output(ctx, []string{"/bin/sh", "-c", script, "sh", pidFile, exe}, 1024)
	if err != nil {
		t.Fatalf("Output error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("Output took %v with inherited stdout", elapsed)
	}
	if string(out) != "DETACHED_OK" || code != 0 {
		t.Fatalf("Output = %q, code %d", out, code)
	}
}

func TestStopTerminatesDirectSetsidSidecarChild(t *testing.T) {
	t.Parallel()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(t.TempDir(), "setsid-child.pid")
	m := shellManager(t)
	s := createShell(t, m, nil)
	script := `WEBMUX_TEST_SETSID=1 WEBMUX_TEST_PIDFILE="$1" "$2" -test.run=^TestSetsidHelper$ & wait`
	sidecar, err := s.StartSidecar([]string{"/bin/sh", "-c", script, "sh", pidFile, exe}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	childPID := waitForPIDFile(t, pidFile)
	t.Cleanup(func() { _ = syscall.Kill(childPID, syscall.SIGKILL) })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sidecar.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !processGone(childPID) {
		if time.Now().After(deadline) {
			t.Fatalf("direct setsid child %d survived sidecar.Stop", childPID)
		}
		reapAdopted(procpeek.Processes())
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSidecarHUPHelper(t *testing.T) {
	if os.Getenv("WEBMUX_TEST_SIDECAR_HUP") != "1" {
		return
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	_, _ = os.Stdout.WriteString("ready\n")
	<-ch
	os.Exit(0)
}

func TestSidecarStopDoesNotWaitSecondsWhenStdinEOFIgnored(t *testing.T) {
	t.Parallel()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m := shellManager(t)
	s := createShell(t, m, nil)
	sidecar, err := s.StartSidecar(
		[]string{exe, "-test.run=^TestSidecarHUPHelper$"},
		false,
		map[string]string{"WEBMUX_TEST_SIDECAR_HUP": "1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(sidecar.Stdout()).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "ready" {
		t.Fatalf("helper ready line = %q, %v", line, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	if err := sidecar.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 1500*time.Millisecond {
		t.Fatalf("Sidecar.Stop waited %v for a process ignoring stdin EOF, want < 1.5s", elapsed)
	}
}
