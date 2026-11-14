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

// capabilityPlugins is the protocol's capability ownership map. Unknown IDs are
// never interpreted as permission to invoke an optional endpoint.
var capabilityPlugins = map[string]string{
	"terminal.pty": "", "workspace.manage": "workspace", "code.browse": "code",
	"files.upload": "files", "launch.configs": "launch", "chat.acp": "chat",
	"notifications.push": "notifications", "quick-commands.send": "quick-commands",
	"voice.recognize": "voice", "forward.ports": "forward", "cloudflare.manage": "cloudflare",
	"workspace.git-workbench": "workspace",
	"tailscale.manage":        "tailscale", "p2p.direct": "p2p", "mcp.control": "mcp", "immersive.xr": "immersive",
}

// CapabilityError distinguishes disabled plugins from old or broken servers.
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
	var shape struct {
		Plugins []struct {
			Enabled *bool `json:"enabled"`
		} `json:"plugins"`
		Capabilities []struct {
			Available *bool `json:"available"`
		} `json:"capabilities"`
	}
	var snapshot contract.Capabilities
	if json.Unmarshal(raw, &shape) != nil || jsonutil.Decode(raw, &snapshot) != nil || shape.Plugins == nil || shape.Capabilities == nil {
		return contract.Capabilities{}, errors.New("invalid WebMux capabilities response")
	}
	for _, p := range shape.Plugins {
		if p.Enabled == nil {
			return contract.Capabilities{}, errors.New("invalid WebMux plugin enabled field")
		}
	}
	for _, c := range shape.Capabilities {
		if c.Available == nil {
			return contract.Capabilities{}, errors.New("invalid WebMux capability available field")
		}
	}
	if err := ValidateCapabilities(snapshot); err != nil {
		return contract.Capabilities{}, err
	}
	return snapshot, nil
}

// ValidateCapabilities also validates snapshots supplied by an in-process host.
func ValidateCapabilities(snapshot contract.Capabilities) error {
	invalid := errors.New("invalid or unsupported WebMux capabilities response")
	if snapshot.Version != contract.CapabilitiesVersion || snapshot.Plugins == nil || snapshot.Capabilities == nil {
		return invalid
	}
	known := make(map[string]bool)
	for _, p := range builtin.Catalog() {
		known[p.ID] = true
	}
	plugins := make(map[string]contract.PluginStatus, len(snapshot.Plugins))
	enabled := make([]string, 0, len(snapshot.Plugins))
	for _, p := range snapshot.Plugins {
		if !known[p.ID] {
			return invalid
		}
		if _, exists := plugins[p.ID]; exists {
			return invalid
		}
		if !validCapabilityReason(p.Reason) {
			return invalid
		}
		switch p.Status {
		case contract.PluginStatusReady:
			if !p.Enabled || p.Reason != "" {
				return invalid
			}
		case contract.PluginStatusDisabled:
			if p.Enabled || p.Reason != contract.CapabilityReasonPluginDisabled {
				return invalid
			}
		case contract.PluginStatusUnavailable:
			if !p.Enabled || p.Reason == "" || p.Reason == contract.CapabilityReasonPluginDisabled {
				return invalid
			}
		default:
			return invalid
		}
		plugins[p.ID] = p
		if p.Enabled {
			enabled = append(enabled, p.ID)
		}
	}
	if len(plugins) != len(known) {
		return invalid
	}
	if _, err := builtin.Resolve(enabled); err != nil {
		return invalid
	}
	seen := make(map[string]bool, len(snapshot.Capabilities))
	for _, c := range snapshot.Capabilities {
		owner, ok := capabilityPlugins[c.ID]
		if !ok || seen[c.ID] || !validCapabilityReason(c.Reason) {
			return invalid
		}
		seen[c.ID] = true
		ownerEnabled, ownerReady := true, true
		if owner != "" {
			ownerEnabled = plugins[owner].Enabled
			ownerReady = plugins[owner].Status == contract.PluginStatusReady
		}
		if c.ID == "workspace.git-workbench" {
			ownerEnabled = ownerEnabled && plugins["code"].Enabled
			ownerReady = ownerReady && plugins["code"].Status == contract.PluginStatusReady
		}
		if c.Available {
			if c.Reason != "" || !ownerReady {
				return invalid
			}
		} else if c.Reason == "" || owner == "" {
			return invalid
		} else if !ownerEnabled && c.Reason != contract.CapabilityReasonPluginDisabled {
			return invalid
		} else if ownerEnabled && c.Reason == contract.CapabilityReasonPluginDisabled {
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
	case "", contract.CapabilityReasonPluginDisabled, contract.CapabilityReasonConfigurationOff,
		contract.CapabilityReasonUnsupported, contract.CapabilityReasonStorageUnavailable,
		contract.CapabilityReasonCommandUnavailable, contract.CapabilityReasonServiceUnavailable:
		return true
	default:
		return false
	}
}

// CapabilityAvailable consults only a validated snapshot; absent IDs deny access.
func CapabilityAvailable(snapshot contract.Capabilities, id string) bool {
	if _, known := capabilityPlugins[id]; !known {
		return false
	}
	for _, c := range snapshot.Capabilities {
		if c.ID == id {
			return c.Available
		}
	}
	return false
}

// PluginEnabled consults a validated snapshot for diagnostic surfaces that
// remain useful even when the plugin's business capability is unavailable.
func PluginEnabled(snapshot contract.Capabilities, id string) bool {
	for _, p := range snapshot.Plugins {
		if p.ID == id {
			return p.Enabled
		}
	}
	return false
}

// RequirePlugin allows diagnostics for selected but unavailable plugins, while
// still failing closed on invalid discovery or disabled plugins.
func (c *Client) RequirePlugin(ctx context.Context, id string) error {
	snapshot, err := c.Capabilities(ctx)
	if err != nil {
		return err
	}
	if PluginEnabled(snapshot, id) {
		return nil
	}
	return &CapabilityError{ID: id, Reason: contract.CapabilityReasonPluginDisabled}
}

// RequireCapability reports a disabled plugin without guessing from a 404.
func (c *Client) RequireCapability(ctx context.Context, id string) error {
	snapshot, err := c.Capabilities(ctx)
	if err != nil {
		return err
	}
	return RequireSnapshotCapability(snapshot, id)
}

func RequireSnapshotCapability(snapshot contract.Capabilities, id string) error {
	if _, known := capabilityPlugins[id]; !known {
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
	owner, known := capabilityPlugins[id]
	if known && owner != "" {
		for _, p := range snapshot.Plugins {
			if p.ID == owner && !p.Enabled {
				return &CapabilityError{ID: id, Reason: contract.CapabilityReasonPluginDisabled}
			}
			if id == "workspace.git-workbench" && p.ID == "code" && !p.Enabled {
				return &CapabilityError{ID: id, Reason: contract.CapabilityReasonPluginDisabled}
			}
		}
	}
	return &CapabilityError{ID: id, Reason: contract.CapabilityReasonServiceUnavailable}
}
