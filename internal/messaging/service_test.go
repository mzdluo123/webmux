package messaging

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func openTest(t *testing.T) *Service {
	t.Helper()
	s := Open(t.TempDir())
	t.Cleanup(s.Close)
	if err := s.Unavailable(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestChannelAndHistoryPersistAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	if _, err := s.CreateChannel("general", "Team"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateChannel("builds", ""); err != nil {
		t.Fatal(err)
	}
	var first Message
	for i := range 65 {
		m, err := s.Send("general", fmt.Sprintf("hello %d", i), fmt.Sprintf("request-%08d", i))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = m
		}
	}
	if _, err := s.Send("builds", "separate channel", "build-request"); err != nil {
		t.Fatal(err)
	}
	latest, err := s.Messages("general", "", 50)
	if err != nil || len(latest) != 50 || latest[0].Text != "hello 15" || latest[49].Text != "hello 64" {
		t.Fatalf("latest: %d, %v", len(latest), err)
	}
	older, err := s.Messages("general", latest[0].ID, 50)
	if err != nil || len(older) != 15 || older[0] != first {
		t.Fatalf("older: %+v %v", older, err)
	}
	if _, err := s.Messages("builds", first.ID, 50); !errors.Is(err, ErrInvalid) {
		t.Fatal("accepted another channel's cursor")
	}
	s.Close()
	s = Open(dir)
	defer s.Close()
	channels, err := s.Channels()
	if err != nil || len(channels) != 2 || channels[0].Topic != "Team" || channels[0].MessageCount != 65 || channels[0].LatestText != "hello 64" {
		t.Fatalf("channels after restart: %+v %v", channels, err)
	}
	retry, err := s.Send("general", first.Text, first.ClientRequestID)
	if err != nil || retry != first {
		t.Fatalf("retry after restart: %+v %v", retry, err)
	}
	if _, err := s.Send("general", "changed", first.ClientRequestID); !errors.Is(err, ErrConflict) {
		t.Fatalf("request ID conflict: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "messaging.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("private snapshot: %v %v", info, err)
	}
}

func TestConcurrentSendsAreDurableAndUnique(t *testing.T) {
	s := openTest(t)
	if _, err := s.CreateChannel("general", ""); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 40 {
		wg.Go(func() {
			if _, err := s.Send("general", fmt.Sprint(i), fmt.Sprintf("concurrent-%08d", i)); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	messages, err := s.Messages("general", "", 100)
	if err != nil || len(messages) != 40 {
		t.Fatalf("messages: %d %v", len(messages), err)
	}
	ids := map[string]bool{}
	for _, m := range messages {
		if ids[m.ID] {
			t.Fatal("duplicate ID")
		}
		ids[m.ID] = true
	}
}

func TestInvalidInputAndUnavailableStore(t *testing.T) {
	s := openTest(t)
	for _, name := range []string{"", "..", "../general", "Uppercase", "space name"} {
		if _, err := s.CreateChannel(name, ""); !errors.Is(err, ErrInvalid) {
			t.Errorf("channel %q: %v", name, err)
		}
	}
	if _, err := s.Send("missing", "hello", "request-id"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.CreateChannel("general", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateChannel("general", ""); !errors.Is(err, ErrDuplicate) {
		t.Fatal(err)
	}
	for _, tt := range []struct{ text, id string }{{" ", "request-id"}, {"valid", "short"}} {
		if _, err := s.Send("general", tt.text, tt.id); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err := s.Messages("general", "", 101); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	s.Close()
	if _, err := s.Channels(); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestCorruptOrLockedStoreIsPreserved(t *testing.T) {
	dir := t.TempDir()
	first := Open(dir)
	defer first.Close()
	second := Open(dir)
	defer second.Close()
	if !errors.Is(second.Unavailable(), ErrUnavailable) {
		t.Fatal("second writer acquired the store")
	}
	first.Close()
	path := filepath.Join(dir, "messaging.json")
	original := []byte(`{"v":99,"channels":[]}`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	broken := Open(dir)
	defer broken.Close()
	if !errors.Is(broken.Unavailable(), ErrUnavailable) {
		t.Fatal("corrupt store was accepted")
	}
	if _, err := broken.CreateChannel("general", ""); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(original) {
		t.Fatal("corrupt store overwritten")
	}
}
