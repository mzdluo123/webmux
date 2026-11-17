package localclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rainchan/webmux/internal/builtin"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/jsonutil"
)

// capabilityFeatures is the protocol's capability ownership map. Unknown IDs are
// never interpreted as permission to invoke an optional endpoint.
var capabilityFeatures = map[string]string{
	"terminal.pty": "", "workspace.manage": "workspace", "code.browse": "code",
	"files.upload": "files", "launch.configs": "launch", "chat.acp": "chat",
	"notifications.push": "notifications", "quick-commands.send": "quick-commands",
	"voice.recognize": "voice", "forward.ports": "forward", "cloudflare.manage": "cloudflare",
	"workspace.git-workbench": "workspace",
	"workspace.projects":      "workspace",
	"workspace.clone":         "workspace",
	"workspace.github":        "workspace",
	"relay.hybrid":            "p2p",
	"tailscale.manage":        "tailscale", "p2p.direct": "p2p", "mcp.control": "mcp", "immersive.xr": "immersive",
}

// CapabilityError distinguishes disabled features from old or broken servers.
type CapabilityError struct {
	ID     string
	Reason string
}

func (e *CapabilityError) Error() string {
	return fmt.Sprintf("capability %s unavailable (%s)", e.ID, e.Reason)
}

// Capabilities reads the versioned instance snapshot without probing business
// endpoints. Invalid snapshots fail closed, including missing boolean fields.
func (c *Client) Capabilities(ctx context.Context) (contract.Capabilities, error) {
	var raw json.RawMessage
	if err := c.GetBounded(ctx, contract.PathCapabilities, 64<<10, &raw); err != nil {
		return contract.Capabilities{}, fmt.Errorf("read WebMux capabilities: %w", err)
	}
	var probe struct {
		Version int `json:"version"`
	}
	if json.Unmarshal(raw, &probe) != nil {
		return contract.Capabilities{}, errors.New("invalid WebMux capabilities response")
	}
	var snapshot contract.Capabilities
	if probe.Version == contract.CapabilitiesVersionV1 {
		translated, err := decodeCapabilitiesV1(raw)
		if err != nil {
			return contract.Capabilities{}, err
		}
		snapshot = translated
	} else {
		var shape struct {
			Features []struct {
				Enabled *bool `json:"enabled"`
			} `json:"features"`
			Capabilities []struct {
				Available *bool `json:"available"`
			} `json:"capabilities"`
		}
		if json.Unmarshal(raw, &shape) != nil || jsonutil.Decode(raw, &snapshot) != nil || shape.Features == nil || shape.Capabilities == nil {
			return contract.Capabilities{}, errors.New("invalid WebMux capabilities response")
		}
		for _, p := range shape.Features {
			if p.Enabled == nil {
				return contract.Capabilities{}, errors.New("invalid WebMux feature enabled field")
			}
		}
		for _, c := range shape.Capabilities {
			if c.Available == nil {
				return contract.Capabilities{}, errors.New("invalid WebMux capability available field")
			}
		}
	}
	if err := ValidateCapabilities(snapshot); err != nil {
		return contract.Capabilities{}, err
	}
	return snapshot, nil
}

// reasonFeatureDisabledV1 is the retired v1 reason code on the wire.
const reasonFeatureDisabledV1 = "plugin_disabled"

// decodeCapabilitiesV1 reads the retired v1 snapshot (a plugins field carrying
// the plugin_disabled reason) and normalizes it into the current model. The
// same strict shape and fail-closed validation then applies unchanged.
func decodeCapabilitiesV1(raw json.RawMessage) (contract.Capabilities, error) {
	var shape struct {
		Plugins []struct {
			Enabled *bool `json:"enabled"`
		} `json:"plugins"`
		Capabilities []struct {
			Available *bool `json:"available"`
		} `json:"capabilities"`
	}
	var legacy struct {
		Version      int                      `json:"version"`
		Plugins      []contract.FeatureStatus `json:"plugins"`
		Capabilities []contract.Capability    `json:"capabilities"`
	}
	if json.Unmarshal(raw, &shape) != nil || jsonutil.Decode(raw, &legacy) != nil || legacy.Version != contract.CapabilitiesVersionV1 || shape.Plugins == nil || shape.Capabilities == nil {
		return contract.Capabilities{}, errors.New("invalid WebMux capabilities response")
	}
	snapshot := contract.Capabilities{Version: contract.CapabilitiesVersion, Features: legacy.Plugins, Capabilities: legacy.Capabilities}
	for i := range shape.Plugins {
		if shape.Plugins[i].Enabled == nil {
			return contract.Capabilities{}, errors.New("invalid WebMux feature enabled field")
		}
		if snapshot.Features[i].Reason == reasonFeatureDisabledV1 {
			snapshot.Features[i].Reason = contract.CapabilityReasonFeatureDisabled
		}
	}
	for i := range shape.Capabilities {
		if shape.Capabilities[i].Available == nil {
			return contract.Capabilities{}, errors.New("invalid WebMux capability available field")
		}
		if snapshot.Capabilities[i].Reason == reasonFeatureDisabledV1 {
			snapshot.Capabilities[i].Reason = contract.CapabilityReasonFeatureDisabled
		}
	}
	return snapshot, nil
}

