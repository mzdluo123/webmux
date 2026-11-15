package contract

const (
	PathCapabilities    = "/api/capabilities"
	CapabilitiesVersion = 2
	// CapabilitiesVersionV1 is the retired shape: a plugins field and the
	// plugin_disabled reason. Local clients still decode it for a transition
	// period; servers never emit it.
	CapabilitiesVersionV1 = 1
)

const (
	FeatureStatusDisabled    = "disabled"
	FeatureStatusReady       = "ready"
	FeatureStatusUnavailable = "unavailable"
)

// Capability reasons are safe protocol codes, never underlying error text,
// command arguments, credentials, or storage contents.
const (
	CapabilityReasonFeatureDisabled    = "feature_disabled"
	CapabilityReasonConfigurationOff   = "configuration_off"
	CapabilityReasonUnsupported        = "unsupported"
	CapabilityReasonStorageUnavailable = "storage_unavailable"
	CapabilityReasonCommandUnavailable = "command_unavailable"
	CapabilityReasonServiceUnavailable = "service_unavailable"
)

// Capabilities is the authenticated, current-instance discovery snapshot.
// Constructors must set Version to CapabilitiesVersion and initialize both
// arrays to non-nil slices, including for a terminal-core-only instance.
type Capabilities struct {
	Version      int             `json:"version"`
	Features     []FeatureStatus `json:"features"`
	Capabilities []Capability    `json:"capabilities"`
}

// FeatureStatus describes module initialization, not its business on/off setting.
type FeatureStatus struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
}

// Capability describes an entry point's actual availability after configuration,
// platform support, and service health have been considered.
type Capability struct {
	ID        string `json:"id"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}
