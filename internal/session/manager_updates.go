package session

import (
	"context"
	"fmt"
	"github.com/rainchan/webmux/internal/procpeek"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func (m *Manager) Subscribe() *ListSubscriber {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub := &ListSubscriber{Out: make(chan Event, sessionListQueueCapacity), Slow: make(chan struct{})}
	sub.Out <- Event{Type: "snapshot", Sessions: m.listLocked()}
	m.subs[sub] = struct{}{}
	return sub
}
func (m *Manager) Unsubscribe(sub *ListSubscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subs, sub)
}

// Observe registers a cache-only observer and captures its initial snapshot
// under the same lock. Publication calls observers without a session lock.
func (m *Manager) Observe(observer ListObserver) func() {
	m.mu.Lock()
	if m.observers == nil {
		m.observers = make(map[ListObserver]struct{})
	}
	m.observers[observer] = struct{}{}
	observer.Observe(Event{Type: "snapshot", Sessions: m.listLocked()})
	m.mu.Unlock()
	return func() {
		m.mu.Lock()
		delete(m.observers, observer)
		m.mu.Unlock()
	}
}
func (m *Manager) publishLocked(e Event) {
	for observer := range m.observers {
		observer.Observe(e)
	}
	for sub := range m.subs {
		if sub.failed {
			continue
		}
		select {
		case sub.Out <- e:
		default:
			sub.failed = true
			close(sub.Slow)
		}
	}
}
func (m *Manager) updates() {
	defer close(m.updatesDone)
	timer := time.NewTicker(500 * time.Millisecond)
	defer timer.Stop()
	previous := make(map[string]Info)
	previewSent := make(map[string]time.Time)
	var snapshot procpeek.Snapshot
	probeTick := false
	for {
		select {
		case <-m.done:
			return
		case now := <-timer.C:
			probeTick = !probeTick
			if probeTick {
				table := procpeek.Processes()
				reapAdopted(table)
				snapshot = procpeek.NewSnapshot(table)
			}
			m.mu.Lock()
			list := make([]*Session, 0, len(m.sessions))
			for _, s := range m.sessions {
				list = append(list, s)
			}
			m.mu.Unlock()
			// Classify every 500 ms using the once-per-second process table.
			// This leaves headroom for delivering the 3-second waiting signal.
			// Classification reads /proc, so it runs without the manager lock.
			infos := make(map[*Session]Info, len(list))
			for _, s := range list {
				s.evaluate(now, snapshot, m.options.IdleAfter, m.patterns)
				infos[s] = s.Info()
			}
			m.mu.Lock()
			for id, s := range m.sessions {
				info, evaluated := infos[s]
				if !evaluated {
					// Created after classification; the next tick covers it.
					continue
				}
				old, known := previous[id]
				if known && now.Sub(previewSent[id]) < 2*time.Second {
					info.Preview = old.Preview
				}
				if !known || !sameDisplay(old, info) {
					m.publishLocked(Event{Type: "updated", Session: &info})
					s.publishState()
					if !known || old.Preview != info.Preview {
						previewSent[id] = now
					}
					previous[id] = info
				}
			}
			for id := range previous {
				if _, exists := m.sessions[id]; !exists {
					delete(previous, id)
					delete(previewSent, id)
				}
			}
			m.mu.Unlock()
		}
	}
}
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	m.stopped = true
	m.mu.Unlock()
	// Wait for an in-flight create to either register or clean up its process
	// before taking the list Shutdown will close.
	m.createMu.Lock()
	m.mu.Lock()
	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s)
	}
	m.mu.Unlock()
	m.createMu.Unlock()
	// Shells are closed without their drivers: their records must survive
	// for recovery after restart.
	for _, s := range list {
		s.Close()
	}
	for _, s := range list {
		s.awaitProcesses(ctx)
		if s.shell {
			s.retire(true)
		}
	}
	reapAdopted(procpeek.Processes())
	m.stopOnce.Do(func() { close(m.done) })
	<-m.updatesDone
}

// Notify publishes a plugin's already-committed metadata change through the
// same bounded, ordered source as process updates. It performs no plugin I/O.
func (m *Manager) Notify(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return
	}
	s.mu.Lock()
	info := s.infoLocked()
	s.broadcastLocked(Message{Info: &info})
	s.mu.Unlock()
	m.publishLocked(Event{Type: "updated", Session: &info})
}

// MaxNameBytes bounds a user-chosen session name, the same limit an OSC title
// is held to.
const MaxNameBytes = 512

// Rename gives session id the user-chosen name, which then outranks terminal
// and agent titles. An empty name hands the title back to them, showing the
// launch directory until one arrives. A name is trimmed and may not hold
// control characters or exceed MaxNameBytes.
func (m *Manager) Rename(id, name string) (Info, error) {
	name = strings.TrimSpace(name)
	if len(name) > MaxNameBytes || !utf8.ValidString(name) || strings.ContainsFunc(name, unicode.IsControl) {
		return Info{}, fmt.Errorf("%w: 名称不能含控制字符或超过 %d 字节", ErrInvalid, MaxNameBytes)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return Info{}, ErrNotFound
	}
	s.mu.Lock()
	s.userTitle = name
	s.info.Title = filepath.Base(s.info.Cwd)
	s.info.PlaceholderTitle = name == ""
	if name != "" {
		s.info.Title = name
	}
	info := s.infoLocked()
	s.broadcastLocked(s.stateLocked())
	s.mu.Unlock()
	m.publishLocked(Event{Type: "updated", Session: &info})
	return info, nil
}