// ValidateCapabilities also validates snapshots supplied by an in-process host.
func ValidateCapabilities(snapshot contract.Capabilities) error {
	invalid := errors.New("invalid or unsupported WebMux capabilities response")
	if snapshot.Version != contract.CapabilitiesVersion || snapshot.Features == nil || snapshot.Capabilities == nil {
		return invalid
	}
	known := make(map[string]bool)
	for _, p := range builtin.Catalog() {
		known[p.ID] = true
	}
	features := make(map[string]contract.FeatureStatus, len(snapshot.Features))
	enabled := make([]string, 0, len(snapshot.Features))
	for _, p := range snapshot.Features {
		if !known[p.ID] {
			return invalid
		}
		if _, exists := features[p.ID]; exists {
			return invalid
		}
		if !validCapabilityReason(p.Reason) {
			return invalid
		}
		switch p.Status {
		case contract.FeatureStatusReady:
			if !p.Enabled || p.Reason != "" {
				return invalid
			}
		case contract.FeatureStatusDisabled:
			if p.Enabled || p.Reason != contract.CapabilityReasonFeatureDisabled {
				return invalid
			}
		case contract.FeatureStatusUnavailable:
			if !p.Enabled || p.Reason == "" || p.Reason == contract.CapabilityReasonFeatureDisabled {
				return invalid
			}
		default:
			return invalid
		}
		features[p.ID] = p
		if p.Enabled {
			enabled = append(enabled, p.ID)
		}
	}
	if len(features) != len(known) {
		return invalid
	}
	if _, err := builtin.Resolve(enabled); err != nil {
		return invalid
	}
	seen := make(map[string]bool, len(snapshot.Capabilities))
	for _, c := range snapshot.Capabilities {
		owner, ok := capabilityFeatures[c.ID]
		if !ok || seen[c.ID] || !validCapabilityReason(c.Reason) {
			return invalid
		}
		seen[c.ID] = true
		ownerEnabled, ownerReady := true, true
		if owner != "" {
			ownerEnabled = features[owner].Enabled
			ownerReady = features[owner].Status == contract.FeatureStatusReady
		}
		if c.ID == "workspace.git-workbench" {
			ownerEnabled = ownerEnabled && features["code"].Enabled
			ownerReady = ownerReady && features["code"].Status == contract.FeatureStatusReady
		}
		if c.Available {
			if c.Reason != "" || !ownerReady {
				return invalid
			}
		} else if c.Reason == "" || owner == "" {
			return invalid
		} else if !ownerEnabled && c.Reason != contract.CapabilityReasonFeatureDisabled {
			return invalid
		} else if ownerEnabled && c.Reason == contract.CapabilityReasonFeatureDisabled {
			return invalid
		}
	}
	if !seen["terminal.pty"] {
		return invalid
	}
	return nil
}

func validCapabilityReason(reason string) bool {
	switch reason {
	case "", contract.CapabilityReasonFeatureDisabled, contract.CapabilityReasonConfigurationOff,
		contract.CapabilityReasonUnsupported, contract.CapabilityReasonStorageUnavailable,
		contract.CapabilityReasonCommandUnavailable, contract.CapabilityReasonServiceUnavailable:
		return true
	default:
		return false
	}
}

// CapabilityAvailable consults only a validated snapshot; absent IDs deny access.
func CapabilityAvailable(snapshot contract.Capabilities, id string) bool {
	if _, known := capabilityFeatures[id]; !known {
		return false
	}
	for _, c := range snapshot.Capabilities {
		if c.ID == id {
			return c.Available
		}
	}
	return false
}

// FeatureEnabled consults a validated snapshot for diagnostic surfaces that
// remain useful even when the feature's business capability is unavailable.
func FeatureEnabled(snapshot contract.Capabilities, id string) bool {
	for _, p := range snapshot.Features {
		if p.ID == id {
			return p.Enabled
		}
	}
	return false
}

// RequireFeature allows diagnostics for selected but unavailable features, while
// still failing closed on invalid discovery or disabled features.
func (c *Client) RequireFeature(ctx context.Context, id string) error {
	snapshot, err := c.Capabilities(ctx)
	if err != nil {
		return err
	}
	if FeatureEnabled(snapshot, id) {
		return nil
	}
	return &CapabilityError{ID: id, Reason: contract.CapabilityReasonFeatureDisabled}
}

// RequireCapability reports a disabled feature without guessing from a 404.
func (c *Client) RequireCapability(ctx context.Context, id string) error {
	snapshot, err := c.Capabilities(ctx)
	if err != nil {
		return err
	}
	return RequireSnapshotCapability(snapshot, id)
}

func RequireSnapshotCapability(snapshot contract.Capabilities, id string) error {
	if _, known := capabilityFeatures[id]; !known {
		return &CapabilityError{ID: id, Reason: contract.CapabilityReasonServiceUnavailable}
	}
	for _, c := range snapshot.Capabilities {
		if c.ID == id {
			if c.Available {
				return nil
			}
			return &CapabilityError{ID: id, Reason: c.Reason}
		}
	}
	owner, known := capabilityFeatures[id]
	if known && owner != "" {
		for _, p := range snapshot.Features {
			if p.ID == owner && !p.Enabled {
				return &CapabilityError{ID: id, Reason: contract.CapabilityReasonFeatureDisabled}
			}
			if id == "workspace.git-workbench" && p.ID == "code" && !p.Enabled {
				return &CapabilityError{ID: id, Reason: contract.CapabilityReasonFeatureDisabled}
			}
		}
	}
	return &CapabilityError{ID: id, Reason: contract.CapabilityReasonServiceUnavailable}
}
