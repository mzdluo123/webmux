// Package relay implements the host side of the authenticated Relay transport.
package relay

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

// Config contains the control authority and startup-fixed WebMux listener.
// TLSConfig may supply additional RootCAs, never a verification bypass.
type Config struct {
	InteractiveConnections int
	BulkConnections        int
	ControlEndpoint        string
	ControlAudience        string
	LoopbackAddress        string
	TLSConfig              *tls.Config
	LoopbackTLSConfig      *tls.Config
}

// Client owns at most one running connector. It never persists credentials.
type Client struct {
	config      Config
	http        *http.Client
	dialContext func(context.Context, string, string) (net.Conn, error)
	mu          sync.Mutex
	assignment  *protocol.Assignment
	active      *connection
	connecting  bool
}

func New(cfg Config) (*Client, error) {
	if cfg.InteractiveConnections == 0 {
		cfg.InteractiveConnections = protocol.DefaultInteractiveConnections
	}
	if cfg.BulkConnections == 0 {
		cfg.BulkConnections = protocol.DefaultBulkConnections
	}
	if cfg.InteractiveConnections < 1 || cfg.InteractiveConnections > protocol.MaxRoleConnections || cfg.BulkConnections < 1 || cfg.BulkConnections > protocol.MaxRoleConnections {
		return nil, protocol.Error{Code: protocol.InvalidConfig}
	}
	if !validOrigin(cfg.ControlEndpoint) || !validAudience(cfg.ControlAudience) {
		return nil, protocol.Error{Code: protocol.InvalidConfig}
	}
	host, port, err := net.SplitHostPort(cfg.LoopbackAddress)
	ip := net.ParseIP(host)
	p, portErr := strconv.ParseUint(port, 10, 16)
	if err != nil || ip == nil || !ip.IsLoopback() || portErr != nil || p == 0 {
		return nil, protocol.Error{Code: protocol.InvalidConfig}
	}
	if cfg.TLSConfig != nil {
		t := cfg.TLSConfig
		if t.InsecureSkipVerify || t.VerifyConnection != nil || t.VerifyPeerCertificate != nil || t.ServerName != "" || len(t.Certificates) != 0 || t.GetClientCertificate != nil || t.MaxVersion != 0 && t.MaxVersion < tls.VersionTLS13 {
			return nil, protocol.Error{Code: protocol.InvalidConfig}
		}
		// Own the test trust configuration; later caller mutation cannot change it.
		cfg.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS13}
		if t.RootCAs != nil {
			cfg.TLSConfig.RootCAs = t.RootCAs.Clone()
		}
	}
	local := cfg.LoopbackTLSConfig
	if local == nil || local.InsecureSkipVerify || local.VerifyConnection != nil || local.VerifyPeerCertificate != nil || local.RootCAs == nil || local.ServerName == "" || len(local.Certificates) != 1 || local.GetClientCertificate != nil || local.MaxVersion != 0 && local.MaxVersion < tls.VersionTLS13 {
		return nil, protocol.Error{Code: protocol.InvalidConfig}
	}
	if len(local.Certificates[0].Certificate) == 0 || local.Certificates[0].PrivateKey == nil {
		return nil, protocol.Error{Code: protocol.InvalidConfig}
	}
	cfg.LoopbackTLSConfig = local.Clone()
	cfg.LoopbackTLSConfig.RootCAs = local.RootCAs.Clone()
	cfg.LoopbackTLSConfig.MinVersion = tls.VersionTLS13
	c := &Client{config: cfg, dialContext: (&net.Dialer{Timeout: protocol.AuthTimeout, KeepAlive: protocol.KeepAliveInterval}).DialContext}
	c.http = c.httpClient(nil)
	return c, nil
}

