package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Controller is the browser client whose input and grid drive the PTY. It is
// identified by a client ID that survives that client's reconnects.
type Controller struct {
	ClientID string    `json:"clientId"`
	Label    string    `json:"label"`
	Since    time.Time `json:"since"`
}

// ControlState is the 0x06 frame: who controls the PTY and the grid that
// observers render without resizing it.
type ControlState struct {
	Controller *Controller `json:"controller"`
	Cols       uint16      `json:"cols"`
	Rows       uint16      `json:"rows"`
}

func randomClientID() string {
	var raw [12]byte
	_, _ = rand.Read(raw[:])
	return "anon-" + hex.EncodeToString(raw[:])
}

func (s *Session) controlFrameLocked() []byte {
	state := ControlState{Cols: s.info.Cols, Rows: s.info.Rows}
	if s.control != nil {
		controller := *s.control
		state.Controller = &controller
	}
	return jsonFrame(6, state)
}

func (s *Session) setControlLocked(sub *Subscriber) {
	s.control = &Controller{ClientID: sub.client, Label: sub.label, Since: time.Now()}
	s.broadcastLocked(s.controlFrameLocked())
}

func (s *Session) clientAttachedLocked(client string) bool {
	for sub := range s.subs {
		if sub.client == client {
			return true
		}
	}
	return false
}

// Claim gives control to the subscriber's client and reports whether it changed.
func (s *Session) Claim(sub *Subscriber) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, attached := s.subs[sub]; !attached || s.control != nil && s.control.ClientID == sub.client {
		return false
	}
	s.setControlLocked(sub)
	return true
}

// Controls reports whether the subscriber's client currently holds control.
func (s *Session) Controls(sub *Subscriber) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.control != nil && s.control.ClientID == sub.client
}

// Responds reports whether automatic terminal replies from this subscriber
// reach the PTY: the controller's, or without one the oldest connection's.
// Every attached xterm answers the same queries; only one answer is written.
func (s *Session) Responds(sub *Subscriber) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, attached := s.subs[sub]; !attached {
		return false
	}
	if s.control != nil {
		return s.control.ClientID == sub.client
	}
	for other := range s.subs {
		if other.seq < sub.seq {
			return false
		}
	}
	return true
}

// TerminalReplyOnly reports input made solely of automatic terminal replies.
func TerminalReplyOnly(p []byte) bool { return terminalReplyOnly(p) }

type SubmissionState string

const (
	SubmissionReceived SubmissionState = "received"
	SubmissionWritten  SubmissionState = "written"
	// Partial means some, but not all, submission bytes reached the PTY.
	SubmissionPartial SubmissionState = "partial"
	SubmissionFailed  SubmissionState = "failed"
	// Missing answers a query for an ID this run has no record of: it never
	// arrived, or its record expired.
	SubmissionMissing SubmissionState = "missing"
)

// SubmissionStatus is the 0x05 frame.
type SubmissionStatus struct {
	ID    string          `json:"id"`
	State SubmissionState `json:"state"`
	Error string          `json:"error,omitempty"`
}

const (
	submissionRetention = 30 * time.Minute
	submissionLimit     = 256
)

type submissionRecord struct {
	status SubmissionStatus
	client string
	at     time.Time
}

// submissionLog is a bounded, mu-owned record of recent submissions so that a
// client whose acknowledgement was lost can ask what happened, and so that
// resending the same ID never writes it twice.
type submissionLog struct {
	records map[string]*submissionRecord
	order   []string
}

func (l *submissionLog) get(id string, now time.Time) *submissionRecord {
	l.expire(now)
	return l.records[id]
}

func (l *submissionLog) add(id, client string, now time.Time) *submissionRecord {
	l.expire(now)
	if l.records == nil {
		l.records = make(map[string]*submissionRecord)
	}
	for len(l.order) >= submissionLimit {
		delete(l.records, l.order[0])
		l.order = l.order[1:]
	}
	record := &submissionRecord{status: SubmissionStatus{ID: id, State: SubmissionReceived}, client: client, at: now}
	l.records[id] = record
	l.order = append(l.order, id)
	return record
}

func (l *submissionLog) expire(now time.Time) {
	for len(l.order) > 0 {
		record := l.records[l.order[0]]
		if record != nil && now.Sub(record.at) < submissionRetention {
			return
		}
		delete(l.records, l.order[0])
		l.order = l.order[1:]
	}
}

// Submit writes a body and its submit key as one input critical section, so
// input from other connections cannot interleave between them. A repeated ID
// is never written again; the caller receives the recorded status instead.
// Status frames go to every connection of the submitting client, including
// one that reconnected while the write was in progress.
func (s *Session) Submit(ctx context.Context, sub *Subscriber, id, generation string, body, submit []byte) SubmissionStatus {
	now := time.Now()
	s.mu.Lock()
	if record := s.submissions.get(id, now); record != nil {
		status := record.status
		s.sendLocked(sub, jsonFrame(5, status))
		s.mu.Unlock()
		return status
	}
	if generation != s.info.Generation {
		status := SubmissionStatus{ID: id, State: SubmissionFailed, Error: "generation"}
		s.sendLocked(sub, jsonFrame(5, status))
		s.mu.Unlock()
		return status
	}
	record := s.submissions.add(id, sub.client, now)
	s.notifyClientLocked(record)
	s.mu.Unlock()

	written, err := s.writeParts(ctx, body, submit)
	status := SubmissionStatus{ID: id, State: SubmissionWritten}
	if err != nil {
		status.State, status.Error = SubmissionFailed, "write"
		if written > 0 {
			status.State = SubmissionPartial
		}
	}
	s.mu.Lock()
	record.status = status
	s.notifyClientLocked(record)
	s.mu.Unlock()
	return status
}

func (s *Session) notifyClientLocked(record *submissionRecord) {
	frame := jsonFrame(5, record.status)
	for other := range s.subs {
		if other.client == record.client {
			s.sendLocked(other, frame)
		}
	}
}

// SendSubmissionStatus answers one subscriber without recording anything.
func (s *Session) SendSubmissionStatus(sub *Subscriber, status SubmissionStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendLocked(sub, jsonFrame(5, status))
}

// QuerySubmissions answers the subscriber with the recorded status of each ID.
func (s *Session) QuerySubmissions(sub *Subscriber, ids []string) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		status := SubmissionStatus{ID: id, State: SubmissionMissing}
		if record := s.submissions.get(id, now); record != nil {
			status = record.status
		}
		s.sendLocked(sub, jsonFrame(5, status))
	}
}
