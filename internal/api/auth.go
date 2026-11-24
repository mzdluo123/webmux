package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/auth"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/wire"
)

const stepUpHeader = "X-WebMux-Step-Up"

type principalKey struct{}

func withPrincipal(ctx context.Context, p auth.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func principalFrom(r *http.Request) (auth.Principal, bool) {
	p, ok := r.Context().Value(principalKey{}).(auth.Principal)
	return p, ok
}

func isPublicAuthPath(path string) bool {
	switch path {
	case "/api/auth/status",
		"/api/auth/onboarding/verify",
		"/api/auth/onboarding/passkey/begin",
		"/api/auth/onboarding/passkey/finish",
		"/api/auth/login/begin",
		"/api/auth/login/finish",
		"/api/auth/pair/begin",
		"/api/auth/pair/finish",
		"/api/auth/recovery":
		return true
	default:
		return false
	}
}

func (s *Server) authRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/status", s.authStatus)
	mux.HandleFunc("POST /api/auth/onboarding/verify", s.authOnboardingVerify)
	mux.HandleFunc("POST /api/auth/onboarding/passkey/begin", s.authOnboardingBegin)
	mux.HandleFunc("POST /api/auth/onboarding/passkey/finish", s.authOnboardingFinish)
	mux.HandleFunc("POST /api/auth/onboarding/complete", s.authOnboardingComplete)
	mux.HandleFunc("POST /api/auth/login/begin", s.authLoginBegin)
	mux.HandleFunc("POST /api/auth/login/finish", s.authLoginFinish)
	mux.HandleFunc("POST /api/auth/pair/begin", s.authPairBegin)
	mux.HandleFunc("POST /api/auth/pair/finish", s.authPairFinish)
	mux.HandleFunc("POST /api/auth/recovery", s.authRecoveryLogin)
	s.authManageRoutes(mux)
}

func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	hostname, _ := os.Hostname()
	if s.Auth == nil {
		state := contract.AuthDisabled
		if s.Config.Token != "" {
			state = contract.AuthLegacyToken
		}
		reply(w, http.StatusOK, contract.AuthStatusResponse{
			State:               state,
			OnboardingCompleted: true,
			Hostname:            hostname,
		})
		return
	}
	snap := s.Auth.Status(s.webAuthnHost(r))
	reply(w, http.StatusOK, wire.AuthStatus(snap, hostname))
}

func (s *Server) authOnboardingVerify(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.allowPublicAuth(w, r) {
		return
	}
	var req contract.OnboardingVerifyRequest
	if !decode(w, r, &req) {
		return
	}
	ticket, err := s.Auth.VerifySetupCode(req.Code)
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, contract.OnboardingVerifyResponse{
		Ticket:    ticket.Ticket,
		ExpiresAt: ticket.ExpiresAt.UTC().Format(time.RFC3339Nano),
	})
}

func (s *Server) authOnboardingBegin(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.allowPublicAuth(w, r) {
		return
	}
	var req contract.OnboardingBeginRequest
	if !decode(w, r, &req) {
		return
	}
	opts, err := s.Auth.BeginOnboardingPasskey(req.Ticket, s.webAuthnHost(r))
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, wire.WebAuthnCreation(opts))
}

func (s *Server) authOnboardingFinish(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.allowPublicAuth(w, r) {
		return
	}
	var req contract.OnboardingFinishRequest
	if !decode(w, r, &req) {
		return
	}
	res, err := s.Auth.FinishOnboardingPasskey(req.Ticket, auth.PasskeyFinishInput{
		ChallengeID:       req.ChallengeID,
		Name:              req.Name,
		Origin:            s.webAuthnOrigin(r),
		ClientDataJSON:    req.ClientDataJSON,
		AttestationObject: req.AttestationObject,
		Transports:        req.Transports,
		DeviceName:        deviceNameFrom(req.DeviceName, r),
	})
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusCreated, wire.AuthOnboardingFinish(res))
}

func (s *Server) authOnboardingComplete(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if s.Auth == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := s.Auth.CompleteOnboarding(); err != nil {
		authFailure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authLoginBegin(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.allowPublicAuth(w, r) {
		return
	}
	opts, err := s.Auth.BeginLogin(s.webAuthnHost(r))
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, wire.WebAuthnAssertion(opts))
}

func (s *Server) authLoginFinish(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.allowPublicAuth(w, r) {
		return
	}
	var req contract.AssertionFinishRequest
	if !decode(w, r, &req) {
		return
	}
	res, err := s.Auth.FinishLogin(auth.AssertionFinishInput{
		ChallengeID:       req.ChallengeID,
		CredentialID:      req.CredentialID,
		Origin:            s.webAuthnOrigin(r),
		ClientDataJSON:    req.ClientDataJSON,
		AuthenticatorData: req.AuthenticatorData,
		Signature:         req.Signature,
		DeviceName:        deviceNameFrom(req.DeviceName, r),
	})
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, wire.AuthLogin(res, nil))
}

