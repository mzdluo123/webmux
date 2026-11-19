//go:build !nop2p

package p2p

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
)

// Signal handles offer/answer negotiation, trickle ICE candidates, and peer
// closure without ever logging SDP bodies.
func (s *Service) Signal(ctx context.Context, in SignalInput) (SignalOutput, error) {
	switch in.Action {
	case "offer":
		return s.handleOffer(ctx, in)
	case "candidate":
		return s.handleCandidate(in)
	case "close":
		if in.PeerID == "" {
			return SignalOutput{}, fmt.Errorf("%w: peerId is required", ErrInvalid)
		}
		s.mu.Lock()
		entry := s.peers[in.PeerID]
		if entry == nil || !entry.permits(in.Owner) {
			s.mu.Unlock()
			return SignalOutput{}, ErrPeerMissing
		}
		delete(s.peers, in.PeerID)
		s.mu.Unlock()
		entry.close()
		return SignalOutput{PeerID: in.PeerID, Candidates: []ICECandidate{}, STUNServers: s.signalSTUNServers(in.Owner)}, nil
	default:
		return SignalOutput{}, fmt.Errorf("%w: unsupported action", ErrInvalid)
	}
}

func (s *Service) stunServers() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.STUNServers == nil {
		return []string{}
	}
	return slices.Clone(s.cfg.STUNServers)
}

func (s *Service) signalSTUNServers(owner *OwnerAuthorization) []string {
	if owner != nil {
		return []string{}
	}
	return s.stunServers()
}

func (s *Service) allowSignalLocked(now time.Time) bool {
	cutoff := now.Add(-time.Minute)
	keep := s.signalTimes[:0]
	for _, ts := range s.signalTimes {
		if ts.After(cutoff) {
			keep = append(keep, ts)
		}
	}
	s.signalTimes = keep
	if len(s.signalTimes) >= maxSignalPerMinute {
		return false
	}
	s.signalTimes = append(s.signalTimes, now)
	return true
}