func validAudience(s string) bool {
	if len(s) == 0 || len(s) > 255 {
		return false
	}
	for _, r := range s {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	return true
}

func validOrigin(s string) bool {
	if len(s) == 0 || len(s) > protocol.MaxURLBytes || strings.ContainsAny(s, "\r\n\t\\") {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Port() != "" && u.Port() != "443" {
		return false
	}
	h := u.Hostname()
	if h == "" || len(h) > 253 || net.ParseIP(h) != nil || strings.ToLower(h) != h || strings.HasSuffix(h, ".") {
		return false
	}
	for _, label := range strings.Split(h, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

// A nil trust selects normal CA verification for control requests only.
// WSS callers must pass their assigned trust, even when it is invalid or empty.
func (c *Client) httpClient(trust *protocol.ServiceTrust) *http.Client {
	t := &tls.Config{MinVersion: tls.VersionTLS13}
	if c.config.TLSConfig != nil {
		t.RootCAs = c.config.TLSConfig.RootCAs
	}
	if trust != nil {
		t.VerifyConnection = func(state tls.ConnectionState) error {
			if state.Version < tls.VersionTLS13 || len(state.VerifiedChains) == 0 || len(state.PeerCertificates) == 0 {
				return protocol.Error{Code: protocol.Unauthenticated}
			}
			return protocol.VerifyCertificatePin(state.PeerCertificates[0], *trust)
		}
	}
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:        t,
			DialContext:            c.dialContext,
			TLSHandshakeTimeout:    protocol.AuthTimeout,
			ResponseHeaderTimeout:  protocol.RequestTimeout,
			MaxResponseHeaderBytes: protocol.MaxControlBytes,
			IdleConnTimeout:        protocol.ControlLease,
			MaxIdleConns:           2, MaxIdleConnsPerHost: 2,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// request performs one operation only: lost enrollment/delete responses are not retried.
func (c *Client) request(ctx context.Context, method, path string, input, output any, statuses ...int) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, protocol.RequestTimeout)
	defer cancel()
	body, err := json.Marshal(input)
	if err != nil || len(body) > protocol.MaxAuthBytes {
		return 0, protocol.Error{Code: protocol.Invalid}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.config.ControlEndpoint+path, bytes.NewReader(body))
	if err != nil {
		return 0, protocol.Error{Code: protocol.InvalidConfig}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-store")
	res, err := c.http.Do(req)
	if err != nil {
		return 0, transportError(ctx, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.ContentLength > protocol.MaxControlBytes {
		return res.StatusCode, protocol.Error{Code: protocol.Invalid}
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, protocol.MaxControlBytes+1))
	if err != nil {
		return res.StatusCode, transportError(ctx, err)
	}
	if len(data) > protocol.MaxControlBytes {
		return res.StatusCode, protocol.Error{Code: protocol.Invalid}
	}
	if !slices.Contains(statuses, res.StatusCode) {
		var remote protocol.Error
		if protocol.Decode(data, &remote, protocol.MaxAuthBytes) == nil && remote.Code.Valid() && protocol.HTTPStatus(remote.Code) == res.StatusCode {
			// Correlation identifiers are deliberately not surfaced to diagnostics.
			return res.StatusCode, protocol.Error{Code: remote.Code, RetryAfterSeconds: remote.RetryAfterSeconds}
		}
		if res.StatusCode >= 300 && res.StatusCode < 400 {
			return res.StatusCode, protocol.Error{Code: protocol.Forbidden}
		}
		return res.StatusCode, protocol.Error{Code: protocol.Unavailable}
	}
	if output == nil {
		if len(data) != 0 {
			return res.StatusCode, protocol.Error{Code: protocol.Invalid}
		}
		return res.StatusCode, nil
	}
	if err := protocol.Decode(data, output, protocol.MaxControlBytes); err != nil {
		return res.StatusCode, err
	}
	return res.StatusCode, nil
}

func transportError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var cert *tls.CertificateVerificationError
	var unknown x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var p protocol.Error
	if errors.As(err, &cert) || errors.As(err, &unknown) || errors.As(err, &hostname) || errors.As(err, &invalid) || errors.As(err, &p) && p.Code == protocol.Unauthenticated {
		return protocol.Error{Code: protocol.Unauthenticated}
	}
	// net/url errors can contain URLs and credentials; never return them.
	return protocol.Error{Code: protocol.Unavailable}
}

func (c *Client) Enroll(ctx context.Context, key ed25519.PrivateKey, machineID protocol.MachineID, rebind bool) (protocol.Enrollment, error) {
	var out protocol.Enrollment
	if len(key) != ed25519.PrivateKeySize || machineID.Validate() != nil {
		return out, protocol.Error{Code: protocol.Invalid}
	}
	in := protocol.EnrollmentRequest{Version: protocol.V1, PublicKey: protocol.PublicKey{Algorithm: protocol.HostAlgorithm, Key: base64.RawURLEncoding.EncodeToString(key.Public().(ed25519.PublicKey))}, MachineID: machineID, Rebind: rebind}
	if _, err := c.request(ctx, http.MethodPost, protocol.PathEnrollments, in, &out, http.StatusCreated); err != nil {
		return out, err
	}
	u, err := url.Parse(out.VerificationURL)
	now := time.Now()
	if out.Version != protocol.V1 || out.Handle.Validate() != nil || out.UserCode == "" || len(out.UserCode) > 256 || strings.ContainsAny(out.UserCode, "\r\n\x00") || err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || len(out.VerificationURL) > protocol.MaxURLBytes || out.ExpiresAt <= now.Unix() || out.ExpiresAt > now.Add(15*time.Minute).Unix() || out.PollIntervalSeconds < 1 || out.PollIntervalSeconds > 60 {
		return protocol.Enrollment{}, protocol.Error{Code: protocol.Invalid}
	}
	return out, nil
}

func (c *Client) Poll(ctx context.Context, handle protocol.Token) (protocol.EnrollmentResult, error) {
	var out protocol.EnrollmentResult
	if err := handle.Validate(); err != nil {
		return out, err
	}
	if _, err := c.request(ctx, http.MethodPost, protocol.PathEnrollmentPoll, protocol.EnrollmentPoll{Version: protocol.V1, Handle: handle}, &out, http.StatusOK); err != nil {
		return out, err
	}
	if out.Version != protocol.V1 {
		return protocol.EnrollmentResult{}, protocol.Error{Code: protocol.Incompatible}
	}
	if err := out.Binding.Validate(); err != nil {
		return protocol.EnrollmentResult{}, err
	}
	return out, nil
}

func (c *Client) Cancel(ctx context.Context, handle protocol.Token) error {
	if err := handle.Validate(); err != nil {
		return err
	}
	_, err := c.request(ctx, http.MethodDelete, protocol.PathEnrollments, protocol.EnrollmentPoll{Version: protocol.V1, Handle: handle}, nil, http.StatusNoContent)
	return err
}

func (c *Client) hostProof(ctx context.Context, key ed25519.PrivateKey, binding protocol.Binding, purpose protocol.HostPurpose) (protocol.HostProof, protocol.Token, error) {
	if len(key) != ed25519.PrivateKeySize {
		return protocol.HostProof{}, "", protocol.Error{Code: protocol.Invalid}
	}
	if err := binding.Validate(); err != nil {
		return protocol.HostProof{}, "", err
	}
	var challenge protocol.HostChallenge
	_, err := c.request(ctx, http.MethodPost, protocol.PathHostChallenge, protocol.HostChallengeRequest{Version: protocol.V1, HostID: binding.HostID, Purpose: purpose}, &challenge, http.StatusOK)
	if err != nil {
		return protocol.HostProof{}, "", err
	}
	if err := challenge.Validate(time.Now()); err != nil {
		return protocol.HostProof{}, "", err
	}
	if challenge.Audience != c.config.ControlAudience || challenge.HostID != binding.HostID || challenge.BindingGeneration != binding.BindingGeneration || challenge.Purpose != purpose {
		return protocol.HostProof{}, "", protocol.Error{Code: protocol.WrongAudience}
	}
	proof, err := protocol.SignHost(key, challenge, time.Now())
	return proof, challenge.ControlEpoch, err
}

func (c *Client) Assignment(ctx context.Context, key ed25519.PrivateKey, binding protocol.Binding) (protocol.Assignment, error) {
	var out protocol.Assignment
	proof, epoch, err := c.hostProof(ctx, key, binding, protocol.PurposeAssignment)
	if err != nil {
		return out, err
	}
	if _, err := c.request(ctx, http.MethodPost, protocol.PathAssignments, proof, &out, http.StatusOK); err != nil {
		return protocol.Assignment{}, err
	}
	if err := out.Validate(time.Now()); err != nil {
		return protocol.Assignment{}, err
	}
	if out.Binding != binding || out.ControlEpoch != epoch || out.ControlAudience != c.config.ControlAudience {
		return protocol.Assignment{}, protocol.Error{Code: protocol.WrongAudience}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.assignment != nil && c.assignment.Binding.HostID == binding.HostID && out.Revision < c.assignment.Revision {
		return protocol.Assignment{}, protocol.Error{Code: protocol.StaleGeneration}
	}
	owned := cloneAssignment(out)
	c.assignment = &owned
	return out, nil
}

func (c *Client) Delete(ctx context.Context, key ed25519.PrivateKey, binding protocol.Binding) (protocol.LogoutResult, error) {
	var out protocol.LogoutResult
	proof, _, err := c.hostProof(ctx, key, binding, protocol.PurposeDelete)
	if err != nil {
		return out, err
	}
	status, err := c.request(ctx, http.MethodDelete, protocol.PathBinding, proof, &out, http.StatusOK, http.StatusAccepted)
	if err != nil {
		return protocol.LogoutResult{}, err
	}
	if out.Version != protocol.V1 || out.RevocationID.Validate() != nil || out.Complete != (status == http.StatusOK) {
		return protocol.LogoutResult{}, protocol.Error{Code: protocol.Invalid}
	}
	return out, nil
}

func cloneAssignment(a protocol.Assignment) protocol.Assignment {
	a.Capabilities = slices.Clone(a.Capabilities)
	a.Trust.Pins = slices.Clone(a.Trust.Pins)
	return a
}

func sameAssignment(a, b protocol.Assignment) bool {
	return a.Version == b.Version && a.Revision == b.Revision && a.ControlEpoch == b.ControlEpoch && a.ControlAudience == b.ControlAudience && a.Source == b.Source && a.ExpiresAt == b.ExpiresAt && a.Binding == b.Binding && a.NodeID == b.NodeID && slices.Equal(a.Capabilities, b.Capabilities) && a.ConfigRevision == b.ConfigRevision && a.ConnectorURL == b.ConnectorURL && a.DataOrigin == b.DataOrigin && a.NodeAudience == b.NodeAudience && a.Trust.Algorithm == b.Trust.Algorithm && slices.Equal(a.Trust.Pins, b.Trust.Pins)
}
