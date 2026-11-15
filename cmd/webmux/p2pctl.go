package main

import (
	"context"
	"sync"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/api"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/p2p"
	"github.com/rainchan/webmux/internal/p2psettings"
	"github.com/rainchan/webmux/internal/pluginwire"
	"github.com/rainchan/webmux/internal/relaysetting"
)

// p2pController bridges the persistent [p2p] settings store with the running
// p2p.Service.
type p2pController struct {
	store     *p2psettings.Service
	svc       *p2p.Service
	relayAuth *relaysetting.Auth

	mu  sync.Mutex
	cfg config.P2PConfig
	web bool
}

func newP2PController(store *p2psettings.Service, svc *p2p.Service) *p2pController {
	cfg, web := store.Effective()
	return &p2pController{
		store: store,
		svc:   svc,
		cfg:   cfg,
		web:   web,
	}
}

func (c *p2pController) Enabled() bool {
	return c.svc != nil && c.svc.Enabled()
}

func (c *p2pController) Status() contract.P2PStatus {
	c.mu.Lock()
	cfg, web := c.cfg, c.web
	c.mu.Unlock()
	source := "config"
	if web {
		source = "web"
	}
	return pluginwire.P2PStatus(cfg, source, c.store.Unavailable(), c.svc.Status())
}

func (c *p2pController) UpdateSettings(in contract.P2PSettingsUpdate) (contract.P2PStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	next, err := p2psettings.Prepare(config.P2PConfig{
		Enabled:     in.Enabled,
		PortMapping: in.PortMapping,
		STUNServers: in.STUNServers,
	})
	if err != nil {
		return contract.P2PStatus{}, err
	}
	if err := c.store.Save(next); err != nil {
		return contract.P2PStatus{}, err
	}
	c.cfg, c.web = c.store.Effective()
	c.svc.Reconfigure(p2p.Config{
		Enabled:     c.cfg.Enabled,
		PortMapping: c.cfg.PortMapping,
		STUNServers: c.cfg.STUNServers,
	})
	source := "config"
	if c.web {
		source = "web"
	}
	return pluginwire.P2PStatus(c.cfg, source, c.store.Unavailable(), c.svc.Status()), nil
}

func (c *p2pController) ResetSettings() (contract.P2PStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	defaults, err := c.store.Reset()
	if err != nil {
		return contract.P2PStatus{}, err
	}
	c.cfg, c.web = defaults, false
	c.svc.Reconfigure(p2p.Config{
		Enabled:     defaults.Enabled,
		PortMapping: defaults.PortMapping,
		STUNServers: defaults.STUNServers,
	})
	return pluginwire.P2PStatus(c.cfg, "config", c.store.Unavailable(), c.svc.Status()), nil
}

func (c *p2pController) Signal(ctx context.Context, req contract.P2PSignalRequest) (contract.P2PSignalResponse, error) {
	if c.svc == nil {
		return contract.P2PSignalResponse{}, p2p.ErrUnsupported
	}
	input := pluginwire.P2PSignalInput(req)
	if principal, owner := api.RelayPrincipalContext(ctx); owner {
		if c.relayAuth == nil {
			return contract.P2PSignalResponse{}, protocol.Error{Code: protocol.Unauthenticated}
		}
		connectionID := protocol.Token(req.PeerConnectionID)
		if err := connectionID.Validate(); err != nil {
			return contract.P2PSignalResponse{}, err
		}
		var peer relaysetting.AuthPeer
		var err error
		switch req.Action {
		case "offer":
			fingerprint, parseErr := p2p.OfferFingerprint(req.SDP)
			if parseErr != nil {
				return contract.P2PSignalResponse{}, parseErr
			}
			peer, err = c.relayAuth.AuthorizePeer(principal, connectionID, fingerprint)
		case "candidate", "close":
			fingerprint, lookupErr := c.svc.OwnerPeerFingerprint(principal.ID, req.PeerConnectionID, req.PeerID)
			if lookupErr != nil {
				return contract.P2PSignalResponse{}, lookupErr
			}
			peer, err = c.relayAuth.LookupPeer(principal, connectionID, fingerprint)
		default:
			return contract.P2PSignalResponse{}, p2p.ErrInvalid
		}
		if err != nil {
			return contract.P2PSignalResponse{}, err
		}
		servers := make([]p2p.ICEServer, len(peer.Host.Servers))
		for i, server := range peer.Host.Servers {
			servers[i] = p2p.ICEServer{URLs: server.URLs, Username: server.Username, Credential: server.Credential}
		}
		input.Owner = &p2p.OwnerAuthorization{
			OwnerID:          principal.ID,
			Context:          api.RelayOwnerContext(peer.Context, principal),
			PeerConnectionID: string(peer.PeerConnectionID),
			Fingerprint:      peer.Fingerprint,
			ICEServers:       servers,
			ExpiresAt:        time.Unix(peer.Host.ExpiresAt, 0),
			RegisterAccess: func(close func(), done <-chan struct{}) (func(), error) {
				return c.relayAuth.RegisterAccess(principal, close, done)
			},
		}
	} else if req.PeerConnectionID != "" {
		// A recovery token or local identity is not owner TURN authority.
		return contract.P2PSignalResponse{}, protocol.Error{Code: protocol.Forbidden}
	}
	out, err := c.svc.Signal(ctx, input)
	if err != nil {
		return contract.P2PSignalResponse{}, err
	}
	return pluginwire.P2PSignalResponse(out), nil
}
