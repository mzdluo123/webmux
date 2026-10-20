package hub

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/session"
)

var (
	clientPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)
	generationPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)
)

// AttachOptions reads the optional client identity and resume point from the
// terminal WebSocket query. Malformed values are dropped, never rejected:
// an anonymous client simply observes or replays in full.
func AttachOptions(q url.Values) session.AttachOptions {
	var o session.AttachOptions
	if client := q.Get("client"); clientPattern.MatchString(client) {
		o.Client = client
	}
	label := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == unicode.ReplacementChar {
			return -1
		}
		return r
	}, q.Get("label"))
	if runes := []rune(strings.TrimSpace(label)); len(runes) > 40 {
		o.Label = string(runes[:40])
	} else {
		o.Label = string(runes)
	}
	generation := q.Get("generation")
	offset, err := strconv.ParseUint(q.Get("offset"), 10, 64)
	if err == nil && generationPattern.MatchString(generation) {
		o.Resume = &session.ResumePoint{Generation: generation, Offset: offset}
	}
	return o
}

type submission struct {
	ID         string `json:"id"`
	Generation string `json:"generation"`
	BodyBytes  int    `json:"bodyBytes"`
	body       []byte
	submit     []byte
}

// parseSubmission splits a 0x03 payload: a JSON header line, then the body
// bytes followed by the submit key bytes. Terminal bytes never enter JSON.
func parseSubmission(p []byte) (*submission, bool) {
	header, rest, found := bytes.Cut(p, []byte{'\n'})
	var item submission
	if !found || json.Unmarshal(header, &item) != nil || !clientPattern.MatchString(item.ID) {
		return nil, false
	}
	if item.BodyBytes < 0 || item.BodyBytes > len(rest) || len(rest) == 0 {
		return &item, false
	}
	item.body, item.submit = rest[:item.BodyBytes], rest[item.BodyBytes:]
	return &item, true
}

type input struct {
	data   []byte
	reply  bool
	submit *submission
}

func write(ctx context.Context, c *websocket.Conn, kind websocket.MessageType, p []byte) error {
	deadline, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.Write(deadline, kind, p)
}

// Attach owns the only data writer. Its reader is cancelled and joined on all exits.
func Attach(ctx context.Context, s *session.Session, c *websocket.Conn, o session.AttachOptions) {
	ctx, cancel := context.WithCancel(ctx)
	sub := s.AttachClient(o)
	defer s.Detach(sub)
	c.SetReadLimit(1024 * 1024)
	readerDone := make(chan struct{})
	inputDone := make(chan struct{})
	inputs := make(chan input, 16)
	go func() {
		defer close(inputDone)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case item := <-inputs:
				if item.submit != nil {
					// A received submission finishes even if this connection
					// drops: its status stays queryable, and the PTY write
					// remains bounded by the write deadline and session exit.
					submit := item.submit
					s.Submit(context.WithoutCancel(ctx), sub, submit.ID, submit.Generation, submit.body, submit.submit)
					if ctx.Err() != nil {
						return
					}
				} else if s.WriteInput(ctx, item.data, item.reply) != nil {
					return
				}
			}
		}
	}()
	enqueue := func(item input) bool {
		select {
		case inputs <- item:
			return true
		case <-ctx.Done():
		default:
			cancel()
		}
		return false
	}
	go func() {
		defer close(readerDone)
		defer cancel()
		for {
			kind, p, err := c.Read(ctx)
			if err != nil {
				return
			}
			if kind != websocket.MessageBinary || len(p) == 0 {
				continue
			}
			switch p[0] {
			case 0:
				// User input takes control.
				s.Claim(sub)
				if !enqueue(input{data: p[1:]}) {
					return
				}
			case 6:
				// Every attached xterm answers the same device queries; only
				// the responsible connection's answers reach the PTY, and
				// answering never takes control.
				if s.Responds(sub) && !enqueue(input{data: p[1:], reply: true}) {
					return
				}
			case 1:
				var size struct {
					Cols uint16 `json:"cols"`
					Rows uint16 `json:"rows"`
				}
				// Observers render the controller's grid and never resize it.
				if json.Unmarshal(p[1:], &size) == nil {
					_ = s.ResizeFrom(sub, size.Cols, size.Rows)
				}
			case 2:
				s.Replay(sub)
			case 3:
				item, ok := parseSubmission(p[1:])
				if !ok {
					if item != nil {
						s.SendSubmissionStatus(sub, session.SubmissionStatus{ID: item.ID, State: session.SubmissionFailed, Error: "invalid"})
					}
					continue
				}
				s.Claim(sub)
				if !enqueue(input{submit: item}) {
					return
				}
			case 4:
				var query struct {
					IDs []string `json:"ids"`
				}
				if json.Unmarshal(p[1:], &query) != nil || len(query.IDs) > 64 {
					continue
				}
				ids := query.IDs[:0]
				for _, id := range query.IDs {
					if clientPattern.MatchString(id) {
						ids = append(ids, id)
					}
				}
				s.QuerySubmissions(sub, ids)
			case 5:
				s.Claim(sub)
			}
		}
	}()
	defer func() {
		cancel()
		_ = c.CloseNow()
		<-readerDone
		<-inputDone
	}()
	var pending []byte
	var combinedBuffer []byte
	for {
		var frame []byte
		if pending != nil {
			frame = pending
			pending = nil
		} else {
			select {
			case <-ctx.Done():
				return
			case <-sub.Slow:
				_ = c.Close(4408, "slow client")
				return
			case frame = <-sub.Out:
			}
		}
		if frame[0] == 0 {
			if combinedBuffer == nil {
				combinedBuffer = make([]byte, 0, 65537)
			}
			combined := append(combinedBuffer[:0], frame...)
			timer := time.NewTimer(50 * time.Millisecond)
		aggregate:
			for len(combined) < 65537 {
				select {
				case next := <-sub.Out:
					if next[0] != 0 || len(combined)+len(next)-1 > 65537 {
						pending = next
						break aggregate
					}
					combined = append(combined, next[1:]...)
				case <-timer.C:
					break aggregate
				case <-ctx.Done():
					timer.Stop()
					return
				case <-sub.Slow:
					timer.Stop()
					_ = c.Close(4408, "slow client")
					return
				}
			}
			timer.Stop()
			frame = combined
		}
		if write(ctx, c, websocket.MessageBinary, frame) != nil {
			_ = c.Close(4408, "write timeout")
			return
		}
		if frame[0] == 3 {
			_ = c.Close(4410, "session exited")
			return
		}
	}
}
func List(ctx context.Context, m *session.Manager, c *websocket.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	sub := m.Subscribe()
	defer m.Unsubscribe(sub)
	reader := c.CloseRead(ctx)
	defer func() {
		cancel()
		_ = c.CloseNow()
	}()
	for {
		select {
		case <-reader.Done():
			return
		case <-sub.Slow:
			_ = c.Close(4408, "slow client")
			return
		case event := <-sub.Out:
			data, err := json.Marshal(event)
			if err != nil || write(ctx, c, websocket.MessageText, data) != nil {
				_ = c.Close(4408, "write timeout")
				return
			}
		}
	}
}
