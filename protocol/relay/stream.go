package relay

import "time"

type ConnectorReady struct {
	Version             Version `json:"version"`
	HostID              HostID  `json:"hostId"`
	BindingGeneration   Counter `json:"bindingGeneration"`
	ConnectorGeneration Counter `json:"connectorGeneration"`
	Role                Role    `json:"role"`
	ControlEpoch        Token   `json:"controlEpoch"`
	NodeEpoch           Token   `json:"nodeEpoch"`
}

// Scope is supplied by the authenticated connection, not public HTTP headers.
type Scope struct {
	HostID              HostID  `json:"hostId"`
	BindingGeneration   Counter `json:"bindingGeneration"`
	ConnectorGeneration Counter `json:"connectorGeneration"`
	ControlEpoch        Token   `json:"controlEpoch"`
	NodeEpoch           Token   `json:"nodeEpoch"`
}

func (s Scope) Validate() error {
	if err := s.HostID.Validate(); err != nil {
		return err
	}
	if s.BindingGeneration == 0 || s.ConnectorGeneration == 0 {
		return Error{Code: Invalid}
	}
	if err := s.ControlEpoch.Validate(); err != nil {
		return err
	}
	return s.NodeEpoch.Validate()
}

type StreamKind string

const (
	StreamHTTP    StreamKind = "http"
	StreamControl StreamKind = "control"
)

// StreamHeader is a uint32 big-endian JSON byte length then exactly that many
// UTF-8 bytes. HTTP streams carry ordinary HTTP/1.1 after a StreamResult prefix;
// no destination, URL, port, Cookie or proxy identity can be supplied here.
type StreamHeader struct {
	Version   Version    `json:"version"`
	Kind      StreamKind `json:"kind"`
	RequestID Token      `json:"requestId"`
	Scope     Scope      `json:"scope"`
	Role      Role       `json:"role"`
}

func (h StreamHeader) Validate(expected Scope, role Role) error {
	if h.Version != V1 {
		return Error{Code: Incompatible}
	}
	if err := h.RequestID.Validate(); err != nil {
		return err
	}
	if err := h.Scope.Validate(); err != nil {
		return err
	}
	if h.Scope != expected {
		return Error{Code: StaleGeneration}
	}
	if !role.Valid() || h.Role != role || h.Kind != StreamHTTP && h.Kind != StreamControl || h.Kind == StreamControl && role != RoleControl {
		return Error{Code: Invalid}
	}
	return nil
}

type StreamResult struct {
	Version   Version `json:"version"`
	RequestID Token   `json:"requestId"`
	Error     *Error  `json:"error,omitempty"`
}

type ControlType string

const (
	ControlExchange   ControlType = "login.exchange"
	ControlGrant      ControlType = "login.grant"
	ControlICERequest ControlType = "ice.request"
	ControlICEConfig  ControlType = "ice.config"
	ControlRevoke     ControlType = "revoke"
	ControlRevokeAck  ControlType = "revoke.ack"
	ControlFailure    ControlType = "error"
	ControlPing       ControlType = "ping"
	ControlPong       ControlType = "pong"
	ControlGrow       ControlType = "transport.grow"
)

// ConnectionDemand requests a bounded target pool size, not an increment.
// Every added socket authenticates independently in the existing Scope/Role.
type ConnectionDemand struct {
	Role        Role   `json:"role"`
	Connections uint32 `json:"connections"`
}

type Revocation struct {
	ID Token `json:"id"`
	// Scope is exactly one of binding, console-session, grant, connector.
	Scope               string  `json:"scope"`
	HostID              HostID  `json:"hostId"`
	BindingGeneration   Counter `json:"bindingGeneration"`
	ConnectorGeneration Counter `json:"connectorGeneration,omitempty"`
	ConsoleSessionID    Token   `json:"consoleSessionId,omitempty"`
	GrantID             Token   `json:"grantId,omitempty"`
	Reason              string  `json:"reason"`
}

func (r Revocation) Validate() error {
	if err := r.ID.Validate(); err != nil {
		return err
	}
	if err := r.HostID.Validate(); err != nil {
		return err
	}
	if r.BindingGeneration == 0 {
		return Error{Code: Invalid}
	}
	switch r.Scope {
	case "binding":
		if r.ConnectorGeneration != 0 || r.ConsoleSessionID != "" || r.GrantID != "" {
			return Error{Code: Invalid}
		}
	case "connector":
		if r.ConnectorGeneration == 0 || r.ConsoleSessionID != "" || r.GrantID != "" {
			return Error{Code: Invalid}
		}
	case "console-session":
		if err := r.ConsoleSessionID.Validate(); err != nil {
			return err
		}
		if r.GrantID != "" || r.ConnectorGeneration != 0 {
			return Error{Code: Invalid}
		}
	case "grant":
		if err := r.GrantID.Validate(); err != nil {
			return err
		}
		if r.ConsoleSessionID != "" || r.ConnectorGeneration != 0 {
			return Error{Code: Invalid}
		}
	default:
		return Error{Code: Invalid}
	}
	switch r.Reason {
	case "logout", "expired", "unbound", "rekeyed", "uninvited", "disconnected", "control-restarted":
	default:
		return Error{Code: Invalid}
	}
	return nil
}

