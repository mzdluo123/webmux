package settings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

func highPortsChange(value bool) Changes {
	return Changes{Fields: map[string]bool{"ignoreHighPorts": true}, Values: Values{IgnoreHighPorts: &value}}
}
func TestDurableRevisionsAndSubscriptions(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	initial, err := s.Snapshot()
	if err != nil || initial.Revision != 0 || initial.Values.IgnoreHighPorts != nil {
		t.Fatalf("initial: %+v %v", initial, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("read created data: %v", err)
	}
	second := Open(dir)
	defer second.Close()
	if _, err := second.Snapshot(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("second writer: %v", err)
	}
	sub, err := s.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	if first := <-sub.Events; first.Snapshot == nil || first.Snapshot.Revision != 0 {
		t.Fatalf("first: %+v", first)
	}
	next, err := s.Patch(0, highPortsChange(true))
	if err != nil || next.Revision != 1 {
		t.Fatalf("patch: %+v %v", next, err)
	}
	if e := <-sub.Events; e.Revision != 1 || !e.Changes.Fields["ignoreHighPorts"] {
		t.Fatalf("event: %+v", e)
	}
	*next.Values.IgnoreHighPorts = false
	if _, err := s.Patch(0, highPortsChange(false)); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	s.Close()
	reopened := Open(dir)
	defer reopened.Close()
	got, err := reopened.Snapshot()
	if err != nil || got.Revision != 1 || got.Values.IgnoreHighPorts == nil || !*got.Values.IgnoreHighPorts {
		t.Fatalf("reopen: %+v %v", got, err)
	}
}
func TestAtomicConcurrentPatch(t *testing.T) {
	s := Open(t.TempDir())
	defer s.Close()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, value := range []bool{true, false} {
		wg.Go(func() { _, err := s.Patch(0, highPortsChange(value)); results <- err })
	}
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflict=%d", successes, conflicts)
	}
}
func TestCorruptionPreserved(t *testing.T) {
	for _, data := range []string{`{"v":1,"revision":0,"values":{},"values":{}}`, `{"v":999}`, `{"v":1,"secret":"do-not-leak"}`, `null`} {
		t.Run(data, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "settings.json")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			s := Open(dir)
			defer s.Close()
			if _, err := s.Patch(0, highPortsChange(true)); !errors.Is(err, ErrUnavailable) {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != data {
				t.Fatalf("changed: %q %v", got, err)
			}
		})
	}
}

