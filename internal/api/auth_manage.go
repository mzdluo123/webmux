package api

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/auth"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/wire"
)

func (s *Server) authManageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/logout", s.authLogout)
	mux.HandleFunc("POST /api/auth/step-up/begin", s.authStepUpBegin)
	mux.HandleFunc("POST /api/auth/step-up/finish", s.authStepUpFinish)
	mux.HandleFunc("GET /api/auth/passkeys", s.authListPasskeys)
	mux.HandleFunc("POST /api/auth/passkeys/begin", s.authAddPasskeyBegin)
	mux.HandleFunc("POST /api/auth/passkeys/finish", s.authAddPasskeyFinish)
	mux.HandleFunc("PATCH /api/auth/passkeys/{id}", s.authRenamePasskey)
	mux.HandleFunc("DELETE /api/auth/passkeys/{id}", s.authDeletePasskey)
	mux.HandleFunc("POST /api/auth/pair/create", s.authPairCreate)
	mux.HandleFunc("GET /api/auth/sessions", s.authListSessions)
	mux.HandleFunc("DELETE /api/auth/sessions/{id}", s.authRevokeSession)
	mux.HandleFunc("POST /api/auth/sessions/revoke-others", s.authRevokeOtherSessions)
	mux.HandleFunc("GET /api/auth/tokens", s.authListAPITokens)
	mux.HandleFunc("POST /api/auth/tokens", s.authCreateAPIToken)
	mux.HandleFunc("DELETE /api/auth/tokens/{id}", s.authRevokeAPIToken)
	mux.HandleFunc("POST /api/auth/recovery-codes/regenerate", s.authRegenerateRecovery)
}

func (s *Server) requireAuthService(w http.ResponseWriter) bool {
	if s.Auth == nil {
		failure(w, http.StatusNotFound, errors.New("passkey authentication is not enabled"))
		return false
	}
	return true
}

// requireStepUp enforces fresh Passkey re-authentication for sensitive security
// mutations. API tokens cannot manage Passkeys, pairing codes, or recovery codes.
func (s *Server) requireStepUp(w http.ResponseWriter, r *http.Request) (auth.Principal, bool) {
	if !s.requireAuthService(w) {
		return auth.Principal{}, false
	}
	if s.local {
		return auth.Principal{}, true
	}
	p, ok := principalFrom(r)
	if !ok || p.Kind != auth.PrincipalSession {
		failure(w, http.StatusForbidden, errors.New("browser passkey session required for this operation"))
		return auth.Principal{}, false
	}
	if !s.Auth.VerifyStepUp(p.SessionID, strings.TrimSpace(r.Header.Get(stepUpHeader))) {
		authFailure(w, auth.ErrStepUpRequired)
		return auth.Principal{}, false
	}
	return p, true
}

func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.requireAuthService(w) {
		return
	}
	if p, ok := principalFrom(r); ok && p.SessionID != "" {
		if err := s.Auth.Logout(p.SessionID); err != nil && !errors.Is(err, auth.ErrNotFound) {
			authFailure(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authStepUpBegin(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.requireAuthService(w) {
		return
	}
	p, ok := principalFrom(r)
	if !ok || p.Kind != auth.PrincipalSession {
		failure(w, http.StatusForbidden, errors.New("browser passkey session required for step-up verification"))
		return
	}
	opts, err := s.Auth.BeginStepUp(p.SessionID, s.webAuthnHost(r))
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, wire.WebAuthnAssertion(opts))
}

func (s *Server) authStepUpFinish(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.requireAuthService(w) {
		return
	}
	p, ok := principalFrom(r)
	if !ok || p.Kind != auth.PrincipalSession {
		failure(w, http.StatusForbidden, errors.New("browser passkey session required for step-up verification"))
		return
	}
	var req contract.AssertionFinishRequest
	if !decode(w, r, &req) {
		return
	}
	res, err := s.Auth.FinishStepUp(p.SessionID, auth.AssertionFinishInput{
		ChallengeID:       req.ChallengeID,
		CredentialID:      req.CredentialID,
		Origin:            s.webAuthnOrigin(r),
		ClientDataJSON:    req.ClientDataJSON,
		AuthenticatorData: req.AuthenticatorData,
		Signature:         req.Signature,
	})
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, wire.AuthStepUp(res))
}

func (s *Server) authListPasskeys(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.requireAuthService(w) {
		return
	}
	items, err := s.Auth.ListPasskeys()
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, wire.AuthPasskeys(items))
}

