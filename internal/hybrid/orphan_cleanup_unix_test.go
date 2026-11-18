//go:build linux || darwin

package hybrid

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	stdruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/runtime"
)

func TestOrphanCleanupHelperProcess(t *testing.T) {
	role := os.Getenv("WEBMUX_TEST_ORPHAN_ROLE")
	if role == "" {
		return
	}
	switch role {
	case "parent":
		child := exec.Command(os.Args[0], "-test.run=^TestOrphanCleanupHelperProcess$")
		child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		child.Env = append(os.Environ(), "WEBMUX_TEST_ORPHAN_ROLE=detached")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		child.ExtraFiles = []*os.File{os.NewFile(3, "hold")}
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	case "detached":
		// Spawn a grandchild with a cleared environment so only PPID+Start closure reaches it.
		grand := exec.Command(os.Args[0], "-test.run=^TestOrphanCleanupHelperProcess$")
		grand.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		grand.Env = []string{"WEBMUX_TEST_ORPHAN_ROLE=leaf"}
		grand.ExtraFiles = []*os.File{os.NewFile(3, "hold")}
		if err := grand.Start(); err != nil {
			os.Exit(3)
		}
		_, _ = fmt.Fprintf(os.Stdout, "%d %d\n", os.Getpid(), grand.Process.Pid)
		for {
			time.Sleep(time.Second)
		}
	case "leaf":
		for {
			time.Sleep(time.Second)
		}
	case "parent-hold":
		child := exec.Command(os.Args[0], "-test.run=^TestOrphanCleanupHelperProcess$")
		child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		child.Env = append(os.Environ(), "WEBMUX_TEST_ORPHAN_ROLE=detached-ignore-hup")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(4)
		}
		for {
			time.Sleep(time.Second)
		}
	case "detached-ignore-hup":
		signal.Ignore(syscall.SIGHUP)
		_, _ = fmt.Fprintf(os.Stdout, "%d\n", os.Getpid())
		for {
			time.Sleep(time.Second)
		}
	}
}

func spawnDetachedMarkedTree(t *testing.T, sessionID string) (parentPID int, parentIdentity runtime.Identity, detachedPID, leafPID int, closed <-chan struct{}) {
	t.Helper()
	holdR, holdW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	parent := exec.Command(os.Args[0], "-test.run=^TestOrphanCleanupHelperProcess$")
	parent.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	parent.Env = append(os.Environ(),
		contract.SessionIDEnv+"="+sessionID,
		"WEBMUX_TEST_ORPHAN_ROLE=parent",
	)
	parent.Stdout = outW
	parent.Stderr = os.Stderr
	parent.ExtraFiles = []*os.File{holdW}
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	parentPID = parent.Process.Pid
	parentIdentity, err = runtime.ProcessIdentity(parentPID)
	if err != nil {
		// Parent may exit fast; capture at least synthetic non-matching fallback only if needed.
		parentIdentity = runtime.Identity{Start: "exited-parent"}
	}
	_ = outW.Close()
	_ = holdW.Close()
	if err := parent.Wait(); err != nil {
		t.Fatalf("parent helper wait: %v", err)
	}

	line, err := bufio.NewReader(outR).ReadString('\n')
	_ = outR.Close()
	if err != nil {
		t.Fatalf("read detached helper PIDs: %v", err)
	}
	fields := strings.Fields(line)
	if len(fields) != 2 {
		t.Fatalf("unexpected helper output %q", line)
	}
	detachedPID, err = strconv.Atoi(fields[0])
	if err != nil {
		t.Fatal(err)
	}
	leafPID, err = strconv.Atoi(fields[1])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(detachedPID, syscall.SIGKILL)
		_ = syscall.Kill(leafPID, syscall.SIGKILL)
		_ = holdR.Close()
	})
	eof := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, holdR)
		_ = holdR.Close()
		close(eof)
	}()
	return parentPID, parentIdentity, detachedPID, leafPID, eof
}

