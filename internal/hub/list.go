package hub

import (
	"context"
	"encoding/json"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/wire"
)

// Absorb short metadata bursts; a stalled client never blocks publication.
const projectedListQueueCapacity = 256

type projectedList struct {
	Out     chan contract.SessionEvent
	Slow    chan struct{}
	project func(session.Info) contract.SessionInfo
	failed  bool
}

var _ session.ListObserver = (*projectedList)(nil)

func (s *projectedList) Observe(event session.Event) {
	if s.failed {
		return
	}
	value := wire.SessionEvent(event, s.project)
	select {
	case s.Out <- value:
	default:
		s.failed = true
		close(s.Slow)
	}
}

// List registers every contribution before capturing the core snapshot. Plugin
// caches commit before Notify, so a concurrent change is either in this
// snapshot or in the bounded, ordered core subscription (possibly both).
func List(ctx context.Context, m *session.Manager, c MessageConn, contribution Options) {
	ctx, cancel := context.WithCancel(ctx)
	var changes <-chan struct{}
	if contribution.SubscribeForwards != nil {
		var stop func()
		changes, stop = contribution.SubscribeForwards()
		defer stop()
	}
	sub := &projectedList{Out: make(chan contract.SessionEvent, projectedListQueueCapacity), Slow: make(chan struct{}), project: contribution.Project}
	stop := m.Observe(sub)
	defer stop()
	reader := c.CloseRead(ctx)
	defer func() { cancel(); _ = c.CloseNow() }()
	send := func(event contract.SessionEvent) bool {
		data, err := json.Marshal(event)
		if err != nil || Write(ctx, c, MessageText, data) != nil {
			_ = c.Close(StatusSlowClient, "write timeout")
			return false
		}
		return true
	}
	forwards := func() []contract.Forward {
		if contribution.Forwards != nil {
			return contribution.Forwards()
		}
		return []contract.Forward{}
	}
	var view activeView
	snapshot := view.filter(<-sub.Out)
	snapshot.Forwards = forwards()
	if !send(snapshot) {
		return
	}
	for {
		select {
		case <-reader.Done():
			return
		case <-sub.Slow:
			_ = c.Close(StatusSlowClient, "slow client")
			return
		case event := <-sub.Out:
			projected, ok := view.event(event)
			if ok && !send(projected) {
				return
			}
		case <-changes:
			if !send(contract.SessionEvent{Type: "forwards", Forwards: forwards()}) {
				return
			}
		}
	}
}

// activeView filters the host's typed projection, not any plugin's model.
type activeView struct{ hidden map[string]bool }

func active(info contract.SessionInfo) bool {
	return info.State == "" || info.State == contract.SessionStateActive
}

func (v *activeView) filter(e contract.SessionEvent) contract.SessionEvent {
	v.hidden = make(map[string]bool)
	visible := make([]contract.SessionInfo, 0, len(e.Sessions))
	for _, info := range e.Sessions {
		if active(info) {
			visible = append(visible, info)
		} else {
			v.hidden[info.ID] = true
		}
	}
	e.Sessions = visible
	return e
}

func (v *activeView) event(e contract.SessionEvent) (contract.SessionEvent, bool) {
	switch e.Type {
	case "snapshot":
		return v.filter(e), true
	case "removed":
		if v.hidden[e.ID] {
			delete(v.hidden, e.ID)
			return e, false
		}
		return e, true
	}
	if e.Session == nil {
		return e, true
	}
	id := e.Session.ID
	switch {
	case !active(*e.Session) && v.hidden[id]:
		return e, false
	case !active(*e.Session):
		v.hidden[id] = true
		if e.Type == "created" {
			return e, false
		}
		return contract.SessionEvent{Type: "removed", ID: id}, true
	case v.hidden[id]:
		delete(v.hidden, id)
		e.Type = "created"
	}
	return e, true
}