func (s *Service) handleOffer(ctx context.Context, in SignalInput) (SignalOutput, error) {
	if strings.TrimSpace(in.SDP) == "" || len(in.SDP) > 64*1024 {
		return SignalOutput{}, fmt.Errorf("%w: invalid offer SDP", ErrInvalid)
	}
	if in.Owner != nil {
		fingerprint, err := OfferFingerprint(in.SDP)
		if !validOwner(in.Owner) || in.Owner.RegisterAccess == nil || err != nil || fingerprint != in.Owner.Fingerprint {
			return SignalOutput{}, ErrInvalid
		}
	}

	now := time.Now()
	s.mu.Lock()
	if s.closed || (!s.cfg.Enabled && in.Owner == nil) {
		s.mu.Unlock()
		return SignalOutput{}, ErrDisabled
	}
	if !s.allowSignalLocked(now) {
		s.mu.Unlock()
		return SignalOutput{}, ErrRateLimited
	}
	if len(s.peers) >= s.maxPeers {
		s.evictStaleOrOldestLocked(now)
	}
	if len(s.peers) >= s.maxPeers {
		s.mu.Unlock()
		return SignalOutput{}, ErrTooMany
	}
	stuns := slices.Clone(s.cfg.STUNServers)
	mappedIP := s.mappedIP
	udpMux := s.udpMux
	if in.Owner != nil {
		// Owner ICE is isolated from manual STUN, UPnP rewrites and the local
		// shared UDP mux. Only authenticated, per-peer discovery is applied.
		stuns, mappedIP, udpMux = nil, "", nil
		for _, peer := range s.peers {
			if peer.owner != nil && peer.owner.OwnerID == in.Owner.OwnerID &&
				peer.owner.PeerConnectionID == in.Owner.PeerConnectionID {
				s.mu.Unlock()
				return SignalOutput{}, ErrInvalid
			}
		}
	}
	s.mu.Unlock()

	var se webrtc.SettingEngine
	se.SetSCTPMaxReceiveBufferSize(maxBufferedAmount)
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetIncludeLoopbackCandidate(true)
	se.SetInterfaceFilter(s.allowInterface)
	se.SetNetworkTypes([]webrtc.NetworkType{
		webrtc.NetworkTypeUDP4,
		webrtc.NetworkTypeUDP6,
	})
	if udpMux != nil {
		se.SetICEUDPMux(udpMux)
	}
	if mappedIP != "" {
		_ = se.SetICEAddressRewriteRules(webrtc.ICEAddressRewriteRule{
			External:        []string{mappedIP},
			AsCandidateType: webrtc.ICECandidateTypeHost,
			Mode:            webrtc.ICEAddressRewriteAppend,
		})
	}

	apiEngine := webrtc.NewAPI(webrtc.WithSettingEngine(se))
	var iceServers []webrtc.ICEServer
	if len(stuns) > 0 {
		iceServers = append(iceServers, webrtc.ICEServer{URLs: stuns})
	}
	if in.Owner != nil {
		for _, server := range in.Owner.ICEServers {
			iceServers = append(iceServers, webrtc.ICEServer{
				URLs: slices.Clone(server.URLs), Username: server.Username,
				Credential: server.Credential, CredentialType: webrtc.ICECredentialTypePassword,
			})
		}
	}
	pc, err := apiEngine.NewPeerConnection(webrtc.Configuration{ICEServers: iceServers})
	if err != nil {
		return SignalOutput{}, err
	}

	parentCtx := context.Background()
	var owner *OwnerAuthorization
	if in.Owner != nil {
		copyOwner := *in.Owner
		copyOwner.ICEServers = nil // Pion owns its configuration; don't retain another secret copy.
		copyOwner.RegisterAccess = nil
		owner = &copyOwner
		parentCtx = owner.Context
	}
	var peerCtx context.Context
	var cancel context.CancelFunc
	if owner != nil && !owner.ExpiresAt.IsZero() {
		peerCtx, cancel = context.WithDeadline(parentCtx, owner.ExpiresAt)
	} else {
		peerCtx, cancel = context.WithCancel(parentCtx)
	}
	id := randomPeerID()
	entry := &peerEntry{
		id:        id,
		pc:        pc,
		ctx:       peerCtx,
		cancel:    cancel,
		createdAt: now,
		owner:     owner,
		done:      make(chan struct{}),
	}
	entry.touch()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		cancel()
		_ = pc.Close()
		return SignalOutput{}, ErrDisabled
	}
	s.peerWorkers.Add(1)
	entry.onClosed = s.peerWorkers.Done
	s.mu.Unlock()
	if owner != nil {
		release, err := in.Owner.RegisterAccess(func() { s.removePeerEntry(entry) }, entry.done)
		if err != nil {
			entry.close()
			return SignalOutput{}, err
		}
		if release == nil {
			entry.close()
			return SignalOutput{}, ErrInvalid
		}
		go func() {
			<-entry.done
			release()
		}()
	}

	gatherDone := make(chan struct{})
	var gatherOnce sync.Once
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			gatherOnce.Do(func() { close(gatherDone) })
			return
		}
		init := c.ToJSON()
		entry.mu.Lock()
		entry.pendingLocal = append(entry.pendingLocal, ICECandidate{
			Candidate:     init.Candidate,
			SDPMid:        init.SDPMid,
			SDPMLineIndex: init.SDPMLineIndex,
		})
		entry.mu.Unlock()
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		entry.touch()
		switch state {
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			s.removePeerEntry(entry)
		}
	})

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		entry.touch()
		if entry.ctx.Err() != nil {
			_ = dc.Close()
			return
		}
		label := dc.Label()
		switch {
		case label == controlChannelLabel:
			s.attachControlChannel(entry, dc)
		case strings.HasPrefix(label, streamChannelPrefix):
			path := strings.TrimPrefix(label, streamChannelPrefix)
			s.attachStreamChannel(entry, path, dc)
		case label == "rpc" || strings.HasPrefix(label, rpcChannelPrefix):
			s.attachRPCChannel(entry, dc)
		default:
			_ = dc.Close()
		}
	})

	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  in.SDP,
	}); err != nil {
		entry.close()
		return SignalOutput{}, fmt.Errorf("%w: invalid remote offer", ErrInvalid)
	}
	for _, cand := range in.Candidates {
		if cand.Candidate != "" {
			if err := pc.AddICECandidate(webrtc.ICECandidateInit{
				Candidate: cand.Candidate, SDPMid: cand.SDPMid, SDPMLineIndex: cand.SDPMLineIndex,
			}); err != nil {
				entry.close()
				return SignalOutput{}, fmt.Errorf("%w: invalid ICE candidate", ErrInvalid)
			}
		}
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		entry.close()
		return SignalOutput{}, err
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		entry.close()
		return SignalOutput{}, err
	}

	s.mu.Lock()
	if err := entry.ctx.Err(); err != nil {
		s.mu.Unlock()
		entry.close()
		return SignalOutput{}, err
	}
	if owner != nil && !validOwner(owner) {
		s.mu.Unlock()
		entry.close()
		return SignalOutput{}, context.DeadlineExceeded
	}
	if s.closed || (!s.cfg.Enabled && owner == nil) {
		s.mu.Unlock()
		entry.close()
		return SignalOutput{}, ErrDisabled
	}
	if len(s.peers) >= s.maxPeers {
		s.mu.Unlock()
		entry.close()
		return SignalOutput{}, ErrTooMany
	}
	for _, peer := range s.peers {
		if owner != nil && peer.owner != nil && peer.owner.OwnerID == owner.OwnerID &&
			peer.owner.PeerConnectionID == owner.PeerConnectionID {
			s.mu.Unlock()
			entry.close()
			return SignalOutput{}, ErrInvalid
		}
	}
	s.peers[id] = entry
	s.mu.Unlock()
	if owner != nil {
		context.AfterFunc(entry.ctx, func() { s.removePeerEntry(entry) })
	}

	timer := time.NewTimer(gatherWaitTimeout)
	select {
	case <-gatherDone:
	case <-timer.C:
	case <-entry.ctx.Done():
		timer.Stop()
		s.removePeerEntry(entry)
		return SignalOutput{}, entry.ctx.Err()
	case <-ctx.Done():
		timer.Stop()
		s.removePeerEntry(entry)
		return SignalOutput{}, ctx.Err()
	}
	timer.Stop()

	entry.mu.Lock()
	cands := slices.Clone(entry.pendingLocal)
	entry.pendingLocal = nil
	entry.mu.Unlock()
	if cands == nil {
		cands = []ICECandidate{}
	}
	if stuns == nil {
		stuns = []string{}
	}
	localDesc := pc.LocalDescription()
	sdp := answer.SDP
	if localDesc != nil {
		sdp = localDesc.SDP
	}
	return SignalOutput{
		PeerID:      id,
		SDP:         sdp,
		Candidates:  cands,
		STUNServers: stuns,
	}, nil
}