// RevocationAck is emitted AFTER denying new access and closing every derived
// HTTP/WS/P2P/forward connection in scope, not on receipt. It never proves TURN
// allocation destruction or PTY termination. PTYs MUST remain running.
type RevocationAck struct {
	ID                  Token   `json:"id"`
	HostID              HostID  `json:"hostId"`
	BindingGeneration   Counter `json:"bindingGeneration"`
	ConnectorGeneration Counter `json:"connectorGeneration"`
	AccessClosed        bool    `json:"accessClosed"`
}

// ControlMessage is a closed discriminated union on one long-lived control
// stream. Exactly one matching payload is present, except ping/pong (none).
// Responses echo RequestID; events use independent IDs. No implicit retries.
type ControlMessage struct {
	Version       Version           `json:"version"`
	Type          ControlType       `json:"type"`
	RequestID     Token             `json:"requestId"`
	Scope         Scope             `json:"scope"`
	ExpiresAt     int64             `json:"expiresAt"`
	Exchange      *LoginExchange    `json:"exchange,omitempty"`
	Grant         *BrowserGrant     `json:"grant,omitempty"`
	ICERequest    *ICERequest       `json:"iceRequest,omitempty"`
	ICEConfig     *ICEConfig        `json:"iceConfig,omitempty"`
	Revocation    *Revocation       `json:"revocation,omitempty"`
	RevocationAck *RevocationAck    `json:"revocationAck,omitempty"`
	Grow          *ConnectionDemand `json:"grow,omitempty"`
	Error         *Error            `json:"error,omitempty"`
}

func (m ControlMessage) Validate(expected Scope, now time.Time) error {
	if m.Version != V1 {
		return Error{Code: Incompatible}
	}
	if err := m.RequestID.Validate(); err != nil {
		return err
	}
	if err := m.Scope.Validate(); err != nil {
		return err
	}
	if m.Scope != expected {
		return Error{Code: StaleGeneration}
	}
	if !validUntil(now, m.ExpiresAt, ControlMessageTTL) {
		return Error{Code: Expired}
	}
	count := 0
	for _, present := range []bool{m.Exchange != nil, m.Grant != nil, m.ICERequest != nil, m.ICEConfig != nil, m.Revocation != nil, m.RevocationAck != nil, m.Error != nil, m.Grow != nil} {
		if present {
			count++
		}
	}
	if m.Type == ControlPing || m.Type == ControlPong {
		if count != 0 {
			return Error{Code: Invalid}
		}
		return nil
	}
	if count != 1 {
		return Error{Code: Invalid}
	}
	switch m.Type {
	case ControlGrow:
		if m.Grow != nil && m.Grow.Role.Valid() && m.Grow.Connections > 0 && m.Grow.Connections <= MaxRoleConnections {
			return nil
		}
	case ControlExchange:
		if m.Exchange != nil {
			return m.Exchange.Validate()
		}
	case ControlGrant:
		if m.Grant != nil {
			g := m.Grant
			if g.HostID != expected.HostID || g.BindingGeneration != expected.BindingGeneration || g.ConnectorGeneration != expected.ConnectorGeneration || g.ControlEpoch != expected.ControlEpoch || g.NodeEpoch != expected.NodeEpoch {
				return Error{Code: StaleGeneration}
			}
			return g.Validate(now)
		}
	case ControlICERequest:
		if m.ICERequest != nil {
			if err := m.ICERequest.GrantID.Validate(); err != nil {
				return err
			}
			return m.ICERequest.PeerConnectionID.Validate()
		}
	case ControlICEConfig:
		if m.ICEConfig != nil {
			return m.ICEConfig.Validate(now)
		}
	case ControlRevoke:
		if m.Revocation != nil {
			if m.Revocation.HostID != expected.HostID || m.Revocation.BindingGeneration != expected.BindingGeneration {
				return Error{Code: StaleGeneration}
			}
			return m.Revocation.Validate()
		}
	case ControlRevokeAck:
		if m.RevocationAck != nil {
			a := m.RevocationAck
			if err := a.ID.Validate(); err != nil {
				return err
			}
			if a.HostID != expected.HostID || a.BindingGeneration != expected.BindingGeneration || a.ConnectorGeneration != expected.ConnectorGeneration || !a.AccessClosed {
				return Error{Code: Invalid}
			}
			return nil
		}
	case ControlFailure:
		if m.Error != nil && m.Error.Code.Valid() {
			return nil
		}
	}
	return Error{Code: Invalid}
}
