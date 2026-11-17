// Package messaging owns local channels and their persistent message history.
package messaging

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/statefile"
)

var (
	ErrUnavailable = errors.New("消息存储不可用")
	ErrInvalid     = errors.New("频道或消息格式无效")
	ErrNotFound    = errors.New("频道不存在")
	ErrDuplicate   = errors.New("频道名称已存在")
	ErrConflict    = errors.New("消息请求 ID 已用于其他内容")
	ErrLimit       = errors.New("消息存储已达容量上限")
)

const (
	MaxTextBytes = 32000
	maxChannels  = 100
	maxMessages  = 10000
)

var channelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
var requestPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,128}$`)

type Channel struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Topic string `json:"topic"`
}

type Message struct {
	ID              string `json:"id"`
	Channel         string `json:"channel"`
	AgentID         string `json:"agentId"`
	AgentName       string `json:"agentName"`
	Text            string `json:"text"`
	CreatedAt       string `json:"createdAt"`
	ClientRequestID string `json:"clientRequestId"`
}

type storedFile struct {
	V        int       `json:"v"`
	NextID   uint64    `json:"nextId"`
	Channels []Channel `json:"channels"`
	Messages []Message `json:"messages"`
}

// Service serializes snapshots and publishes memory only after a durable commit.
// The store owns its lifetime lock; a corrupt file is preserved for repair.
type Service struct {
	mu     sync.Mutex
	store  *statefile.Handle[storedFile]
	data   storedFile
	broken error
}

func Open(stateDir string) *Service {
	s := &Service{}
	s.store, s.data, s.broken = statefile.Open[storedFile](stateDir, statefile.Messaging, validateStored)
	if statefile.IsKind(s.broken, statefile.Missing) {
		s.broken = nil
		s.data = storedFile{V: 1, NextID: 1, Channels: []Channel{}, Messages: []Message{}}
	}
	return s
}

// Inspect reads and validates the snapshot without creating a file or lock.
func Inspect(stateDir string) error {
	_, err := statefile.ReadSnapshot[storedFile](stateDir, statefile.Messaging, validateStored)
	return err
}

func (s *Service) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store.Close()
	s.broken = ErrUnavailable
}

func (s *Service) Unavailable() error {
	if s == nil {
		return ErrUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unavailableLocked()
}

func (s *Service) unavailableLocked() error {
	if s.broken != nil || s.store.Err() != nil {
		return ErrUnavailable
	}
	return nil
}

func (s *Service) Channels() ([]Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.unavailableLocked(); err != nil {
		return nil, err
	}
	return slices.Clone(s.data.Channels), nil
}

func (s *Service) CreateChannel(name, topic string) (Channel, error) {
	if !channelPattern.MatchString(name) || len(topic) > 1024 || !utf8.ValidString(topic) {
		return Channel{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.unavailableLocked(); err != nil {
		return Channel{}, err
	}
	for _, c := range s.data.Channels {
		if c.Name == name {
			return Channel{}, ErrDuplicate
		}
	}
	if len(s.data.Channels) >= maxChannels {
		return Channel{}, ErrLimit
	}
	c := Channel{ID: strconv.FormatUint(s.data.NextID, 10), Name: name, Topic: topic}
	next := s.data
	next.NextID++
	next.Channels = append(slices.Clip(s.data.Channels), c)
	if err := s.commitLocked(next); err != nil {
		return Channel{}, err
	}
	return c, nil
}

// Messages pages backward using a stable message ID, returning chronological
// order. The cursor must belong to this channel; limit is bounded by the API.
func (s *Service) Messages(channel, before string, limit int) ([]Message, error) {
	if !channelPattern.MatchString(channel) || limit < 1 || limit > 100 {
		return nil, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.unavailableLocked(); err != nil {
		return nil, err
	}
	if !s.hasChannelLocked(channel) {
		return nil, ErrNotFound
	}
	end := len(s.data.Messages)
	if before != "" {
		end = -1
		for i, m := range s.data.Messages {
			if m.ID == before && m.Channel == channel {
				end = i
				break
			}
		}
		if end < 0 {
			return nil, ErrInvalid
		}
	}
	out := make([]Message, 0, limit)
	for i := end - 1; i >= 0 && len(out) < limit; i-- {
		if s.data.Messages[i].Channel == channel {
			out = append(out, s.data.Messages[i])
		}
	}
	slices.Reverse(out)
	return out, nil
}

// Send identifies the authenticated single WebMux owner on the server.
// A repeated request ID returns the original message without another write.
func (s *Service) Send(channel, text, requestID string) (Message, error) {
	if !channelPattern.MatchString(channel) || strings.TrimSpace(text) == "" || len(text) > MaxTextBytes || !utf8.ValidString(text) || !requestPattern.MatchString(requestID) {
		return Message{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.unavailableLocked(); err != nil {
		return Message{}, err
	}
	if !s.hasChannelLocked(channel) {
		return Message{}, ErrNotFound
	}
	for _, m := range s.data.Messages {
		if m.ClientRequestID == requestID {
			if m.Channel != channel || m.Text != text {
				return Message{}, ErrConflict
			}
			return m, nil
		}
	}
	if len(s.data.Messages) >= maxMessages {
		return Message{}, ErrLimit
	}
	m := Message{ID: strconv.FormatUint(s.data.NextID, 10), Channel: channel, AgentID: "owner", AgentName: "WebMux", Text: text, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), ClientRequestID: requestID}
	next := s.data
	next.NextID++
	next.Messages = append(slices.Clip(s.data.Messages), m)
	if err := s.commitLocked(next); err != nil {
		return Message{}, err
	}
	return m, nil
}

func (s *Service) hasChannelLocked(name string) bool {
	for _, c := range s.data.Channels {
		if c.Name == name {
			return true
		}
	}
	return false
}

func (s *Service) commitLocked(next storedFile) error {
	if !s.store.Fits(next) {
		return ErrLimit
	}
	if err := s.store.Commit(next); err != nil {
		return ErrUnavailable
	}
	s.data = next
	return nil
}

func validateStored(f storedFile) error {
	if f.V != 1 || f.NextID == 0 || f.Channels == nil || f.Messages == nil || len(f.Channels) > maxChannels || len(f.Messages) > maxMessages {
		return ErrInvalid
	}
	ids := map[string]bool{}
	channels := map[string]bool{}
	requests := map[string]bool{}
	validID := func(id string) bool {
		n, err := strconv.ParseUint(id, 10, 64)
		if err != nil || n == 0 || n >= f.NextID || strconv.FormatUint(n, 10) != id || ids[id] {
			return false
		}
		ids[id] = true
		return true
	}
	for _, c := range f.Channels {
		if !validID(c.ID) || !channelPattern.MatchString(c.Name) || channels[c.Name] || len(c.Topic) > 1024 {
			return ErrInvalid
		}
		channels[c.Name] = true
	}
	var previous uint64
	for _, m := range f.Messages {
		if !validID(m.ID) || !channels[m.Channel] || m.AgentID != "owner" || m.AgentName != "WebMux" || strings.TrimSpace(m.Text) == "" || len(m.Text) > MaxTextBytes || !requestPattern.MatchString(m.ClientRequestID) || requests[m.ClientRequestID] {
			return ErrInvalid
		}
		n, _ := strconv.ParseUint(m.ID, 10, 64)
		if n <= previous {
			return ErrInvalid
		}
		previous = n
		if _, err := time.Parse(time.RFC3339Nano, m.CreatedAt); err != nil {
			return ErrInvalid
		}
		requests[m.ClientRequestID] = true
	}
	return nil
}
