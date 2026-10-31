package session

import (
	"bytes"
	"testing"
)

func TestMessageRequiresExactlyOnePayload(t *testing.T) {
	setters := []struct {
		name string
		set  func(*Message)
	}{
		{"frame", func(m *Message) { m.Frame = []byte{0} }},
		{"info", func(m *Message) { m.Info = &Info{} }},
		{"exit", func(m *Message) { m.Exit = new(int) }},
		{"position", func(m *Message) { m.Position = &Position{} }},
		{"submission", func(m *Message) { m.Submission = &SubmissionStatus{} }},
		{"control", func(m *Message) { m.Control = &ControlState{} }},
	}
	if (Message{}).Valid() {
		t.Fatal("empty message is valid")
	}
	for i, first := range setters {
		var single Message
		first.set(&single)
		if !single.Valid() {
			t.Errorf("single %s payload is invalid", first.name)
		}
		for _, second := range setters[i+1:] {
			multiple := single
			second.set(&multiple)
			if multiple.Valid() {
				t.Errorf("%s plus %s payloads are valid", first.name, second.name)
			}
		}
	}
	for _, frame := range [][]byte{nil, {}, {2}, {255}} {
		if frameMessage(frame).Valid() {
			t.Errorf("malformed raw frame %x is valid", frame)
		}
	}
}

func TestMessageConstructorsAndSharedOutput(t *testing.T) {
	for _, message := range []Message{
		frameMessage([]byte{1}), infoMessage(Info{}), exitMessage(7),
		positionMessage(Position{}), submissionMessage(SubmissionStatus{}), controlMessage(ControlState{}),
	} {
		if !message.Valid() {
			t.Fatalf("constructor produced an invalid message: %+v", message)
		}
	}
	input := []byte("output")
	message := outputMessage(input)
	input[0] = 'X'
	if !bytes.Equal(message.Frame, []byte("\x00output")) {
		t.Fatal("output message retained the reusable PTY read buffer")
	}
	first := &Subscriber{Out: make(chan Message, 1), Slow: make(chan struct{})}
	second := &Subscriber{Out: make(chan Message, 1), Slow: make(chan struct{})}
	s := &Session{subs: map[*Subscriber]struct{}{first: {}, second: {}}}
	s.broadcastLocked(message)
	for _, sub := range []*Subscriber{first, second} {
		received := <-sub.Out
		if !received.Valid() || &received.Frame[0] != &message.Frame[0] {
			t.Fatal("broadcast copied or changed the shared output frame")
		}
	}
	replay := []byte{1, 'r'}
	if got := frameMessage(replay); &got.Frame[0] != &replay[0] {
		t.Fatal("replay constructor copied an already-prefixed frame")
	}
}

func TestSubscriberQueueBoundsBacklogBytes(t *testing.T) {
	newSub := func() *Subscriber {
		return &Subscriber{Out: make(chan Message, terminalOutputQueueCapacity), Slow: make(chan struct{})}
	}
	s := &Session{}
	frame := func(size int) Message { return frameMessage(make([]byte, size)) }

	// A burst of small PTY reads is far below the byte budget.
	reads := newSub()
	for range 1000 {
		s.sendLocked(reads, outputMessage([]byte("line\r\n")))
	}
	if reads.failed {
		t.Fatal("1000 small reads marked the subscriber slow")
	}

	// An empty queue takes a snapshot larger than the budget; anything
	// queued behind it exceeds the budget.
	snapshot := newSub()
	s.sendLocked(snapshot, frame(terminalOutputQueueBytes+1))
	if snapshot.failed {
		t.Fatal("an empty queue rejected an oversized snapshot")
	}
	s.sendLocked(snapshot, frame(1))
	if !snapshot.failed {
		t.Fatal("a backlog over the byte budget did not mark the subscriber slow")
	}
	select {
	case <-snapshot.Slow:
	default:
		t.Fatal("Slow was not closed")
	}

	// Released bytes make room again.
	drained := newSub()
	s.sendLocked(drained, frame(terminalOutputQueueBytes))
	drained.Release(<-drained.Out)
	s.sendLocked(drained, frame(1))
	s.sendLocked(drained, frame(terminalOutputQueueBytes-1))
	if drained.failed {
		t.Fatal("released bytes still counted against the budget")
	}
}