func TestRecoverAfterRestartCleansMarkedDetachedOrphan(t *testing.T) {
	if stdruntime.GOOS != "linux" {
		t.Skip("environment marker scanning uses /proc/<pid>/environ on Linux")
	}
	parentPID, parentIdentity, detachedPID, leafPID, targetEOF := spawnDetachedMarkedTree(t, "abcdefgh")
	_, _, bystanderPID, bystanderLeafPID, bystanderEOF := spawnDetachedMarkedTree(t, "zzzzzzzz")

	if err := syscall.Kill(detachedPID, 0); err != nil {
		t.Fatalf("detached child not running before recovery: %v", err)
	}
	if err := syscall.Kill(leafPID, 0); err != nil {
		t.Fatalf("detached grandchild not running before recovery: %v", err)
	}

	dir := t.TempDir()
	r := openRecords(dir)
	rec := testRecord("abcdefgh")
	rec.PID, rec.Process = parentPID, &parentIdentity
	if _, err := r.add(rec, nil); err != nil {
		t.Fatal(err)
	}
	r.close()

	r = openTestRecords(t, dir)
	if err := r.recoverAfterRestart(time.Now()); err != nil {
		t.Fatal(err)
	}

	// recoverAfterRestart waits for the orphans to exit before returning, so the
	// inherited pipe write-ends are already closed immediately on return.
	select {
	case <-targetEOF:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("recoverAfterRestart returned before marked detached orphans exited")
	}
	if _, err := runtime.ProcessIdentity(detachedPID); !errors.Is(err, runtime.ErrNoProcess) {
		t.Fatalf("detached orphan PID %d still running: err=%v", detachedPID, err)
	}
	if _, err := runtime.ProcessIdentity(leafPID); !errors.Is(err, runtime.ErrNoProcess) {
		t.Fatalf("env-cleared grandchild PID %d still running: err=%v", leafPID, err)
	}
	select {
	case <-bystanderEOF:
		t.Fatal("bystander session orphan was unexpectedly killed")
	default:
	}
	if _, err := runtime.ProcessIdentity(bystanderPID); err != nil {
		t.Fatalf("bystander detached process died: %v", err)
	}
	if _, err := runtime.ProcessIdentity(bystanderLeafPID); err != nil {
		t.Fatalf("bystander leaf process died: %v", err)
	}

	got, ok := r.get("abcdefgh")
	if !ok || got.State != StateInterrupted || !got.ResumeOnOpen || got.PID != 0 || got.Process != nil {
		t.Fatalf("record after restart = %+v", got)
	}
}

func TestCleanupRecordOrphansSkipsDeadRootGroupOnKillEscalation(t *testing.T) {
	if stdruntime.GOOS != "linux" {
		t.Skip("environment marker scanning uses /proc/<pid>/environ on Linux")
	}

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	parent := exec.Command(os.Args[0], "-test.run=^TestOrphanCleanupHelperProcess$")
	parent.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	parent.Env = append(os.Environ(),
		contract.SessionIDEnv+"=hupescal",
		"WEBMUX_TEST_ORPHAN_ROLE=parent-hold",
	)
	parent.Stdout = outW
	parent.Stderr = os.Stderr
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	_ = outW.Close()
	parentPID := parent.Process.Pid
	parentIdentity, err := runtime.ProcessIdentity(parentPID)
	if err != nil {
		_ = syscall.Kill(-parentPID, syscall.SIGKILL)
		_ = parent.Wait()
		t.Fatalf("ProcessIdentity(%d): %v", parentPID, err)
	}
	parentReaped := make(chan struct{})
	go func() {
		_ = parent.Wait()
		close(parentReaped)
	}()

	line, err := bufio.NewReader(outR).ReadString('\n')
	_ = outR.Close()
	if err != nil {
		_ = syscall.Kill(-parentPID, syscall.SIGKILL)
		<-parentReaped
		t.Fatalf("read detached helper PID: %v", err)
	}
	detachedPID, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		_ = syscall.Kill(-parentPID, syscall.SIGKILL)
		<-parentReaped
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-parentPID, syscall.SIGKILL)
		_ = syscall.Kill(detachedPID, syscall.SIGKILL)
		<-parentReaped
	})

	origHangup, origKill, origPoll, origSignal := orphanHangupWait, orphanKillWait, orphanPollInterval, orphanSignal
	defer func() {
		orphanHangupWait, orphanKillWait, orphanPollInterval, orphanSignal = origHangup, origKill, origPoll, origSignal
	}()
	orphanHangupWait = 50 * time.Millisecond
	orphanKillWait = 50 * time.Millisecond
	orphanPollInterval = 10 * time.Millisecond

	type sigCall struct {
		pid int
		sig syscall.Signal
	}
	var mu sync.Mutex
	var calls []sigCall
	orphanSignal = func(pid int, sig syscall.Signal) error {
		mu.Lock()
		calls = append(calls, sigCall{pid: pid, sig: sig})
		mu.Unlock()
		err := syscall.Kill(pid, sig)
		if pid == -parentPID && sig == syscall.SIGHUP {
			// Ensure parent has exited and been reaped before orphanHangupWait expires.
			<-parentReaped
		}
		return err
	}

	if !cleanupRecordOrphans("hupescal", parentPID, &parentIdentity) {
		t.Fatal("cleanupRecordOrphans returned false after SIGKILL escalation")
	}
	if _, err := runtime.ProcessIdentity(detachedPID); !errors.Is(err, runtime.ErrNoProcess) {
		t.Fatalf("detached SIGHUP-ignoring orphan %d survived cleanup: %v", detachedPID, err)
	}

	mu.Lock()
	defer mu.Unlock()
	sawDetachedKill := false
	for _, c := range calls {
		if c.pid == -parentPID && c.sig == syscall.SIGKILL {
			t.Fatalf("sent blind PGID SIGKILL to already-reaped root PID %d", parentPID)
		}
		if c.pid == detachedPID && c.sig == syscall.SIGKILL {
			sawDetachedKill = true
		}
	}
	if !sawDetachedKill {
		t.Fatalf("expected SIGKILL escalation to detached orphan %d; calls=%+v", detachedPID, calls)
	}
}

