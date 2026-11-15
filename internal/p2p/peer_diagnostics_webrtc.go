//go:build !nop2p

package p2p

import (
	"net"
	"slices"
	"strings"
	"time"

	"github.com/pion/webrtc/v4"
)

func (p *peerEntry) selectedCandidatePair() *webrtc.ICECandidatePair {
	sctp := p.pc.SCTP()
	if sctp == nil || sctp.Transport() == nil || sctp.Transport().ICETransport() == nil {
		return nil
	}
	pair, err := sctp.Transport().ICETransport().GetSelectedCandidatePair()
	if err != nil || pair == nil || pair.Local == nil || pair.Remote == nil {
		return nil
	}
	return pair
}

func (p *peerEntry) selectedDiagnostics() (candidateType, family, protocol, path string) {
	pair := p.selectedCandidatePair()
	if pair != nil {
		candidateType = pair.Local.Typ.String() + "/" + pair.Remote.Typ.String()
		family = "ipv4"
		if strings.Contains(pair.Remote.Address, ":") {
			family = "ipv6"
		}
		protocol = pair.Local.Protocol.String()
		path = "webrtc-direct"
		if pair.Local.Typ == webrtc.ICECandidateTypeRelay || pair.Remote.Typ == webrtc.ICECandidateTypeRelay {
			path = "webrtc-turn"
		}
	}
	// Preserve independent local diagnostics, but never trust owner peer_meta
	// for candidate selection or transport classification.
	if p.owner == nil {
		p.mu.Lock()
		if p.candidateType != "" {
			candidateType, family = p.candidateType, p.ipFamily
		}
		p.mu.Unlock()
	}
	return
}

// Status returns sanitized diagnostics for active peers and port mapping.
func (s *Service) Status() Status {
	pmStatus, _ := s.mapperStatus()
	s.mu.Lock()
	bindings := make([]UDPBinding, 0, len(s.udpConns))
	if !s.closed {
		for _, conn := range s.udpConns {
			if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
				network := "udp6"
				if addr.IP.To4() != nil {
					network = "udp4"
				}
				bindings = append(bindings, UDPBinding{Network: network, Port: addr.Port})
			}
		}
	}

	entries := make([]*peerEntry, 0, len(s.peers))
	for _, p := range s.peers {
		entries = append(entries, p)
	}
	s.mu.Unlock()

	peers := make([]PeerInfo, 0, len(entries))
	for _, p := range entries {
		ct, fam, protocol, path := p.selectedDiagnostics()
		ownerID := ""
		if p.owner != nil {
			ownerID = p.owner.OwnerID
		}
		p.mu.Lock()
		rtt := p.rttMs
		p.mu.Unlock()
		peers = append(peers, PeerInfo{
			ID:            p.id,
			State:         p.pc.ConnectionState().String(),
			CandidateType: ct,
			IPFamily:      fam,
			Transport:     protocol,
			Path:          path,
			OwnerID:       ownerID,
			RTTMs:         rtt,
			Channels:      int(max(0, p.channels.Load())),
			CreatedAt:     p.createdAt.UTC().Format(time.RFC3339),
		})
	}
	slices.SortFunc(peers, func(a, b PeerInfo) int {
		return strings.Compare(a.ID, b.ID)
	})
	return Status{
		Supported:   true,
		UDPPort:     s.udpPortCfg,
		UDPBindings: bindings,
		PortMap:     pmStatus,
		ActivePeers: len(peers),
		Peers:       peers,
	}
}
