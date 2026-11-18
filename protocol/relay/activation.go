package relay

import "time"

func (e Enrollment) Validate(now time.Time) error {
	if e.Version != BindingAPIVersion {
		return Error{Code: Incompatible}
	}
	if err := e.Handle.Validate(); err != nil {
		return err
	}
	if len(e.UserCode) < 1 || len(e.UserCode) > 64 || e.PollIntervalSeconds < 1 || e.PollIntervalSeconds > 60 || !validUntil(now, e.ExpiresAt) {
		return Error{Code: Invalid}
	}
	for _, c := range e.UserCode {
		if c < 0x21 || c > 0x7e {
			return Error{Code: Invalid}
		}
	}
	_, err := secureURL(e.VerificationURL, "https", false)
	return err
}

func (r EnrollmentPoll) Validate() error {
	if r.Version != BindingAPIVersion {
		return Error{Code: Incompatible}
	}
	return r.Handle.Validate()
}

func (r BindingDeletionResult) Validate() error {
	if r.Version != BindingAPIVersion {
		return Error{Code: Incompatible}
	}
	if !r.Deleted {
		return Error{Code: Invalid}
	}
	return r.RevocationID.Validate()
}

func (r HostChallengeRequest) Validate() error {
	if r.Version != V1 {
		return Error{Code: Incompatible}
	}
	if err := r.HostID.Validate(); err != nil {
		return err
	}
	if r.Purpose != PurposeAssignment && r.Purpose != PurposeDelete {
		return Error{Code: Invalid}
	}
	return nil
}

func (r RestartApproval) Validate(s ConfigSnapshot) error {
	if r.NodeID != s.Payload.NodeID || r.Revision != s.Payload.Revision || r.Digest != s.Digest {
		return Error{Code: Conflict}
	}
	if len(r.Components) < 1 || len(r.Components) > 2 {
		return Error{Code: InvalidConfig}
	}
	var prior Component
	for _, c := range r.Components {
		if c <= prior || c != ComponentRelay && c != ComponentCoturn {
			return Error{Code: InvalidConfig}
		}
		prior = c
	}
	return nil
}

func configSecrets(c NodeConfig, component Component) []SecretRef {
	var refs []SecretRef
	if component == ComponentRelay && c.WSS != nil {
		refs = append(refs, c.WSS.TLSSecret)
	}
	if component == ComponentCoturn && c.TURN != nil {
		refs = append(refs, c.TURN.AuthSecret)
		if c.TURN.TLSSecret != nil {
			refs = append(refs, *c.TURN.TLSSecret)
		}
	}
	return refs
}

func (a ConfigAck) Validate(s ConfigSnapshot) error {
	if a.NodeID != s.Payload.NodeID || a.DesiredRevision != s.Payload.Revision || a.DesiredDigest != s.Digest {
		return Error{Code: Conflict}
	}
	if a.AppliedRevision > a.DesiredRevision || len(a.Components) < 1 || len(a.Components) > 2 || a.Error != "" && !a.Error.Valid() {
		return Error{Code: InvalidConfig}
	}
	if a.AppliedRevision == 0 {
		if a.AppliedDigest != "" {
			return Error{Code: InvalidConfig}
		}
	} else if err := a.AppliedDigest.Validate(); err != nil {
		return err
	}
	switch a.State {
	case "applied":
		if a.AppliedRevision != a.DesiredRevision || a.AppliedDigest != a.DesiredDigest || a.Error != "" {
			return Error{Code: InvalidConfig}
		}
	case "degraded":
		if a.AppliedRevision != a.DesiredRevision || a.AppliedDigest != a.DesiredDigest {
			return Error{Code: InvalidConfig}
		}
	case "pending-restart", "failed":
		if a.AppliedRevision >= a.DesiredRevision {
			return Error{Code: InvalidConfig}
		}
	default:
		return Error{Code: InvalidConfig}
	}
	var prior Component
	seenRelay, seenCoturn := false, false
	for _, c := range a.Components {
		if c.Component <= prior || c.Component != ComponentRelay && c.Component != ComponentCoturn || c.AppliedRevision > a.DesiredRevision || c.Error != "" && !c.Error.Valid() || c.ActiveSecrets == nil {
			return Error{Code: InvalidConfig}
		}
		prior = c.Component
		seenRelay = seenRelay || c.Component == ComponentRelay
		seenCoturn = seenCoturn || c.Component == ComponentCoturn
		for i, ref := range c.ActiveSecrets {
			if err := ref.Validate(); err != nil {
				return err
			}
			for _, old := range c.ActiveSecrets[:i] {
				if old == ref {
					return Error{Code: InvalidConfig}
				}
			}
		}
		if a.State == "applied" {
			if !c.Ready || c.AppliedRevision != a.DesiredRevision || c.Error != "" {
				return Error{Code: InvalidConfig}
			}
			want := configSecrets(s.Payload.Config, c.Component)
			if len(c.ActiveSecrets) != len(want) {
				return Error{Code: InvalidConfig}
			}
			for _, ref := range want {
				found := false
				for _, active := range c.ActiveSecrets {
					found = found || ref == active
				}
				if !found {
					return Error{Code: InvalidConfig}
				}
			}
		}
	}
	if a.State == "applied" && (!seenRelay || seenCoturn != (s.Payload.Config.TURN != nil)) {
		return Error{Code: InvalidConfig}
	}
	return nil
}

