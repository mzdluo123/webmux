package hub

import (
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

// The session list shows active sessions only: a hybrid shell that is
// interrupted leaves with removed, a resumed one comes back with created,
// and events about hidden shells are dropped.
func TestActiveViewHidesArchivedShells(t *testing.T) {
	hybrid := func(id, state string) contract.SessionInfo {
		return contract.SessionInfo{ID: id, State: contract.SessionState(state)}
	}
	var v activeView
	snap := v.filter(contract.SessionEvent{Type: "snapshot", Sessions: []contract.SessionInfo{{ID: "plain"}, hybrid("live", "active"), hybrid("gone", "interrupted")}})
	if len(snap.Sessions) != 2 || snap.Sessions[1].ID != "live" {
		t.Fatalf("snapshot %+v", snap.Sessions)
	}
	if _, ok := v.event(contract.SessionEvent{Type: "updated", Session: ptr(hybrid("gone", "interrupted"))}); ok {
		t.Fatal("update of a hidden shell sent")
	}
	if e, ok := v.event(contract.SessionEvent{Type: "updated", Session: ptr(hybrid("live", "interrupted"))}); !ok || e.Type != "removed" || e.ID != "live" {
		t.Fatalf("interrupted: %+v %v", e, ok)
	}
	if e, ok := v.event(contract.SessionEvent{Type: "updated", Session: ptr(hybrid("live", "active"))}); !ok || e.Type != "created" {
		t.Fatalf("active again: %+v %v", e, ok)
	}
	if _, ok := v.event(contract.SessionEvent{Type: "removed", ID: "gone"}); ok {
		t.Fatal("removal of a hidden shell sent")
	}
	if e, ok := v.event(contract.SessionEvent{Type: "removed", ID: "plain"}); !ok || e.ID != "plain" {
		t.Fatalf("plain removal %+v %v", e, ok)
	}
}

func ptr(i contract.SessionInfo) *contract.SessionInfo { return &i }