func (s *Server) authPairBegin(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.allowPublicAuth(w, r) {
		return
	}
	var req contract.PairBeginRequest
	if !decode(w, r, &req) {
		return
	}
	opts, err := s.Auth.BeginPair(req.Code, s.webAuthnHost(r))
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, wire.WebAuthnCreation(opts))
}

func (s *Server) authPairFinish(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.allowPublicAuth(w, r) {
		return
	}
	var req contract.PasskeyFinishRequest
	if !decode(w, r, &req) {
		return
	}
	res, pk, err := s.Auth.FinishPair(auth.PasskeyFinishInput{
		ChallengeID:       req.ChallengeID,
		Name:              req.Name,
		Origin:            s.webAuthnOrigin(r),
		ClientDataJSON:    req.ClientDataJSON,
		AttestationObject: req.AttestationObject,
		Transports:        req.Transports,
		DeviceName:        deviceNameFrom(req.DeviceName, r),
	})
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusCreated, wire.AuthLogin(res, &pk))
}

func (s *Server) authRecoveryLogin(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !s.allowPublicAuth(w, r) {
		return
	}
	var req contract.RecoveryLoginRequest
	if !decode(w, r, &req) {
		return
	}
	res, err := s.Auth.LoginWithRecoveryCode(req.Code, deviceNameFrom(req.DeviceName, r), s.webAuthnHost(r))
	if err != nil {
		authFailure(w, err)
		return
	}
	reply(w, http.StatusOK, wire.AuthLogin(res, nil))
}

func (s *Server) allowPublicAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.Auth == nil {
		failure(w, http.StatusNotFound, errors.New("passkey authentication is not enabled"))
		return false
	}
	if !s.local && !s.Auth.AllowPublic(remoteIPKey(r.RemoteAddr)) {
		failure(w, http.StatusTooManyRequests, auth.ErrRateLimited)
		return false
	}
	return true
}

func (s *Server) webAuthnOrigin(r *http.Request) string {
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		return origin
	}
	scheme := "http"
	if r.TLS != nil || (s.Public != nil && s.Public.HostAllowed(r.Host)) {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func (s *Server) webAuthnHost(r *http.Request) string {
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		if u, err := url.Parse(origin); err == nil && u.Host != "" {
			return u.Host
		}
	}
	return r.Host
}

func remoteIPKey(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil && host != "" {
		return host
	}
	return remoteAddr
}

func deviceNameFrom(explicit string, r *http.Request) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	ua := r.Header.Get("User-Agent")
	if ua == "" {
		return "Browser"
	}
	return shortUserAgent(ua)
}

func shortUserAgent(ua string) string {
	browser := "Browser"
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "Chrome/") && !strings.Contains(ua, "Chromium"):
		browser = "Chrome"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "Safari/") && !strings.Contains(ua, "Chrome/"):
		browser = "Safari"
	}
	platform := ""
	switch {
	case strings.Contains(ua, "iPhone"):
		platform = "iPhone"
	case strings.Contains(ua, "iPad"):
		platform = "iPad"
	case strings.Contains(ua, "Android"):
		platform = "Android"
	case strings.Contains(ua, "Macintosh") || strings.Contains(ua, "Mac OS"):
		platform = "macOS"
	case strings.Contains(ua, "Windows"):
		platform = "Windows"
	case strings.Contains(ua, "Linux"):
		platform = "Linux"
	}
	if platform != "" {
		return browser + " on " + platform
	}
	return browser
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

func authFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrStepUpRequired):
		reply(w, http.StatusForbidden, contract.AuthReasonError{Error: err.Error(), Reason: "step_up_required"})
	case errors.Is(err, auth.ErrStorageUnavailable), errors.Is(err, auth.ErrLocked):
		failure(w, http.StatusServiceUnavailable, err)
	case errors.Is(err, auth.ErrRateLimited):
		failure(w, http.StatusTooManyRequests, err)
	case errors.Is(err, auth.ErrInvalidSetupCode),
		errors.Is(err, auth.ErrInvalidTicket),
		errors.Is(err, auth.ErrInvalidAssertion),
		errors.Is(err, auth.ErrSignCountAnomaly),
		errors.Is(err, auth.ErrInvalidPairingCode),
		errors.Is(err, auth.ErrInvalidRecovery):
		failure(w, http.StatusUnauthorized, err)
	case errors.Is(err, auth.ErrAlreadyInitialized),
		errors.Is(err, auth.ErrNotInitialized),
		errors.Is(err, auth.ErrNoPasskeyForRPID),
		errors.Is(err, auth.ErrLimitReached),
		errors.Is(err, auth.ErrDisabled):
		failure(w, http.StatusConflict, err)
	case errors.Is(err, auth.ErrNotFound):
		failure(w, http.StatusNotFound, err)
	case errors.Is(err, auth.ErrInvalidChallenge),
		errors.Is(err, auth.ErrInvalidRPID),
		errors.Is(err, auth.ErrInvalidInput):
		failure(w, http.StatusBadRequest, err)
	default:
		failure(w, http.StatusInternalServerError, err)
	}
}
