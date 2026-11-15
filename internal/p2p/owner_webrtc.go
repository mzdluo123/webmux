//go:build !nop2p

package p2p

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/pion/sdp/v3"
)

// OfferFingerprint parses the same SDP used for Pion negotiation and requires a
// single SHA-256 DTLS identity across every media section.
func OfferFingerprint(raw string) (string, error) {
	if len(raw) == 0 || len(raw) > 64*1024 {
		return "", fmt.Errorf("%w: invalid offer SDP", ErrInvalid)
	}
	var description sdp.SessionDescription
	if err := description.UnmarshalString(raw); err != nil || len(description.MediaDescriptions) == 0 {
		return "", fmt.Errorf("%w: invalid offer SDP", ErrInvalid)
	}
	fingerprint := ""
	read := func(attributes []sdp.Attribute) error {
		for _, attribute := range attributes {
			if attribute.Key != "fingerprint" {
				continue
			}
			parts := strings.Fields(attribute.Value)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "sha-256") || len(parts[1]) != 95 {
				return ErrInvalid
			}
			for i := 2; i < len(parts[1]); i += 3 {
				if parts[1][i] != ':' {
					return ErrInvalid
				}
			}
			if _, err := hex.DecodeString(strings.ReplaceAll(parts[1], ":", "")); err != nil {
				return ErrInvalid
			}
			canonical := "sha-256 " + strings.ToUpper(parts[1])
			if fingerprint != "" && fingerprint != canonical {
				return ErrInvalid
			}
			fingerprint = canonical
		}
		return nil
	}
	if err := read(description.Attributes); err != nil {
		return "", err
	}
	sessionFingerprint := fingerprint
	for _, media := range description.MediaDescriptions {
		if err := read(media.Attributes); err != nil {
			return "", err
		}
		hasFingerprint := sessionFingerprint != ""
		for _, attribute := range media.Attributes {
			hasFingerprint = hasFingerprint || attribute.Key == "fingerprint"
		}
		if !hasFingerprint {
			return "", ErrInvalid
		}
	}
	if fingerprint == "" {
		return "", ErrInvalid
	}
	return fingerprint, nil
}

func validOwner(owner *OwnerAuthorization) bool {
	return owner != nil && owner.OwnerID != "" && owner.PeerConnectionID != "" &&
		owner.Fingerprint != "" && owner.Context != nil && owner.Context.Err() == nil &&
		time.Now().Before(owner.ExpiresAt)
}

func (p *peerEntry) permits(owner *OwnerAuthorization) bool {
	if p.ctx.Err() != nil {
		return false
	}
	if p.owner == nil {
		return owner == nil
	}
	return validOwner(owner) && p.owner.OwnerID == owner.OwnerID &&
		p.owner.PeerConnectionID == owner.PeerConnectionID &&
		p.owner.Fingerprint == owner.Fingerprint && p.owner.Context.Done() == owner.Context.Done()
}

// OwnerPeerFingerprint only exposes a binding after matching the opaque owner
// session and both peer identifiers; it never turns a local peer into an owner peer.
func (s *Service) OwnerPeerFingerprint(ownerID, connectionID, peerID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	peer := s.peers[peerID]
	if peer == nil || peer.owner == nil || peer.ctx.Err() != nil ||
		peer.owner.OwnerID != ownerID || peer.owner.PeerConnectionID != connectionID {
		return "", ErrPeerMissing
	}
	return peer.owner.Fingerprint, nil
}
