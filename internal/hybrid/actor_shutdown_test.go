package hybrid

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/agent/omp"
)

func TestCloseInFlightPersistsClosedAcrossShutdownAndRestart(t *testing.T) {
	h := newHarness(t, 0)
	h.fast()
	h.svc.timing.interrupt = 5 * time.Second
	s := h.create(h.input(omp.Name, SideTUI, "FAKEAGENT_IGNORE_INTERRUPT=1", "FAKEAGENT_TURN_DELAY=10s"))
	id := s.Info().ID
	origPID := s.Identity().PID
	waitFor(t, "idle terminal", func() bool { return !tuiBusy(s.Info()) })
	typeLine(t, s, strings.Repeat("busy ", 50))
	waitFor(t, "terminal turn", func() bool { return tuiBusy(s.Info()) })

	// Start user close: beginClose blocks in waitIdle(5s).
	if err := h.manager.Close(id); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "actor closing", func() bool {
		a, err := h.svc.actor(id)
		if err != nil {
			return false
		}
		closing, _ := call(ctxT(t), a, func() (bool, error) { return a.closing, nil })
		return closing
	})

	started := time.Now()
	h.svc.Close()
	if elapsed := time.Since(started); elapsed >= 4*time.Second {
		t.Fatalf("Service.Close waited %v instead of cancelling in-flight beginClose", elapsed)
	}

	// Shut down the session manager before reopening a new server instance.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	h.manager.Shutdown(shutdownCtx)
	cancel()

	// Inspect on-disk state written by actor.shutdown before restart recovery.
	beforeRestart := openTestRecords(t, h.state)
	saved, ok := beforeRestart.get(id)
	beforeRestart.close()
	if !ok || saved.State != StateClosed || saved.InterruptedAt != nil || saved.ResumeOnOpen {
		t.Fatalf("saved record during close shutdown = %+v", saved)
	}
	if saved.PID != origPID || saved.Process == nil {
		t.Fatalf("unconfirmed process identity cleared prematurely: %+v (want pid %d)", saved, origPID)
	}

	// Simulate server restart: recoverAfterRestart cleans leftovers and preserves StateClosed.
	h.open(0)
	after := h.record(id)
	if after.State != StateClosed || after.InterruptedAt != nil || after.ResumeOnOpen || after.PID != 0 || after.Process != nil {
		t.Fatalf("record after restart = %+v, want StateClosed without ResumeOnOpen or leftover PID", after)
	}
}