func TestRecoverAfterRestartPreservesRecordMarkerWhenTargetSurvives(t *testing.T) {
	if stdruntime.GOOS != "linux" {
		t.Skip("environment marker scanning uses /proc/<pid>/environ on Linux")
	}
	parentPID, parentIdentity, detachedPID, leafPID, targetEOF := spawnDetachedMarkedTree(t, "survives")

	origHangup, origKill, origPoll, origSignal := orphanHangupWait, orphanKillWait, orphanPollInterval, orphanSignal
	defer func() {
		orphanHangupWait, orphanKillWait, orphanPollInterval, orphanSignal = origHangup, origKill, origPoll, origSignal
	}()
	orphanHangupWait = 30 * time.Millisecond
	orphanKillWait = 30 * time.Millisecond
	orphanPollInterval = 10 * time.Millisecond
	orphanSignal = func(int, syscall.Signal) error {
		return syscall.EPERM
	}

	dir := t.TempDir()
	r := openRecords(dir)
	rec := testRecord("survives")
	rec.PID, rec.Process = parentPID, &parentIdentity
	if _, err := r.add(rec, nil); err != nil {
		t.Fatal(err)
	}
	r.close()

	r = openTestRecords(t, dir)
	if err := r.recoverAfterRestart(time.Now()); err != nil {
		t.Fatal(err)
	}
	got, ok := r.get("survives")
	if !ok || got.State != StateInterrupted || got.PID != parentPID || got.Process == nil {
		t.Fatalf("record with surviving orphan cleared root marker prematurely: %+v", got)
	}
	r.close()

	// Once signaling succeeds on a later restart, the preserved marker triggers
	// retry cleanup and clears PID/Process after the orphans exit.
	orphanSignal = syscall.Kill
	r2 := openTestRecords(t, dir)
	if err := r2.recoverAfterRestart(time.Now()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-targetEOF:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("retry recoverAfterRestart did not terminate surviving orphans")
	}
	if _, err := runtime.ProcessIdentity(detachedPID); !errors.Is(err, runtime.ErrNoProcess) {
		t.Fatalf("detached orphan %d still running after retry: %v", detachedPID, err)
	}
	if _, err := runtime.ProcessIdentity(leafPID); !errors.Is(err, runtime.ErrNoProcess) {
		t.Fatalf("leaf orphan %d still running after retry: %v", leafPID, err)
	}
	got2, ok := r2.get("survives")
	if !ok || got2.PID != 0 || got2.Process != nil {
		t.Fatalf("record after successful retry cleanup = %+v", got2)
	}
}
