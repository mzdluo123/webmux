package messaging

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// Wake is a host-owned task; channel content stays in the inbox tools.
type Wake struct{ SessionID, Channel, RequestID string }

// Run wakes each subscribed session for new requests and binding changes.
// Host acceptance means queued or started, not processed. A reconnect reoffers
// unacknowledged work; acknowledgements are durable and processing is at least once.
func (s *Service) Run(ctx context.Context, wake func(context.Context, Wake) error) {
	var workers sync.WaitGroup
	defer workers.Wait()
	completed := make(chan string, 200)
	inFlight := map[string]bool{}
	sent := map[string]bool{}
	retryAt := map[string]time.Time{}
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		s.mu.Lock()
		changed := s.changed
		if s.unavailableLocked() != nil {
			s.mu.Unlock()
			return
		}
		live := map[string]bool{}
		for _, member := range s.data.Members {
			credential := s.credentials[member.ID]
			if credential == "" {
				continue
			}
			notify := ""
			// Locate notifications independently of the paginated inbox: a long
			// backlog of quiet status updates must not hide a newer task.
			for i := len(s.data.Messages) - 1; i >= 0; i-- {
				message := s.data.Messages[i]
				if !newer(message.ID, member.Cursor) {
					break
				}
				if message.Channel == member.Channel && message.AgentID != member.ID && (message.AgentID == "owner" || message.WakeAgents) {
					notify = message.ID
					break
				}
			}
			if notify == "" {
				continue
			}
			key := fmt.Sprintf("%x", sha256.Sum256([]byte(credential+"/"+member.Channel+"/"+member.Cursor+"/"+notify)))[:40]
			live[key] = true
			if sent[key] || inFlight[key] || time.Now().Before(retryAt[key]) || len(inFlight) >= 200 {
				continue
			}
			inFlight[key] = true
			task := Wake{SessionID: member.ID[len("session:"):], Channel: member.Channel, RequestID: "message-" + key}
			workers.Add(1)
			go func() {
				defer workers.Done()
				callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				err := wake(callCtx, task)
				cancel()
				if err != nil {
					key = "!" + key
				}
				select {
				case completed <- key:
				case <-ctx.Done():
				}
			}()
		}
		s.mu.Unlock()
		for key := range sent {
			if !live[key] {
				delete(sent, key)
				delete(retryAt, key)
			}
		}
		for key := range retryAt {
			if !live[key] && !inFlight[key] {
				delete(retryAt, key)
			}
		}
		select {
		case <-ctx.Done():
			return
		case key := <-completed:
			if key[0] == '!' {
				delete(inFlight, key[1:])
				retryAt[key[1:]] = time.Now().Add(2 * time.Second)
			} else {
				delete(inFlight, key)
				sent[key] = true
			}
		case <-changed:
		case <-tick.C:
		}
	}
}
