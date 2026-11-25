package relay

import "time"

type BindingAuthorization struct {
	Binding            Binding   `json:"binding"`
	PublicKey          PublicKey `json:"publicKey"`
	AssignmentRevision Counter   `json:"assignmentRevision"`
	Limits             Limits    `json:"limits"`
}

// AuthorizationSync is a full replacement, independent of configuration.
// A node must first close access absent from this snapshot, then acknowledge.
// Capacity is bounded to MaxBindings per node; no silent truncation or paging.
type AuthorizationSync struct {
	Revision Counter                `json:"revision"`
	Bindings []BindingAuthorization `json:"bindings"`
}

type SyncAck struct {
	Revision         Counter `json:"revision"`
	AccessReconciled bool    `json:"accessReconciled"`
	// FRPReconciled confirms prior-channel frp registrations are physically
	// closed. Control may honor it only on the first correlated sync ack.
	FRPReconciled bool `json:"frpReconciled,omitempty"`
}

// NodeHello carries no self-asserted identity revision. The control service
// resolves the registered identity from the verified mTLS certificate.
type NodeHello struct{}

type ConnectorAuthorization struct {
	Scope Scope `json:"scope"`
	Role  Role  `json:"role"`
}

type NodeType string

const (
	NodeHelloType      NodeType = "hello"
	NodeConfigType     NodeType = "config"
	NodeConfigAckType  NodeType = "config.ack"
	NodeActivateType   NodeType = "activate"
	NodeSyncType       NodeType = "authorization.sync"
	NodeSyncAckType    NodeType = "authorization.ack"
	NodeAuthorizeType  NodeType = "connector.authorize"
	NodeAuthorizedType NodeType = "connector.authorized"
	NodeForwardType    NodeType = "host.control"
	NodeRevokeType     NodeType = "revoke"
	NodeRevokeAckType  NodeType = "revoke.ack"
	NodeFailureType    NodeType = "error"
	NodePingType       NodeType = "ping"
	NodePongType       NodeType = "pong"
	NodeFRPAllocate    NodeType = "frp.allocate"
	NodeFRPRenew       NodeType = "frp.renew"
	NodeFRPGrant       NodeType = "frp.grant"
	// NodeFRPConfirm acknowledges installation of a prepared grant. Control
	// echoes the exact allocation and revision before the node releases it.
	NodeFRPConfirm       NodeType = "frp.confirm"
	NodeTransportRequest NodeType = "frp.transport.request"
	NodeTransportResult  NodeType = "frp.transport.result"
)

// NodeMessage is UTF-8 JSON in one WSS text message, bounded by
// MaxSnapshotBytes. Every message is fenced by both process epochs, its target
// node, control audience and a per-direction strictly increasing sequence.
// Hello alone has an empty ControlEpoch: the authenticated response establishes
// it. A different control epoch requires closing old access and a fresh channel.
type NodeMessage struct {
	Version       Version                 `json:"version"`
	Type          NodeType                `json:"type"`
	RequestID     Token                   `json:"requestId"`
	NodeID        NodeID                  `json:"nodeId"`
	Audience      string                  `json:"audience"`
	ControlEpoch  Token                   `json:"controlEpoch,omitempty"`
	NodeEpoch     Token                   `json:"nodeEpoch"`
	Sequence      Counter                 `json:"sequence"`
	ExpiresAt     int64                   `json:"expiresAt"`
	Hello         *NodeHello              `json:"hello,omitempty"`
	Config        *ConfigSnapshot         `json:"config,omitempty"`
	ConfigAck     *ConfigAck              `json:"configAck,omitempty"`
	Activate      *RestartApproval        `json:"activate,omitempty"`
	Sync          *AuthorizationSync      `json:"sync,omitempty"`
	SyncAck       *SyncAck                `json:"syncAck,omitempty"`
	Authorize     *ConnectorAuthorization `json:"authorize,omitempty"`
	Authorized    *ConnectorReady         `json:"authorized,omitempty"`
	Forward       *ControlMessage         `json:"forward,omitempty"`
	Revocation    *Revocation             `json:"revocation,omitempty"`
	RevocationAck *RevocationAck          `json:"revocationAck,omitempty"`
	Error         *Error                  `json:"error,omitempty"`

	// Scope fences private frp requests and replies. It is absent on
	// existing messages, which carry any scope in their own payload.
	Scope             *Scope             `json:"scope,omitempty"`
	AllocationRequest *AllocationRequest `json:"allocationRequest,omitempty"`
	AllocationRenew   *AllocationRenew   `json:"allocationRenew,omitempty"`
	AllocationGrant   *AllocationGrant   `json:"allocationGrant,omitempty"`
	TransportRequest  *TransportRequest  `json:"transportRequest,omitempty"`
	TransportResult   *TransportResult   `json:"transportResult,omitempty"`
}

