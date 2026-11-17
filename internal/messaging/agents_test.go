package messaging

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func bindTest(t *testing.T, s *Service, id string) (string, Identity) {
	t.Helper()
	credential, err := s.Bind(id, "Codex", "codex")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := s.Authenticate(credential)
	if err != nil {
		t.Fatal(err)
	}
	return credential, identity
}
func joinTest(t *testing.T, s *Service, identity Identity, channel string) {
	t.Helper()
	if _, err := s.Join(identity, channel); err != nil {
		t.Fatal(err)
	}
}
func sendTest(t *testing.T, s *Service, channel, request string) Message {
	t.Helper()
	m, err := s.Send(channel, request, request)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func TestAgentInboxAckPersistenceAndScopedDedup(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	if _, err := s.CreateChannel("general", ""); err != nil {
		t.Fatal(err)
	}
	sendTest(t, s, "general", "before-join")
	credential, identity := bindTest(t, s, "agent-one")
	joinTest(t, s, identity, "general")
	m := sendTest(t, s, "general", "after-join")
	for range 2 {
		inbox, err := s.Read(t.Context(), identity, "general", 0)
		if err != nil || len(inbox.Messages) != 1 || inbox.Through != m.ID {
			t.Fatalf("read lost unacknowledged work: %+v %v", inbox, err)
		}
	}
	response, err := s.AgentSend(identity, "general", "done", "same-request")
	if err != nil || response.AgentID != identity.ID || response.AgentName != "Codex" {
		t.Fatalf("identity: %+v %v", response, err)
	}
	if _, err := s.Send("general", "owner", "same-request"); err != nil {
		t.Fatal("different sender request IDs must be independent", err)
	}
	if err := s.Ack(identity, "general", m.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Ack(identity, "general", response.ID); !errors.Is(err, ErrInvalid) {
		t.Fatal("acknowledged own message", err)
	}
	s.Close()
	s = Open(dir)
	defer s.Close()
	if _, err := s.Authenticate(credential); !errors.Is(err, ErrCredential) {
		t.Fatal("credential survived restart")
	}
	_, identity = bindTest(t, s, "agent-one")
	retry, err := s.AgentSend(identity, "general", "done", "same-request")
	if err != nil || retry != response {
		t.Fatalf("durable dedup: %+v %v", retry, err)
	}
	inbox, err := s.Read(t.Context(), identity, "general", 0)
	if err != nil || len(inbox.Messages) != 1 || inbox.Messages[0].Text != "owner" {
		t.Fatalf("cursor after restart: %+v %v", inbox, err)
	}
	if err := s.Leave(identity, "general"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AgentSend(identity, "general", "oops", "left-request"); !errors.Is(err, ErrNotFound) {
		t.Fatal("send after leaving", err)
	}
}
func TestWaitHasNoArrivalGapAndCancels(t *testing.T) {
	s := openTest(t)
	_, err := s.CreateChannel("general", "")
	if err != nil {
		t.Fatal(err)
	}
	_, identity := bindTest(t, s, "waiter")
	joinTest(t, s, identity, "general")
	result := make(chan Inbox, 1)
	go func() {
		inbox, err := s.Read(t.Context(), identity, "general", time.Second)
		if err != nil {
			t.Error(err)
		}
		result <- inbox
	}()
	m := sendTest(t, s, "general", "wake-waiter")
	select {
	case inbox := <-result:
		if len(inbox.Messages) != 1 || inbox.Through != m.ID {
			t.Fatalf("gap: %+v", inbox)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reader did not wake")
	}
	if err := s.Ack(identity, "general", m.ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.Read(ctx, identity, "general", time.Minute); !errors.Is(err, context.Canceled) {
		t.Fatal("wait cancellation", err)
	}
}
func TestDispatchNewMessagesAndReconnect(t *testing.T) {
	s := openTest(t)
	if _, err := s.CreateChannel("general", ""); err != nil {
		t.Fatal(err)
	}
	credential, identity := bindTest(t, s, "worker")
	joinTest(t, s, identity, "general")
	tasks := make(chan Wake, 10)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx, func(_ context.Context, w Wake) error { tasks <- w; return nil })
	}()
	defer func() { cancel(); <-done }()
	receive := func() Wake {
		t.Helper()
		select {
		case task := <-tasks:
			return task
		case <-time.After(time.Second):
			t.Fatal("no wake")
			return Wake{}
		}
	}
	first := sendTest(t, s, "general", "first-message")
	task := receive()
	if task.SessionID != "worker" || task.Channel != "general" || len(task.RequestID) > 60 {
		t.Fatalf("task: %+v", task)
	}
	sendTest(t, s, "general", "next-message")
	receive()
	select {
	case w := <-tasks:
		t.Fatalf("duplicate wake: %+v", w)
	case <-time.After(30 * time.Millisecond):
	}
	// Acknowledging the delivered batch leaves newer arrivals pending.
	if err := s.Ack(identity, "general", first.ID); err != nil {
		t.Fatal(err)
	}
	receive()
	newCredential, _ := bindTest(t, s, "worker")
	if newCredential == credential {
		t.Fatal("reconnect kept credential")
	}
	if _, err := s.Authenticate(credential); !errors.Is(err, ErrCredential) {
		t.Fatal("old credential still works")
	}
	receive()
	inbox, err := s.Read(t.Context(), identity, "general", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Ack(identity, "general", inbox.Through); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AgentSend(identity, "general", "done", "agent-response"); err != nil {
		t.Fatal(err)
	}
	select {
	case w := <-tasks:
		t.Fatalf("self wake: %+v", w)
	case <-time.After(30 * time.Millisecond):
	}
}
func TestDispatchFailureIsRetriedWithoutSpinning(t *testing.T) {
	s := openTest(t)
	if _, err := s.CreateChannel("general", ""); err != nil {
		t.Fatal(err)
	}
	_, identity := bindTest(t, s, "retry")
	joinTest(t, s, identity, "general")
	sendTest(t, s, "general", "retry-message")
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	attempts := make(chan struct{}, 20)
	go func() {
		defer close(done)
		s.Run(ctx, func(context.Context, Wake) error { attempts <- struct{}{}; return ErrUnavailable })
	}()
	defer func() { cancel(); <-done }()
	select {
	case <-attempts:
	case <-time.After(time.Second):
		t.Fatal("no first attempt")
	}
	select {
	case <-attempts:
		t.Fatal("failed dispatch spun")
	case <-time.After(40 * time.Millisecond):
	}
	select {
	case <-attempts:
	case <-time.After(5 * time.Second):
		t.Fatal("no retry")
	}
}

func TestRoutineRepliesStayQuietAndExplicitRequestsWakePeers(t *testing.T) {
	s := openTest(t)
	if _, err := s.CreateChannel("general", ""); err != nil {
		t.Fatal(err)
	}
	_, sender := bindTest(t, s, "sender")
	_, recipient := bindTest(t, s, "recipient")
	joinTest(t, s, sender, "general")
	joinTest(t, s, recipient, "general")
	tasks := make(chan Wake, 10)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx, func(_ context.Context, w Wake) error { tasks <- w; return nil })
	}()
	defer func() { cancel(); <-done }()
	for i := range 101 {
		if _, err := s.AgentSend(sender, "general", "status update", fmt.Sprintf("routine-update-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case w := <-tasks:
		t.Fatalf("routine reply woke agent: %+v", w)
	case <-time.After(30 * time.Millisecond):
	}
	if _, err := s.Request(sender, "general", "Please take over", "explicit-request"); err != nil {
		t.Fatal(err)
	}
	select {
	case w := <-tasks:
		if w.SessionID != "recipient" {
			t.Fatalf("wake: %+v", w)
		}
	case <-time.After(time.Second):
		t.Fatal("request did not wake peer")
	}
}
