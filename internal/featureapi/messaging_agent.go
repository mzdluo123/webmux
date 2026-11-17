package featureapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/featurewire"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/messaging"
	"github.com/rainchan/webmux/internal/transport"
)

func (s *Server) messagingAgentRoutes(mux transport.Registrar) {
	s.messagingInviteRoutes(mux)
	mux.HandleFunc("GET /api/messaging/channels/{channel}/members", func(w http.ResponseWriter, r *http.Request) {
		if !s.messagingReady(w) {
			return
		}
		items, err := s.Messaging.Members(r.PathValue("channel"))
		if err != nil {
			messagingFailure(w, err)
			return
		}
		reply(w, http.StatusOK, featurewire.MessagingMembers(items))
	})
	mux.HandleFunc("POST /api/messaging/agent", func(w http.ResponseWriter, r *http.Request) {
		if !transport.IsLocal(r.Context()) {
			failure(w, http.StatusForbidden, errors.New("消息插件使用本机 socket"))
			return
		}
		if !s.messagingReady(w) {
			return
		}
		var in contract.MessagingAgentRequest
		if !decodeStrict(w, r, &in, 256<<10, messaging.ErrInvalid) {
			return
		}
		identity, err := s.Messaging.Authenticate(in.Credential)
		if err != nil {
			if errors.Is(err, messaging.ErrCredential) {
				failure(w, http.StatusUnauthorized, err)
			} else {
				messagingFailure(w, err)
			}
			return
		}

		switch in.Operation {
		case "identity":
			reply(w, http.StatusOK, featurewire.MessagingIdentity(identity))
		case "channels":
			items, err := s.Messaging.Channels()
			if messagingFailed(w, err) {
				return
			}
			reply(w, http.StatusOK, featurewire.MessagingChannels(items))
		case "join":
			item, err := s.Messaging.Join(identity, in.Channel)
			if messagingFailed(w, err) {
				return
			}
			reply(w, http.StatusOK, featurewire.MessagingMember(item))
		case "leave":
			if messagingFailed(w, s.Messaging.Leave(identity, in.Channel)) {
				return
			}
			reply(w, http.StatusOK, contract.Ack{OK: true})
		case "history":
			if _, err := s.Messaging.Read(r.Context(), identity, in.Channel, 0); messagingFailed(w, err) {
				return
			}
			items, err := s.Messaging.Messages(in.Channel, "", 50)
			if messagingFailed(w, err) {
				return
			}
			reply(w, http.StatusOK, featurewire.MessagingMessages(items))
		case "read":
			if in.WaitSeconds < 0 || in.WaitSeconds > 40 {
				messagingFailure(w, messaging.ErrInvalid)
				return
			}
			inbox, err := s.Messaging.Read(r.Context(), identity, in.Channel, time.Duration(in.WaitSeconds)*time.Second)
			if messagingFailed(w, err) {
				return
			}
			reply(w, http.StatusOK, featurewire.MessagingInbox(inbox))
		case "send":
			var item messaging.Message
			var err error
			if in.WakeAgents {
				item, err = s.Messaging.Request(identity, in.Channel, in.Text, in.ClientRequestID)
			} else {
				item, err = s.Messaging.AgentSend(identity, in.Channel, in.Text, in.ClientRequestID)
			}
			if messagingFailed(w, err) {
				return
			}
			reply(w, http.StatusOK, featurewire.MessagingMessage(item))
		case "ack":
			if messagingFailed(w, s.Messaging.Ack(identity, in.Channel, in.Through)) {
				return
			}
			reply(w, http.StatusOK, contract.Ack{OK: true})
		default:
			messagingFailure(w, messaging.ErrInvalid)
		}
	})
}

func (s *Server) messagingInviteRoutes(mux transport.Registrar) {
	mux.HandleFunc("GET /api/messaging/agents", func(w http.ResponseWriter, r *http.Request) {
		if !s.messagingReady(w) {
			return
		}
		out := []contract.MessagingAgent{}
		if s.Hybrid != nil {
			records, err := s.Hybrid.Records(false)
			if err != nil {
				messagingFailure(w, err)
				return
			}
			for _, rec := range records {
				if rec.State == hybrid.StateActive && rec.Side == hybrid.SideACP {
					if _, err := s.Messaging.BoundIdentity(rec.ID); err != nil {
						continue
					}
					name := rec.Title
					if name == "" {
						name = rec.Agent
					}
					out = append(out, contract.MessagingAgent{SessionID: rec.ID, Name: name, Profile: rec.Agent})
				}
			}
		}
		reply(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /api/messaging/channels/{channel}/members", func(w http.ResponseWriter, r *http.Request) {
		if !s.messagingReady(w) {
			return
		}
		var in contract.MessagingInvite
		if !decodeStrict(w, r, &in, 4096, messaging.ErrInvalid) {
			return
		}
		if s.Hybrid == nil {
			messagingFailure(w, messaging.ErrNotFound)
			return
		}
		rec, err := s.Hybrid.Record(in.SessionID)
		if err != nil || rec.State != hybrid.StateActive || rec.Side != hybrid.SideACP {
			messagingFailure(w, messaging.ErrNotFound)
			return
		}
		identity, err := s.Messaging.BoundIdentity(in.SessionID)
		if err != nil {
			messagingFailure(w, err)
			return
		}
		member, err := s.Messaging.Join(identity, r.PathValue("channel"))
		if err != nil {
			messagingFailure(w, err)
			return
		}
		reply(w, http.StatusCreated, featurewire.MessagingMember(member))
	})
	mux.HandleFunc("DELETE /api/messaging/channels/{channel}/members/{session}", func(w http.ResponseWriter, r *http.Request) {
		if !s.messagingReady(w) {
			return
		}
		if err := s.Messaging.Leave(messaging.Identity{ID: "session:" + r.PathValue("session")}, r.PathValue("channel")); err != nil {
			messagingFailure(w, err)
			return
		}
		reply(w, http.StatusOK, contract.Ack{OK: true})
	})
}

func messagingFailed(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	messagingFailure(w, err)
	return true
}
