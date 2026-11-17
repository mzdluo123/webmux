package messaging

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrCredential = errors.New("消息插件身份已失效，请重新打开 Agent 会话")

// Identity is assigned by the host adapter, never by a tool argument.
type Identity struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Profile string `json:"profile"`
}

type Membership struct {
	Identity
	Channel string `json:"channel"`
	Cursor  string `json:"cursor"`
}

type Inbox struct {
	Messages []Message `json:"messages"`
	Through  string    `json:"through"`
}

func validSender(id, name string) bool {
	return (id == "owner" && name == "WebMux") || (strings.HasPrefix(id, "session:") && validSession(strings.TrimPrefix(id, "session:")) && len(name) > 0 && len(name) <= 256 && utf8.ValidString(name))
}
func validSession(id string) bool { return sessionPattern.MatchString(id) }

// Bind returns an ephemeral, session-scoped credential for one ACP adapter.
// Memberships and acknowledgement cursors survive restarts; credentials do not.
func (s *Service) Bind(id, name, profile string) (string, error) {
	identity := Identity{ID: "session:" + id, Name: name, Profile: profile}
	if !validSender(identity.ID, name) || !validSession(profile) {
		return "", ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.unavailableLocked(); err != nil {
		return "", err
	}
	if credential := s.credentials[identity.ID]; credential != "" {
		delete(s.identities, credential)
	}
	if len(s.identities) >= 200 {
		return "", ErrLimit
	}
	credential := rand.Text()
	s.identities[credential] = identity
	s.credentials[identity.ID] = credential
	s.signalLocked()
	return credential, nil
}

func (s *Service) Authenticate(credential string) (Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.unavailableLocked(); err != nil {
		return Identity{}, err
	}
	identity, ok := s.identities[credential]
	if !ok {
		return Identity{}, ErrCredential
	}
	return identity, nil
}

func (s *Service) Members(channel string) ([]Membership, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.unavailableLocked(); err != nil {
		return nil, err
	}
	if !s.hasChannelLocked(channel) {
		return nil, ErrNotFound
	}
	out := []Membership{}
	for _, member := range s.data.Members {
		if member.Channel == channel {
			out = append(out, member)
		}
	}
	return out, nil
}

// Join starts at the current tail. Older context remains available via history.
func (s *Service) Join(identity Identity, channel string) (Membership, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.unavailableLocked(); err != nil {
		return Membership{}, err
	}
	if !s.hasChannelLocked(channel) {
		return Membership{}, ErrNotFound
	}
	for _, member := range s.data.Members {
		if member.ID == identity.ID && member.Channel == channel {
			return member, nil
		}
	}
	if len(s.data.Members) >= 1000 {
		return Membership{}, ErrLimit
	}
	cursor := "0"
	for _, m := range s.data.Messages {
		if m.Channel == channel {
			cursor = m.ID
		}
	}
	member := Membership{Identity: identity, Channel: channel, Cursor: cursor}
	next := s.data
	next.Members = append(slices.Clone(next.Members), member)
	if err := s.commitLocked(next); err != nil {
		return Membership{}, err
	}
	return member, nil
}

func (s *Service) Leave(identity Identity, channel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.unavailableLocked(); err != nil {
		return err
	}
	next := s.data
	next.Members = slices.DeleteFunc(slices.Clone(next.Members), func(m Membership) bool { return m.ID == identity.ID && m.Channel == channel })
	return s.commitLocked(next)
}

func (s *Service) memberLocked(identity Identity, channel string) (int, error) {
	for i, m := range s.data.Members {
		if m.ID == identity.ID && m.Channel == channel {
			return i, nil
		}
	}
	return 0, ErrNotFound
}

func (s *Service) AgentSend(identity Identity, channel, text, requestID string) (Message, error) {
	return s.send(identity, channel, text, requestID, false)
}

func newer(a, b string) bool {
	n, _ := strconv.ParseUint(a, 10, 64)
	p, _ := strconv.ParseUint(b, 10, 64)
	return n > p
}

func (s *Service) inboxLocked(identity Identity, channel string) (Inbox, error) {
	i, err := s.memberLocked(identity, channel)
	if err != nil {
		return Inbox{}, err
	}
	out := Inbox{Messages: []Message{}, Through: s.data.Members[i].Cursor}
	for _, m := range s.data.Messages {
		if m.Channel == channel && m.AgentID != identity.ID && newer(m.ID, out.Through) {
			out.Messages = append(out.Messages, m)
			out.Through = m.ID
			if len(out.Messages) == 100 {
				break
			}
		}
	}
	return out, nil
}

// Read never advances a cursor. Wait uses the same lock for capture and signal.
func (s *Service) Read(ctx context.Context, identity Identity, channel string, wait time.Duration) (Inbox, error) {
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	for {
		s.mu.Lock()
		if err := s.unavailableLocked(); err != nil {
			s.mu.Unlock()
			return Inbox{}, err
		}
		inbox, err := s.inboxLocked(identity, channel)
		changed := s.changed
		s.mu.Unlock()
		if err != nil || len(inbox.Messages) > 0 || wait <= 0 {
			return inbox, err
		}
		select {
		case <-ctx.Done():
			return Inbox{}, ctx.Err()
		case <-deadline.C:
			return inbox, nil
		case <-changed:
		}
	}
}

// Ack advances only through an existing message in this membership's channel.
func (s *Service) Ack(identity Identity, channel, through string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.unavailableLocked(); err != nil {
		return err
	}
	i, err := s.memberLocked(identity, channel)
	if err != nil {
		return err
	}
	if through == s.data.Members[i].Cursor {
		return nil
	}
	found := false
	for _, m := range s.data.Messages {
		if m.Channel == channel && m.ID == through && m.AgentID != identity.ID {
			found = true
			break
		}
	}
	if !found {
		return ErrInvalid
	}
	if !newer(through, s.data.Members[i].Cursor) {
		return nil
	}
	next := s.data
	next.Members = slices.Clone(next.Members)
	next.Members[i].Cursor = through
	return s.commitLocked(next)
}

func (s *Service) signalLocked() { close(s.changed); s.changed = make(chan struct{}) }

func validateMembers(f storedFile) error {
	if len(f.Members) > 1000 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, m := range f.Members {
		key := m.ID + "/" + m.Channel
		if !strings.HasPrefix(m.ID, "session:") || !validSender(m.ID, m.Name) || !validSession(m.Profile) || seen[key] {
			return ErrInvalid
		}
		seen[key] = true
		channel, cursor := false, m.Cursor == "0"
		for _, c := range f.Channels {
			if c.Name == m.Channel {
				channel = true
			}
		}
		for _, msg := range f.Messages {
			if msg.ID == m.Cursor && msg.Channel == m.Channel {
				cursor = true
			}
		}
		if !channel || !cursor {
			return ErrInvalid
		}
	}
	return nil
}

// BoundIdentity returns only a host-bound identity, without its credential.
func (s *Service) BoundIdentity(id string) (Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.unavailableLocked(); err != nil {
		return Identity{}, err
	}
	identity, ok := s.identities[s.credentials["session:"+id]]
	if !ok {
		return Identity{}, ErrNotFound
	}
	return identity, nil
}

// Request sends an explicit request for other subscribed agents to continue.
// Routine agent replies remain readable without starting another round of turns.
func (s *Service) Request(identity Identity, channel, text, requestID string) (Message, error) {
	return s.send(identity, channel, text, requestID, true)
}

// RetainSessions retires runtime credentials for records absent from the host.
// Durable memberships remain available for archived-session recovery.
func (s *Service) RetainSessions(ids []string) error {
	keep := make(map[string]bool, len(ids))
	for _, id := range ids {
		keep["session:"+id] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.unavailableLocked(); err != nil {
		return err
	}
	for id, credential := range s.credentials {
		if !keep[id] {
			delete(s.credentials, id)
			delete(s.identities, credential)
		}
	}
	return nil
}