// Validate checks envelope and closed payload selection. Receivers must also
// validate nested config/host payloads against their own current state, and
// compare Sequence to the last accepted sequence on THIS channel.
func (m NodeMessage) Validate(node NodeID, controlAudience string, controlEpoch, nodeEpoch Token, now time.Time) error {
	if m.Version != V1 {
		return Error{Code: Incompatible}
	}
	if err := m.NodeID.Validate(); err != nil {
		return err
	}
	if m.NodeID != node {
		return Error{Code: Forbidden}
	}
	if !audience(m.Audience) || m.Audience != controlAudience {
		return Error{Code: WrongAudience}
	}
	if err := m.RequestID.Validate(); err != nil {
		return err
	}
	if err := m.NodeEpoch.Validate(); err != nil {
		return err
	}
	if m.NodeEpoch != nodeEpoch || m.Sequence == 0 {
		return Error{Code: StaleGeneration}
	}
	if !validUntil(now, m.ExpiresAt) {
		return Error{Code: Expired}
	}
	if m.Type == NodeHelloType {
		if m.ControlEpoch != "" {
			return Error{Code: Invalid}
		}
	} else {
		if err := m.ControlEpoch.Validate(); err != nil {
			return err
		}
		if m.ControlEpoch != controlEpoch {
			return Error{Code: StaleGeneration}
		}
	}
	switch m.Type {
	case NodeFRPAllocate, NodeFRPRenew, NodeFRPGrant, NodeFRPConfirm, NodeTransportRequest, NodeTransportResult:
		if m.Scope == nil {
			return Error{Code: Invalid}
		}
		if err := m.Scope.Validate(); err != nil {
			return err
		}
		if m.Scope.ControlEpoch != m.ControlEpoch || m.Scope.NodeEpoch != m.NodeEpoch {
			return Error{Code: StaleGeneration}
		}
	default:
		if m.Scope != nil {
			return Error{Code: Invalid}
		}
	}
	count := 0
	for _, present := range [...]bool{
		m.Hello != nil, m.Config != nil, m.ConfigAck != nil, m.Activate != nil, m.Sync != nil, m.SyncAck != nil,
		m.Authorize != nil, m.Authorized != nil, m.Forward != nil, m.Revocation != nil, m.RevocationAck != nil,
		m.Error != nil, m.AllocationRequest != nil, m.AllocationRenew != nil, m.AllocationGrant != nil,
		m.TransportRequest != nil, m.TransportResult != nil,
	} {
		if present {
			count++
		}
	}
	if m.Type == NodePingType || m.Type == NodePongType {
		if count != 0 {
			return Error{Code: Invalid}
		}
		return nil
	}
	if count != 1 {
		return Error{Code: Invalid}
	}
	switch m.Type {
	case NodeHelloType:
		if m.Hello != nil {
			return nil
		}
	case NodeConfigType:
		if m.Config != nil && m.Config.Payload.NodeID == node {
			return nil
		}
	case NodeConfigAckType:
		if m.ConfigAck != nil && m.ConfigAck.NodeID == node {
			return nil
		}
	case NodeActivateType:
		if m.Activate != nil && m.Activate.NodeID == node {
			return nil
		}
	case NodeSyncType:
		if m.Sync != nil {
			return m.Sync.Validate()
		}
	case NodeSyncAckType:
		if m.SyncAck != nil && m.SyncAck.Revision != 0 && m.SyncAck.AccessReconciled {
			return nil
		}
	case NodeAuthorizeType:
		if m.Authorize != nil && m.Authorize.Role.Valid() {
			if m.Authorize.Scope.ControlEpoch != m.ControlEpoch || m.Authorize.Scope.NodeEpoch != m.NodeEpoch {
				return Error{Code: StaleGeneration}
			}
			return m.Authorize.Scope.Validate()
		}
	case NodeAuthorizedType:
		if m.Authorized != nil && m.Authorized.Version == V1 && m.Authorized.Role.Valid() {
			a := m.Authorized
			if a.ControlEpoch != m.ControlEpoch || a.NodeEpoch != m.NodeEpoch {
				return Error{Code: StaleGeneration}
			}
			return (Scope{HostID: a.HostID, BindingGeneration: a.BindingGeneration, ConnectorGeneration: a.ConnectorGeneration, ControlEpoch: a.ControlEpoch, NodeEpoch: a.NodeEpoch}).Validate()
		}
	case NodeForwardType:
		if m.Forward != nil && m.Forward.Scope.ControlEpoch == m.ControlEpoch && m.Forward.Scope.NodeEpoch == m.NodeEpoch {
			switch m.Forward.Type {
			case ControlFRPAllocate, ControlFRPRenew, ControlFRPGrant, ControlFRPStatus, ControlTransportRequest, ControlTransportResult:
				return Error{Code: Invalid}
			}
			return m.Forward.Validate(m.Forward.Scope, now)
		}
	case NodeFRPAllocate:
		if m.AllocationRequest != nil {
			return m.AllocationRequest.validate(true)
		}
	case NodeFRPRenew, NodeFRPConfirm:
		if m.AllocationRenew != nil {
			return validateAllocationRef(m.AllocationRenew.AllocationID, m.AllocationRenew.GrantRevision)
		}
	case NodeFRPGrant:
		if m.AllocationGrant != nil && m.AllocationGrant.Transport == nil {
			return m.AllocationGrant.validate(now, true)
		}
	case NodeTransportRequest:
		if m.TransportRequest != nil {
			return m.TransportRequest.Validate(true)
		}
	case NodeTransportResult:
		if m.TransportResult != nil {
			return m.TransportResult.validate(now, true)
		}
	case NodeRevokeType:
		if m.Revocation != nil {
			return m.Revocation.Validate()
		}
	case NodeRevokeAckType:
		if m.RevocationAck != nil && m.RevocationAck.AccessClosed {
			return m.RevocationAck.ID.Validate()
		}
	case NodeFailureType:
		if m.Error != nil && m.Error.Code.Valid() {
			return nil
		}
	}
	return Error{Code: Invalid}
}

func (s AuthorizationSync) Validate() error {
	if s.Revision == 0 || s.Bindings == nil || len(s.Bindings) > MaxBindings {
		return Error{Code: Invalid}
	}
	var last HostID
	for _, a := range s.Bindings {
		if err := a.Binding.Validate(); err != nil {
			return err
		}
		if a.Binding.HostID <= last || a.AssignmentRevision == 0 {
			return Error{Code: Invalid}
		}
		if _, err := a.PublicKey.Parse(); err != nil {
			return err
		}
		last = a.Binding.HostID
	}
	return nil
}