func (r SecretRequest) Validate(s ConfigSnapshot) error {
	if r.Version != V1 {
		return Error{Code: Incompatible}
	}
	if r.NodeID != s.Payload.NodeID || r.ConfigRevision != s.Payload.Revision {
		return Error{Code: Forbidden}
	}
	if err := r.ControlEpoch.Validate(); err != nil {
		return err
	}
	if err := r.NodeEpoch.Validate(); err != nil {
		return err
	}
	if err := r.Ref.Validate(); err != nil {
		return err
	}
	for _, c := range []Component{ComponentRelay, ComponentCoturn} {
		for _, ref := range configSecrets(s.Payload.Config, c) {
			if ref == r.Ref {
				return nil
			}
		}
	}
	return Error{Code: Forbidden}
}

func (d SecretDelivery) Validate(r SecretRequest) error {
	if d.Version != V1 {
		return Error{Code: Incompatible}
	}
	if d.NodeID != r.NodeID || d.ControlEpoch != r.ControlEpoch || d.NodeEpoch != r.NodeEpoch || d.ConfigRevision != r.ConfigRevision || d.Ref != r.Ref {
		return Error{Code: Forbidden}
	}
	if d.Ref.Purpose == SecretTURN {
		if d.CertificatePEM != "" || d.PrivateKeyPEM != "" {
			return Error{Code: Invalid}
		}
		_, err := decodeFixed(d.Value, 32)
		return err
	}
	if d.Ref.Purpose != SecretTLS || d.Value != "" || len(d.CertificatePEM) == 0 || len(d.CertificatePEM) > 32768 || len(d.PrivateKeyPEM) == 0 || len(d.PrivateKeyPEM) > 8192 {
		return Error{Code: Invalid}
	}
	return nil
}

func (r NodeRegistration) Validate() error {
	if r.Version != V1 {
		return Error{Code: Incompatible}
	}
	if r.Identity.Revision != 1 || !identifier(r.Label, 64) {
		return Error{Code: InvalidConfig}
	}
	return r.Identity.Validate()
}

func (r NodeIdentityRotation) Validate() error {
	if r.Version != V1 {
		return Error{Code: Incompatible}
	}
	if r.ExpectedRevision == 0 || r.Identity.Revision == 0 || r.Identity.Revision != r.ExpectedRevision+1 {
		return Error{Code: Conflict}
	}
	return r.Identity.Validate()
}

func (r NodeConfigUpdate) Validate() error {
	if r.Version != V1 {
		return Error{Code: Incompatible}
	}
	if r.ExpectedRevision == ^Counter(0) {
		return Error{Code: Conflict}
	}
	return r.Config.Validate()
}

func (r AssignmentUpdate) Validate() error {
	if r.Version != V1 {
		return Error{Code: Incompatible}
	}
	if r.ExpectedRevision == ^Counter(0) {
		return Error{Code: Conflict}
	}
	if err := r.WSSNodeID.Validate(); err != nil {
		return err
	}
	if r.TURNNodeID != "" {
		return r.TURNNodeID.Validate()
	}
	return nil
}
