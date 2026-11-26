package contract

import (
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

// Relay protocol types retain their versioned cross-service wire shape.
type RelayICEEndpoint = protocol.ICEEndpoint
type RelayBrowserICERequest = protocol.BrowserICERequest

type RelayStatus struct {
	Operation               RelayOperation           `json:"operation"`
	Enabled                 bool                     `json:"enabled"`
	Paired                  bool                     `json:"paired"`
	Pairing                 bool                     `json:"pairing"`
	State                   string                   `json:"state"`
	Transport               string                   `json:"transport"`
	TransportSelection      *RelayTransportSelection `json:"transportSelection,omitempty"`
	ControlReady            bool                     `json:"controlReady"`
	DataReady               bool                     `json:"dataReady"`
	DataCheckedAt           *time.Time               `json:"dataCheckedAt,omitempty"`
	PublicURL               string                   `json:"publicUrl,omitempty"`
	BoundPublicURL          string                   `json:"boundPublicUrl"`
	HybridSupported         bool                     `json:"hybridSupported"`
	HybridUnavailableReason string                   `json:"hybridUnavailableReason"`
	Error                   string                   `json:"error"`
	ReconnectReason         string                   `json:"reconnectReason"`
	TCPAlgorithm            string                   `json:"tcpAlgorithm"`
	StorageAvailable        bool                     `json:"storageAvailable"`
	TokenStrong             bool                     `json:"tokenStrong"`
	ControlEndpoint         string                   `json:"controlEndpoint"`
	ControlSource           string                   `json:"controlSource"`
	ControlReachability     string                   `json:"controlReachability"`
	DataReachability        string                   `json:"dataReachability"`
	ConnectorURL            string                   `json:"connectorUrl"`
	DataOrigin              string                   `json:"dataOrigin"`
	AssignmentRevision      string                   `json:"assignmentRevision"`
	ConfigRevision          string                   `json:"configRevision"`
	AppliedRevision         string                   `json:"appliedRevision"`
	ReadinessError          string                   `json:"readinessError"`
	OwnerID                 string                   `json:"ownerId"`
	OwnerLogin              string                   `json:"ownerLogin"`
	NodeID                  string                   `json:"nodeId"`
	Capabilities            []string                 `json:"capabilities"`
	AssignmentSource        string                   `json:"assignmentSource"`
	Sessions                []RelaySessionDiagnostic `json:"sessions"`
}

// TransportSelection describes host-to-node data only. Browser paths and the
// authenticated management connection have independent diagnostics.
type RelayTransportSelection struct {
	Supported   bool                   `json:"supported"`
	Preferred   protocol.FRPTransport  `json:"preferred"`
	Active      protocol.FRPTransport  `json:"active,omitempty"`
	Pending     protocol.FRPTransport  `json:"pending,omitempty"`
	LastAttempt *RelayTransportAttempt `json:"lastAttempt,omitempty"`
}

type RelayTransportAttempt struct {
	NodeID      string                `json:"nodeId"`
	Protocol    protocol.FRPTransport `json:"protocol"`
	Stage       string                `json:"stage"`
	Error       string                `json:"error"`
	DurationMS  int64                 `json:"durationMs"`
	CompletedAt time.Time             `json:"completedAt"`
}

type RelayTransportRequest struct {
	Protocol protocol.FRPTransport `json:"protocol"`
}

type RelaySessionDiagnostic struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	CandidateType string `json:"candidateType"`
	Transport     string `json:"transport"`
	Reason        string `json:"reason"`
}

type RelayUpdate struct {
	Enabled       *bool `json:"enabled"`
	ConfirmPublic bool  `json:"confirmPublic"`
}

type RelayEnrollmentRequest struct {
	ConfirmPublic bool `json:"confirmPublic"`
}

type RelayEnrollmentCancel struct{}

type RelayPairing struct {
	Phase               string `json:"phase"`
	UserCode            string `json:"userCode"`
	VerificationURL     string `json:"verificationUrl"`
	ExpiresAt           int64  `json:"expiresAt"`
	PollIntervalSeconds uint32 `json:"pollIntervalSeconds"`
	RetryAt             int64  `json:"retryAt"`
	Error               string `json:"error"`
}

type RelayOperation struct {
	Kind         string `json:"kind"`
	Phase        string `json:"phase"`
	Error        string `json:"error"`
	Deleted      bool   `json:"deleted"`
	AccessClosed bool   `json:"accessClosed"`
}
type RelayAuthSession struct {
	Authenticated bool   `json:"authenticated"`
	CSRFToken     string `json:"csrfToken"`
	ExpiresAt     int64  `json:"expiresAt"`
	Relay         bool   `json:"relay,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

type RelayAuthFailure struct {
	Error  string `json:"error"`
	Relay  bool   `json:"relay"`
	Reason string `json:"reason"`
}