func (s *Service) handleCandidate(in SignalInput) (SignalOutput, error) {
	if in.PeerID == "" {
		return SignalOutput{}, fmt.Errorf("%w: peerId is required", ErrInvalid)
	}
	s.mu.Lock()
	entry := s.peers[in.PeerID]
	s.mu.Unlock()
	if entry == nil || !entry.permits(in.Owner) {
		return SignalOutput{}, ErrPeerMissing
	}
	entry.touch()
	for _, cand := range in.Candidates {
		if strings.TrimSpace(cand.Candidate) == "" {
			continue
		}
		if err := entry.pc.AddICECandidate(webrtc.ICECandidateInit{
			Candidate:     cand.Candidate,
			SDPMid:        cand.SDPMid,
			SDPMLineIndex: cand.SDPMLineIndex,
		}); err != nil {
			return SignalOutput{}, fmt.Errorf("%w: invalid ICE candidate", ErrInvalid)
		}
	}
	entry.mu.Lock()
	cands := slices.Clone(entry.pendingLocal)
	entry.pendingLocal = nil
	entry.mu.Unlock()
	if cands == nil {
		cands = []ICECandidate{}
	}
	return SignalOutput{
		PeerID:      in.PeerID,
		Candidates:  cands,
		STUNServers: s.signalSTUNServers(in.Owner),
	}, nil
}

func (s *Service) removePeerEntry(entry *peerEntry) {
	s.mu.Lock()
	if s.peers[entry.id] == entry {
		delete(s.peers, entry.id)
	}
	s.mu.Unlock()
	entry.close()
}

func (s *Service) evictStaleOrOldestLocked(now time.Time) {
	for id, p := range s.peers {
		st := p.pc.ConnectionState()
		if st == webrtc.PeerConnectionStateDisconnected || st == webrtc.PeerConnectionStateFailed || st == webrtc.PeerConnectionStateClosed {
			delete(s.peers, id)
			p.close()
			return
		}
		last := time.Unix(0, p.lastSeen.Load())
		if now.Sub(last) > s.idleTimeout {
			delete(s.peers, id)
			p.close()
			return
		}
	}
}

func randomPeerID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