// Files written with legacy theme, fontSize, forceSoftKeys or
// chatIdleSleepMinutes keys load cleanly and drop those keys on the next write.
func TestLoadsFileWithLegacySettingsKeys(t *testing.T) {
	dir := t.TempDir()
	data := `{"v":1,"revision":11,"updatedAt":"2026-10-28T02:13:13.53185523Z","values":{"theme":"dark","fontSize":12,"chatIdleSleepMinutes":15,"preview":null,"codeReader":null,"quickCommands":null,"homeWorktrees":null,"forceSoftKeys":true,"ignoreHighPorts":true}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	s := Open(dir)
	defer s.Close()
	v, err := s.Snapshot()
	if err != nil || v.Revision != 11 || v.Values.IgnoreHighPorts == nil || !*v.Values.IgnoreHighPorts {
		t.Fatalf("snapshot: %+v %v", v, err)
	}
	highPorts := false
	if v, err = s.Patch(11, Changes{Fields: map[string]bool{"ignoreHighPorts": true}, Values: Values{IgnoreHighPorts: &highPorts}}); err != nil || v.Values.IgnoreHighPorts == nil || *v.Values.IgnoreHighPorts {
		t.Fatalf("patch: %+v %v", v, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil || strings.Contains(string(raw), "theme") || strings.Contains(string(raw), "fontSize") || strings.Contains(string(raw), "forceSoftKeys") || strings.Contains(string(raw), "chatIdleSleepMinutes") {
		t.Fatalf("legacy keys rewritten: %s %v", raw, err)
	}
}
func TestFailedCommitDoesNotPublish(t *testing.T) {
	s := Open(t.TempDir())
	defer s.Close()
	sub, err := s.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	<-sub.Events
	s.commit = func(storedFile) error { return errors.New("injected disk failure") }
	if _, err := s.Patch(0, highPortsChange(true)); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if s.current.Revision != 0 {
		t.Fatal("uncommitted revision adopted")
	}
	if _, ok := <-sub.Events; ok {
		t.Fatal("uncommitted event")
	}
	if _, err := s.Patch(0, highPortsChange(false)); !errors.Is(err, ErrUnavailable) {
		t.Fatal("writes not frozen")
	}
}
func TestNullAndEmptyDistinct(t *testing.T) {
	s := Open(t.TempDir())
	defer s.Close()
	empty := []Command{}
	v, err := s.Patch(0, Changes{Fields: map[string]bool{"quickCommands": true}, Values: Values{QuickCommands: &empty}})
	if err != nil || v.Values.QuickCommands == nil {
		t.Fatalf("empty: %+v %v", v, err)
	}
	v, err = s.Patch(1, Changes{Fields: map[string]bool{"quickCommands": true}})
	if err != nil || v.Values.QuickCommands != nil {
		t.Fatalf("null: %+v %v", v, err)
	}
}
func TestSlowSubscriberDoesNotBlock(t *testing.T) {
	s := Open(t.TempDir())
	defer s.Close()
	sub, err := s.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	for i := range int64(40) {
		if _, err := s.Patch(i, highPortsChange(true)); err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for range sub.Events {
		count++
	}
	if count != 16 {
		t.Fatalf("queue: %d", count)
	}
}

func TestSubscribeConcurrentCommitHasNoGap(t *testing.T) {
	for range 20 {
		s := Open(t.TempDir())
		start := make(chan struct{})
		subscribed := make(chan *Subscription, 1)
		committed := make(chan error, 1)
		go func() {
			<-start
			sub, err := s.Subscribe()
			if err != nil {
				subscribed <- nil
				return
			}
			subscribed <- sub
		}()
		go func() { <-start; _, err := s.Patch(0, highPortsChange(true)); committed <- err }()
		close(start)
		sub := <-subscribed
		if sub == nil {
			t.Fatal("subscribe failed")
		}
		if err := <-committed; err != nil {
			t.Fatal(err)
		}
		first := <-sub.Events
		if first.Snapshot == nil {
			t.Fatal("first event was not snapshot")
		}
		if first.Snapshot.Revision == 0 {
			select {
			case next := <-sub.Events:
				if next.Revision != 1 {
					t.Fatal("revision skipped")
				}
			default:
				t.Fatal("commit fell in subscription gap")
			}
		} else if first.Snapshot.Revision != 1 {
			t.Fatal("invalid snapshot revision")
		}
		sub.Close()
		s.Close()
	}
}
func TestValidationBoundaries(t *testing.T) {
	for _, p := range []Preview{{0, 0}, {51, 0}, {1, -1}, {1, 51}} {
		if validateValues(Values{Preview: &p}) == nil {
			t.Fatal("preview range accepted")
		}
	}
	for _, p := range []Preview{{1, 0}, {50, 50}} {
		if err := validateValues(Values{Preview: &p}); err != nil {
			t.Fatal(err)
		}
	}
	duplicate := []Command{{ID: "a", Command: "echo a"}, {ID: "a", Command: "echo b"}}
	if validateValues(Values{QuickCommands: &duplicate}) == nil {
		t.Fatal("duplicate command ID")
	}
	if validateValues(Values{HomeWorktrees: map[string][]string{"repo": {"a", "a"}}}) == nil {
		t.Fatal("duplicate worktree")
	}
	if validateValues(Values{WorktreePins: map[string][]string{"repo": {"a", "a"}}}) == nil {
		t.Fatal("duplicate worktree pin")
	}
	if validateValues(Values{WorktreePins: map[string][]string{"repo": nil}}) == nil {
		t.Fatal("null worktree pin array accepted")
	}
	tooManyPins := make([]string, maxWorktreePinsPerRepo+1)
	for i := range tooManyPins {
		tooManyPins[i] = fmt.Sprintf("wt-%d", i)
	}
	if validateValues(Values{WorktreePins: map[string][]string{"repo": tooManyPins}}) == nil {
		t.Fatal("worktree pin repo limit accepted")
	}
	normalizedPins, err := apply(Values{}, Changes{
		Fields: map[string]bool{"worktreePins": true},
		Values: Values{WorktreePins: map[string][]string{"r1": {"wt-2", "wt-1"}, "r2": {}}},
	})
	if err != nil || len(normalizedPins.WorktreePins) != 1 || !slices.Equal(normalizedPins.WorktreePins["r1"], []string{"wt-2", "wt-1"}) {
		t.Fatalf("worktree pin normalization: %+v, err=%v", normalizedPins.WorktreePins, err)
	}
	clearedPins, err := apply(normalizedPins, Changes{
		Fields: map[string]bool{"worktreePins": true},
		Values: Values{WorktreePins: map[string][]string{"r1": {}}},
	})
	if err != nil || clearedPins.WorktreePins != nil {
		t.Fatalf("empty worktree pin map should normalize to nil: %+v, err=%v", clearedPins.WorktreePins, err)
	}
	list := []Command{{ID: "a", Name: "  label  ", Command: "echo a\r\n\n"}}
	v, err := apply(Values{}, Changes{Fields: map[string]bool{"quickCommands": true}, Values: Values{QuickCommands: &list}})
	if err != nil || (*v.QuickCommands)[0].Command != "echo a" || (*v.QuickCommands)[0].Name != "label" {
		t.Fatal("normalization")
	}
}

// A quick command needs a unique name that a {{name}} reference can hold,
// checked when the list is saved. Commands stored before names were
// required are named from their text when the file loads, and a file with
// the retired artifactPrompt key loads and drops the key on the next write.
func TestQuickCommandNames(t *testing.T) {
	set := func(list ...Command) Changes {
		return Changes{Fields: map[string]bool{"quickCommands": true}, Values: Values{QuickCommands: &list}}
	}
	for name, list := range map[string][]Command{
		"empty":     {{ID: "a", Name: " ", Command: "x"}},
		"duplicate": {{ID: "a", Name: "同名", Command: "x"}, {ID: "b", Name: "同名", Command: "y"}},
		"brace":     {{ID: "a", Name: "a}b", Command: "x"}},
		"newline":   {{ID: "a", Name: "a\nb", Command: "x"}},
		"reserved":  {{ID: "a", Name: "user", Command: "x"}},
		"reserved2": {{ID: "a", Name: "webmux", Command: "x"}},
	} {
		if _, err := apply(Values{}, set(list...)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s name accepted: %v", name, err)
		}
	}
	if _, err := apply(Values{}, set(Command{ID: "a", Name: "临时文件夹", Command: "x"}, Command{ID: "b", Name: "Plan", Command: "y"})); err != nil {
		t.Fatalf("valid names: %v", err)
	}

	dir := t.TempDir()
	data := `{"v":1,"revision":3,"updatedAt":"2026-10-28T02:13:13.53185523Z","values":{"theme":"dark","preview":null,"codeReader":null,"quickCommands":[{"id":"a","name":"","command":"/resume","enter":true},{"id":"b","name":"","command":"/resume\n再来","enter":true},{"id":"c","name":"同名","command":"x","enter":false},{"id":"d","name":"同名","command":"放 {dir} 与 {{dir}}","enter":false},{"id":"e","name":"dir","command":"z","enter":false}],"homeWorktrees":null,"forceSoftKeys":null,"artifactPrompt":"看 {dir}"}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	s := Open(dir)
	defer s.Close()
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range *snap.Values.QuickCommands {
		names = append(names, c.Name)
	}
	if want := []string{"/resume", "/resume 2", "同名", "同名 2", "dir 2"}; !slices.Equal(names, want) {
		t.Fatalf("legacy names %q, want %q", names, want)
	}
	if got := (*snap.Values.QuickCommands)[3].Command; got != "放 {{dir}} 与 {{dir}}" {
		t.Fatalf("legacy placeholder %q", got)
	}
	highPorts := true
	if _, err := s.Patch(3, Changes{Fields: map[string]bool{"ignoreHighPorts": true}, Values: Values{IgnoreHighPorts: &highPorts}}); err != nil {
		t.Fatalf("unrelated patch with a legacy unnamed command: %v", err)
	}
	if stored, _ := os.ReadFile(filepath.Join(dir, "settings.json")); strings.Contains(string(stored), "artifactPrompt") || strings.Contains(string(stored), "theme") || strings.Contains(string(stored), "forceSoftKeys") {
		t.Fatalf("retired key written back: %s", stored)
	}
}

func TestBrowserCompatibleStringLimits(t *testing.T) {
	if !validText(strings.Repeat("😀", 20), 40) || validText(strings.Repeat("😀", 21), 40) {
		t.Fatal("UTF-16 limit differs from browser contract")
	}
	if validateValues(Values{HomeWorktrees: map[string][]string{"repo": nil}}) == nil {
		t.Fatal("null worktree array accepted")
	}
}

func TestPinHomeWorktreesSeedsDefaultMainAndPreservesExplicitEmptyEntry(t *testing.T) {
	s := Open(t.TempDir())
	defer s.Close()
	sub, err := s.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	<-sub.Events // initial snapshot

	if err := s.PinHomeWorktrees("r1", "main-1", []string{"wt-a", "wt-a"}); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"main-1", "wt-a"}; !slices.Equal(snap.Values.HomeWorktrees["r1"], want) || snap.Revision != 1 {
		t.Fatalf("got %v rev %d, want %v rev 1", snap.Values.HomeWorktrees["r1"], snap.Revision, want)
	}
	ev := <-sub.Events
	if ev.Revision != 1 || !ev.Changes.Fields["homeWorktrees"] {
		t.Fatalf("unexpected subscription event: %+v", ev)
	}

	// Re-pinning an already-pinned worktree is a no-op.
	if err := s.PinHomeWorktrees("r1", "main-1", []string{"wt-a"}); err != nil {
		t.Fatal(err)
	}
	if snap2, _ := s.Snapshot(); snap2.Revision != 1 {
		t.Fatalf("duplicate pin bumped revision to %d", snap2.Revision)
	}

	// If user explicitly unchecked all worktrees (including main) on r2, pinning a new
	// external worktree does not resurrect main.
	if _, err := s.Patch(1, Changes{
		Fields: map[string]bool{"homeWorktrees": true},
		Values: Values{HomeWorktrees: map[string][]string{"r1": {"main-1"}, "r2": {}}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.PinHomeWorktrees("r2", "main-2", []string{"wt-b"}); err != nil {
		t.Fatal(err)
	}
	snap3, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"wt-b"}; !slices.Equal(snap3.Values.HomeWorktrees["r2"], want) {
		t.Fatalf("r2 got %v, want %v", snap3.Values.HomeWorktrees["r2"], want)
	}
}

func TestRetainAndDropWorktreePinsConvergeOnlyOnStaleEntries(t *testing.T) {
	s := Open(t.TempDir())
	defer s.Close()

	// Empty worktreePins: pruning and dropping are no-ops without bumping revision.
	if err := s.RetainWorktreePins("r1", []string{"wt-a"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DropRepositoryWorktreePins("r1"); err != nil {
		t.Fatal(err)
	}
	if snap, _ := s.Snapshot(); snap.Revision != 0 {
		t.Fatalf("expected revision 0, got %d", snap.Revision)
	}

	if _, err := s.Patch(0, Changes{
		Fields: map[string]bool{"worktreePins": true},
		Values: Values{WorktreePins: map[string][]string{
			"r1": {"wt-2", "wt-gone", "wt-1"},
			"r2": {"wt-x"},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	// All pinned IDs still present: no revision bump.
	if err := s.RetainWorktreePins("r2", []string{"wt-x", "wt-y"}); err != nil {
		t.Fatal(err)
	}
	if snap, _ := s.Snapshot(); snap.Revision != 1 {
		t.Fatalf("expected unchanged revision 1, got %d", snap.Revision)
	}

	// Stale ID removed while preserving saved order and sibling repositories.
	if err := s.RetainWorktreePins("r1", []string{"wt-1", "wt-2", "wt-other"}); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Revision != 2 || !slices.Equal(snap.Values.WorktreePins["r1"], []string{"wt-2", "wt-1"}) || !slices.Equal(snap.Values.WorktreePins["r2"], []string{"wt-x"}) {
		t.Fatalf("unexpected worktreePins after retain: rev=%d pins=%+v", snap.Revision, snap.Values.WorktreePins)
	}

	// Dropping repositories converges empty map to nil.
	if err := s.DropRepositoryWorktreePins("r1"); err != nil {
		t.Fatal(err)
	}
	if err := s.DropRepositoryWorktreePins("r2"); err != nil {
		t.Fatal(err)
	}
	finalSnap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if finalSnap.Revision != 4 || finalSnap.Values.WorktreePins != nil {
		t.Fatalf("expected nil worktreePins at rev 4, got rev=%d pins=%+v", finalSnap.Revision, finalSnap.Values.WorktreePins)
	}
}