func TestShutdownJoinsDetachedSwitchAndReconnectSubtasks(t *testing.T) {
	h := newHarness(t, 0)
	h.fast()

	// Session 1: mid-switch from TUI to ACP (blocked in delayed session/load).
	swSess := h.create(h.input(omp.Name, SideTUI, "FAKEAGENT_DELAY=session/load=5s"))
	swID := swSess.Info().ID
	tuiTurn(t, swSess, "one")
	h.waitEligible(swID)
	swActor, err := h.svc.actor(swID)
	if err != nil {
		t.Fatal(err)
	}
	swDone := make(chan error, 1)
	go func() {
		_, err := h.svc.Switch(context.Background(), swID, SwitchInput{ClientRequestID: rid(), To: SideACP})
		swDone <- err
	}()
	h.waitPhase(swID, PhaseLoading)

	// Session 2: mid-reconnect after ACP crash (blocked in delayed session/resume or load),
	// with closeAfterStart requested while reopening.
	reSess := h.create(h.input(omp.Name, SideACP, "FAKEAGENT_NO_RESUME=1", "FAKEAGENT_DELAY=session/load=5s"))
	reID := reSess.Info().ID
	reActor, err := h.svc.actor(reID)
	if err != nil {
		t.Fatal(err)
	}
	oldACPPID := h.record(reID).PID
	if err := syscall.Kill(oldACPPID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "actor opening", func() bool {
		opening, _ := call(ctxT(t), reActor, func() (bool, error) { return aOpening(reActor), nil })
		return opening
	})
	if err := h.manager.Close(reID); err != nil {
		t.Fatal(err)
	}

	// In-flight ProbeAgentOptions tracked under s.starting.
	probeDone := make(chan error, 1)
	go func() {
		_, err := h.svc.ProbeAgentOptions(context.Background(), h.input(omp.Name, SideACP, "FAKEAGENT_DELAY=session/new=5s"))
		probeDone <- err
	}()
	time.Sleep(100 * time.Millisecond)

	closeStart := time.Now()
	h.svc.Close()
	if elapsed := time.Since(closeStart); elapsed >= 4500*time.Millisecond {
		t.Fatalf("Service.Close took %v instead of cancelling delayed ACP calls", elapsed)
	}

	// Both detached subtasks and ProbeAgentOptions converged before Service.Close returned.
	for name, a := range map[string]*actor{"switch": swActor, "reopen": reActor} {
		subtasksDone := make(chan struct{})
		go func() {
			a.probes.Wait()
			a.subtasks.Wait()
			close(subtasksDone)
		}()
		select {
		case <-subtasksDone:
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("%s actor subtasks were still running after Service.Close returned", name)
		}
	}
	select {
	case err := <-probeDone:
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("ProbeAgentOptions error on shutdown = %v, want ErrUnavailable", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("ProbeAgentOptions still running after Service.Close returned")
	}
	if err := <-swDone; !errors.Is(err, errStopped) {
		t.Fatalf("in-flight Switch error = %v, want errStopped", err)
	}
	if swSess.Identity().PID != 0 {
		t.Fatalf("rollbackToTUI started a terminal process (%d) during shutdown", swSess.Identity().PID)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	h.manager.Shutdown(shutdownCtx)
	cancel()

	h.open(0)
	if rec := h.record(swID); rec.State != StateInterrupted || rec.Activity != "switching" || !rec.ResumeOnOpen {
		t.Fatalf("switch-interrupted record after restart = %+v", rec)
	}
	if rec := h.record(reID); rec.State != StateClosed || rec.ResumeOnOpen {
		t.Fatalf("closeAfterStart record after restart = %+v, want StateClosed", rec)
	}
}

func aOpening(a *actor) bool { return a.opening }

func TestCreateInterruptedByShutdownCleansRecordAndRestoresEvicted(t *testing.T) {
	h := newHarness(t, 0)
	h.fast()

	// Fill records store to MaxRecords with closed records so the next Create evicts the oldest.
	now := time.Now().Add(-time.Hour).UTC()
	var oldestID string
	for i := range MaxRecords {
		id := fmt.Sprintf("aaaa%04s", base32Index(i))
		if i == 0 {
			oldestID = id
		}
		rec := Record{
			ID:          id,
			ConfigID:    "cfgtest01",
			Agent:       omp.Name,
			Cwd:         t.TempDir(),
			Cmd:         []string{omp.Name},
			Side:        SideACP,
			PreviousIDs: []string{},
			Config:      map[string]string{},
			State:       StateClosed,
			CreatedAt:   now.Add(time.Duration(i) * time.Second),
			UpdatedAt:   now.Add(time.Duration(i) * time.Second),
		}
		if _, err := h.svc.records.add(rec, nil); err != nil {
			t.Fatal(err)
		}
	}

	createDone := make(chan error, 1)
	go func() {
		_, err := h.svc.Create(context.Background(), h.input(omp.Name, SideACP, "FAKEAGENT_DELAY=session/new=5s"))
		createDone <- err
	}()
	waitFor(t, "creating actor registered", func() bool {
		h.svc.mu.Lock()
		defer h.svc.mu.Unlock()
		return len(h.svc.actors) == 1
	})

	h.svc.Close()
	if err := <-createDone; !errors.Is(err, errStopped) {
		t.Fatalf("Create error on shutdown = %v, want errStopped", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	h.manager.Shutdown(shutdownCtx)
	cancel()

	h.open(0)
	if _, ok := h.svc.records.get(oldestID); !ok {
		t.Fatalf("evicted record %s was not restored after Create aborted on shutdown", oldestID)
	}
	for _, rec := range h.svc.records.all() {
		if rec.State != StateClosed {
			t.Fatalf("aborted Create left non-closed record %+v", rec)
		}
	}
}

func base32Index(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz234567"
	var out [4]byte
	for i := 3; i >= 0; i-- {
		out[i] = alphabet[n%32]
		n /= 32
	}
	return string(out[:])
}
