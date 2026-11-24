package wire

import (
	"time"

	"github.com/rainchan/webmux/internal/auth"
	"github.com/rainchan/webmux/internal/contract"
)

func AuthSummary(s auth.InspectSummary) contract.AuthSummary {
	return contract.AuthSummary{
		State:                  s.State,
		OnboardingCompleted:    s.OnboardingCompleted,
		PasskeyCount:           s.PasskeyCount,
		RPIDs:                  append([]string(nil), s.RPIDs...),
		SessionCount:           s.SessionCount,
		RecoveryCodesRemaining: s.RecoveryCodesRemaining,
		APITokenCount:          s.APITokenCount,
		Error:                  s.Error,
	}
}

func AuthStatus(s auth.StatusSnapshot, hostname string) contract.AuthStatusResponse {
	return contract.AuthStatusResponse{
		State:                  s.State,
		RPID:                   s.RPID,
		RPIDValid:              s.RPIDValid,
		RPIDError:              s.RPIDError,
		HasPasskeyForRPID:      s.HasPasskeyForRPID,
		OnboardingCompleted:    s.OnboardingCompleted,
		PasskeyCount:           s.PasskeyCount,
		SessionCount:           s.SessionCount,
		RecoveryCodesRemaining: s.RecoveryCodesRemaining,
		Hostname:               hostname,
		Error:                  s.Error,
	}
}

func WebAuthnCreation(o auth.RegistrationOptions) contract.WebAuthnCreationOptions {
	exclude := o.ExcludeIDs
	if exclude == nil {
		exclude = []string{}
	}
	algs := o.PubKeyAlgs
	if algs == nil {
		algs = []int64{}
	}
	return contract.WebAuthnCreationOptions{
		ChallengeID: o.ChallengeID,
		Challenge:   o.Challenge,
		RPID:        o.RPID,
		RPName:      o.RPName,
		UserID:      o.UserID,
		UserName:    o.UserName,
		DisplayName: o.DisplayName,
		ExcludeIDs:  exclude,
		PubKeyAlgs:  algs,
		TimeoutMS:   o.TimeoutMS,
	}
}

func WebAuthnAssertion(o auth.AssertionOptions) contract.WebAuthnAssertionOptions {
	allow := o.AllowIDs
	if allow == nil {
		allow = []string{}
	}
	return contract.WebAuthnAssertionOptions{
		ChallengeID: o.ChallengeID,
		Challenge:   o.Challenge,
		RPID:        o.RPID,
		AllowIDs:    allow,
		TimeoutMS:   o.TimeoutMS,
	}
}

func AuthPasskey(p auth.PasskeyInfo) contract.AuthPasskey {
	return contract.AuthPasskey{
		ID:             p.ID,
		RPID:           p.RPID,
		Name:           p.Name,
		Transports:     append([]string(nil), p.Transports...),
		BackupEligible: p.BackupEligible,
		BackupState:    p.BackupState,
		CreatedAt:      p.CreatedAt,
		LastUsedAt:     p.LastUsedAt,
	}
}

func AuthPasskeys(items []auth.PasskeyInfo) contract.PasskeyListResponse {
	out := make([]contract.AuthPasskey, len(items))
	for i, item := range items {
		out[i] = AuthPasskey(item)
	}
	return contract.PasskeyListResponse{Passkeys: out}
}

func AuthSession(s auth.SessionInfo) contract.AuthSession {
	return contract.AuthSession{
		ID:         s.ID,
		Name:       s.Name,
		RPID:       s.RPID,
		Current:    s.Current,
		CreatedAt:  s.CreatedAt,
		LastUsedAt: s.LastUsedAt,
		ExpiresAt:  s.ExpiresAt,
	}
}

func AuthSessions(items []auth.SessionInfo) contract.SessionListResponse {
	out := make([]contract.AuthSession, len(items))
	for i, item := range items {
		out[i] = AuthSession(item)
	}
	return contract.SessionListResponse{Sessions: out}
}

func AuthAPIToken(t auth.APITokenInfo) contract.AuthAPIToken {
	return contract.AuthAPIToken{
		ID:         t.ID,
		Name:       t.Name,
		CreatedAt:  t.CreatedAt,
		LastUsedAt: t.LastUsedAt,
		ExpiresAt:  t.ExpiresAt,
	}
}

func AuthAPITokens(items []auth.APITokenInfo) contract.APITokenListResponse {
	out := make([]contract.AuthAPIToken, len(items))
	for i, item := range items {
		out[i] = AuthAPIToken(item)
	}
	return contract.APITokenListResponse{Tokens: out}
}

func AuthOnboardingFinish(r auth.OnboardingCompleteResult) contract.OnboardingFinishResponse {
	return contract.OnboardingFinishResponse{
		SessionToken:  r.SessionToken,
		Session:       AuthSession(r.Session),
		Passkey:       AuthPasskey(r.Passkey),
		RecoveryCodes: append([]string{}, r.RecoveryCodes...),
	}
}

func AuthLogin(r auth.LoginResult, pk *auth.PasskeyInfo) contract.AuthLoginResponse {
	out := contract.AuthLoginResponse{
		SessionToken:           r.SessionToken,
		Session:                AuthSession(r.Session),
		RecoveryCodesRemaining: r.RecoveryCodesRemaining,
	}
	if pk != nil {
		converted := AuthPasskey(*pk)
		out.Passkey = &converted
	}
	return out
}

func AuthStepUp(r auth.StepUpResult) contract.StepUpFinishResponse {
	return contract.StepUpFinishResponse{
		StepUpToken: r.StepUpToken,
		ExpiresAt:   r.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}
}
