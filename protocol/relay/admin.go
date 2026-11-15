package relay

// NodeRegistration is accepted only from the deployment-authenticated admin
// surface. GitHub console principals, hosts and nodes cannot call it.
type NodeRegistration struct {
	Version  Version      `json:"version"`
	Identity NodeIdentity `json:"identity"`
	Label    string       `json:"label"`
}

type NodeIdentityRotation struct {
	Version          Version      `json:"version"`
	ExpectedRevision Counter      `json:"expectedRevision"`
	Identity         NodeIdentity `json:"identity"`
}

type NodeConfigUpdate struct {
	Version          Version    `json:"version"`
	ExpectedRevision Counter    `json:"expectedRevision"`
	Config           NodeConfig `json:"config"`
}

type AssignmentUpdate struct {
	Version          Version `json:"version"`
	ExpectedRevision Counter `json:"expectedRevision"`
	WSSNodeID        NodeID  `json:"wssNodeId"`
	TURNNodeID       NodeID  `json:"turnNodeId,omitempty"`
}

type InvitationUpdate struct {
	Version Version `json:"version"`
	Invited bool    `json:"invited"`
}

// SecretUpload is secret-bearing, admin-only and never echoed. The HTTP
// controller accepts at most MaxControlBytes and disables request/body logging.
// Version belongs to Ref; new exact versions never overwrite existing values.
type SecretUpload struct {
	Version        Version   `json:"version"`
	Ref            SecretRef `json:"ref"`
	Value          string    `json:"value,omitempty"`
	CertificatePEM string    `json:"certificatePem,omitempty"`
	PrivateKeyPEM  string    `json:"privateKeyPem,omitempty"`
}

type NodeStatus struct {
	Version               Version    `json:"version"`
	NodeID                NodeID     `json:"nodeId"`
	IdentityRevision      Counter    `json:"identityRevision"`
	DesiredRevision       Counter    `json:"desiredRevision"`
	DesiredDigest         Digest     `json:"desiredDigest"`
	Status                *ConfigAck `json:"status,omitempty"`
	Online                bool       `json:"online"`
	AuthorizationRevision Counter    `json:"authorizationRevision"`
}
