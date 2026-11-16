package contract

import protocol "github.com/mzdluo123/webmux/protocol/relay"

// Relay protocol types retain their versioned cross-service wire shape.
type RelayEnrollment = protocol.Enrollment
type RelayEnrollmentResult = protocol.EnrollmentResult
type RelayICEEndpoint = protocol.ICEEndpoint
type RelayBrowserICERequest = protocol.BrowserICERequest

type RelayStatus struct {
	Enabled                 bool                     `json:"enabled"`
	Paired                  bool                     `json:"paired"`
	Pairing                 bool                     `json:"pairing"`
	State                   string                   `json:"state"`
	Transport               string                   `json:"transport"`
	BulkReady               bool                     `json:"bulkReady"`
	ControlReady            bool                     `json:"controlReady"`
	ReadyConnections        int                      `json:"readyConnections"`
	PublicURL               string                   `json:"publicUrl"`
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

type RelayEnrollmentPoll struct {
	Handle protocol.Token `json:"handle"`
}
type RelayAuthSession struct {
	Authenticated bool   `json:"authenticated"`
	CSRFToken     string `json:"csrfToken"`
	ExpiresAt     int64  `json:"expiresAt"`
}
