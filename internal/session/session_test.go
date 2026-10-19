package session

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPersistentPTY(t *testing.T) {
	m := NewManager(Options{DefaultCwd: t.TempDir(), Presets: []Preset{{Name: "shell", Cmd: []string{"/bin/bash", "--noprofile", "--norc", "-i"}}}})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	}()
	s, err := m.Create("", "shell", nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	sub := s.Attach()
	moved := false
	deadline := time.After(5 * time.Second)
	if err = s.Write([]byte("printf '\\033]2;agent title\\007'; printf 'consumer-marker\\n'; exit 7\n")); err != nil {
		t.Fatal(err)
	}
	for !moved {
		select {
		case frame := <-sub.Out:
			if frame[0] == 3 {
				moved = true
			}
		case <-deadline:
			t.Fatal("PTY failed to exit")
		}
	}
	s.Detach(sub)
	if got := s.Info(); got.Title != "agent title" || got.ExitCode == nil || *got.ExitCode != 7 {
		t.Fatalf("info: %+v", got)
	}
	again := s.Attach()
	defer s.Detach(again)
	if position := <-again.Out; position[0] != 4 {
		t.Fatalf("position: %q", position)
	}
	snapshot := <-again.Out
	if snapshot[0] != 1 || !strings.Contains(string(snapshot[1:]), "consumer-marker") {
		t.Fatalf("replay: %q", snapshot)
	}
	if err = m.Remove(s.Info().ID); err != nil {
		t.Fatal(err)
	}
}
