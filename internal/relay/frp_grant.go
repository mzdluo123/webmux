package relay

import (
	"context"
	"encoding/base64"
	"net/url"
	"strings"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

type frpGrantResult struct {
	grant     *protocol.AllocationGrant
	requested *protocol.AllocationGrant
	err       error
}

func (s *connection) requestFRP(ctx context.Context, csr string, current *protocol.AllocationGrant) <-chan frpGrantResult {
	result := make(chan frpGrantResult, 1)
	go func() {
		if current != nil && current.Transport != nil {
			r := protocol.TransportRequest{Action: protocol.TransportRenew, OperationID: current.Transport.OperationID, RouteVersion: current.Transport.RouteVersion,
				AllocationRenew: &protocol.AllocationRenew{AllocationID: current.AllocationID, GrantRevision: current.GrantRevision}}
			reply, err := s.callTransport(ctx, r)
			if err == nil && (reply.State != protocol.TransportCommitted || reply.Active != current.Protocol() ||
				reply.ActiveAllocationID != current.AllocationID || reply.Grant == nil) {
				err = protocol.Error{Code: protocol.StaleGeneration}
			}
			result <- frpGrantResult{grant: reply.Grant, requested: current, err: err}
			return
		}
		id, err := newToken()
		if err != nil {
			result <- frpGrantResult{requested: current, err: err}
			return
		}
		message := s.rpc.message(protocol.ControlFRPAllocate, id)
		if current == nil {
			message.AllocationRequest = &protocol.AllocationRequest{HostServerCSR: csr}
		} else {
			message.Type = protocol.ControlFRPRenew
			message.AllocationRenew = &protocol.AllocationRenew{AllocationID: current.AllocationID, GrantRevision: current.GrantRevision}
		}
		reply, err := s.rpc.call(ctx, message, protocol.ControlFRPGrant, nil)
		result <- frpGrantResult{grant: reply.AllocationGrant, requested: current, err: err}
	}()
	return result
}

func (s *connection) validateFRPGrant(g, current *protocol.AllocationGrant) error {
	if g == nil || g.NodeOrigin != nil || g.ExpiresAt-g.IssuedAt > 120 || g.ExpiresAt-g.NotBefore > 125 ||
		g.RemotePort < protocol.FRPPortMin || g.RemotePort > protocol.FRPPortMax {
		return protocol.Error{Code: protocol.Invalid}
	}
	if g.Protocol() == protocol.FRPTransportWSS {
		connector, _ := url.Parse(s.assignment.ConnectorURL)
		endpoint, err := url.Parse(g.TransportURL)
		if err != nil || endpoint.Hostname() != connector.Hostname() {
			return protocol.Error{Code: protocol.WrongAudience}
		}
	} else if s.control == nil || !s.control.selection || g.Protocol() != protocol.FRPTransportQUIC || g.Transport.Validate() != nil ||
		g.TransportURL != "" || g.BridgeKey != "" {
		return protocol.Error{Code: protocol.Forbidden}
	}
	// The management channel authenticates this grant. Check that the opaque
	// credential actually describes the same binding; stock frps and node
	// admission independently verify the signature, issuer, and active JTI.
	_, pieces, _ := strings.Cut(g.JWT, ".")
	body, _, _ := strings.Cut(pieces, ".")
	raw, err := base64.RawURLEncoding.Strict().DecodeString(body)
	var claims protocol.FRPClaims
	if err != nil || protocol.Decode(raw, &claims, protocol.MaxControlBytes) != nil || claims.ID.Validate() != nil || claims.Issuer == "" ||
		!g.Transport.Equal(claims.Transport) {
		return protocol.Error{Code: protocol.Forbidden}
	}
	claims.Transport = nil
	if claims != (protocol.FRPClaims{Issuer: claims.Issuer, Audience: protocol.FRPAudience(s.assignment.NodeID), Subject: string(g.AllocationID), ID: claims.ID,
		IssuedAt: g.IssuedAt, NotBefore: g.NotBefore, ExpiresAt: g.ExpiresAt, Scope: s.scope, NodeID: s.assignment.NodeID,
		AllocationID: g.AllocationID, GrantRevision: g.GrantRevision, Role: protocol.FRPRole, ProxyName: g.ProxyName, RemotePort: g.RemotePort}) {
		return protocol.Error{Code: protocol.Forbidden}
	}
	if current == nil {
		return nil
	}
	if g.AllocationID != current.AllocationID || g.GrantRevision != current.GrantRevision+1 ||
		g.ProxyName != current.ProxyName || g.RemotePort != current.RemotePort || g.TransportURL != current.TransportURL ||
		g.BridgeKey != current.BridgeKey || g.Origin != current.Origin || g.JWT == current.JWT || !g.Transport.Equal(current.Transport) {
		return protocol.Error{Code: protocol.StaleGeneration}
	}
	return nil
}