func (s *Server) authAddPasskeyBegin(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	p, ok := s.requireStepUp(w, r)
	if !ok {
		return
	}
	opts, err := s.Auth.BeginAddPasskey(p.SessionID, s.webAuthnHost(r))
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, wire.WebAuthnCreation(opts))
}

func (s *Server) authAddPasskeyFinish(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	p, ok := s.requireStepUp(w, r)
	if !ok {
		return
	}
	var req contract.PasskeyFinishRequest
	if !decode(w, r, &req) {
		return
	}
	pk, err := s.Auth.FinishAddPasskey(p.SessionID, auth.PasskeyFinishInput{
		ChallengeID:       req.ChallengeID,
		Name:              req.Name,
		Origin:            s.webAuthnOrigin(r),
		ClientDataJSON:    req.ClientDataJSON,
		AttestationObject: req.AttestationObject,
		Transports:        req.Transports,
	})
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusCreated, wire.AuthPasskey(pk))
}

func (s *Server) authRenamePasskey(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.requireAuthService(w) {
		return
	}
	var req contract.RenamePasskeyRequest
	if !decode(w, r, &req) {
		return
	}
	pk, err := s.Auth.RenamePasskey(r.PathValue("id"), req.Name)
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, wire.AuthPasskey(pk))
}

func (s *Server) authDeletePasskey(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if _, ok := s.requireStepUp(w, r); !ok {
		return
	}
	if err := s.Auth.DeletePasskey(r.PathValue("id"), false); err != nil {
		authFailure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authPairCreate(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if _, ok := s.requireStepUp(w, r); !ok {
		return
	}
	var req contract.PairCreateRequest
	if !decodeOptional(w, r, &req) {
		return
	}
	grant, err := s.Auth.CreatePairingCode(req.TargetURL)
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusCreated, pairResponse(grant))
}

func pairResponse(grant auth.PairingGrant) contract.PairCreateResponse {
	pairURL := ""
	if grant.TargetURL != "" {
		pairURL = grant.TargetURL + "/pair#code=" + url.QueryEscape(grant.Code)
	}
	return contract.PairCreateResponse{
		Code:        grant.Code,
		DisplayCode: grant.DisplayCode,
		PairURL:     pairURL,
		ExpiresAt:   grant.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}
}

func (s *Server) authListSessions(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.requireAuthService(w) {
		return
	}
	currentID := ""
	if p, ok := principalFrom(r); ok {
		currentID = p.SessionID
	}
	items, err := s.Auth.ListSessions(currentID)
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, wire.AuthSessions(items))
}

func (s *Server) authRevokeSession(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.requireAuthService(w) {
		return
	}
	if err := s.Auth.RevokeSession(r.PathValue("id")); err != nil {
		authFailure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authRevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.requireAuthService(w) {
		return
	}
	currentID := ""
	if p, ok := principalFrom(r); ok {
		currentID = p.SessionID
	}
	removed, err := s.Auth.RevokeOtherSessions(currentID)
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, contract.RevokeOthersResponse{Revoked: removed})
}

func (s *Server) authListAPITokens(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.requireAuthService(w) {
		return
	}
	items, err := s.Auth.ListAPITokens()
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, wire.AuthAPITokens(items))
}

func (s *Server) authCreateAPIToken(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if _, ok := s.requireStepUp(w, r); !ok {
		return
	}
	var req contract.CreateAPITokenRequest
	if !decode(w, r, &req) {
		return
	}
	created, err := s.Auth.CreateAPIToken(req.Name, req.ExpiresDays)
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusCreated, contract.CreateAPITokenResponse{
		Token: created.Token,
		Info:  wire.AuthAPIToken(created.Info),
	})
}

func (s *Server) authRevokeAPIToken(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.requireAuthService(w) {
		return
	}
	if err := s.Auth.RevokeAPIToken(r.PathValue("id")); err != nil {
		authFailure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authRegenerateRecovery(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if _, ok := s.requireStepUp(w, r); !ok {
		return
	}
	codes, err := s.Auth.RegenerateRecoveryCodes()
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, contract.RecoveryCodesResponse{RecoveryCodes: codes})
}
