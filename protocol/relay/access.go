package relay

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	PathEnrollments      = "/v1/enrollments"
	PathEnrollmentPoll   = "/v1/enrollments/poll"
	PathHostChallenge    = "/v1/hosts/challenge"
	PathAssignments      = "/v1/assignments"
	PathBinding          = "/v1/binding"
	PathConnector        = "/v1/connector"
	PathGitHubStart      = "/v1/auth/github/start"
	PathGitHubCallback   = "/v1/auth/github/callback"
	PathConsole          = "/v1/console"
	PathConsoleHosts     = "/v1/console/hosts"
	PathConsoleLogout    = "/v1/console/logout"
	PathLoginAuthorize   = "/v1/login-grants/authorize"
	PathNodeControl      = "/v1/nodes/control"
	PathNodeSecrets      = "/v1/nodes/secrets"
	PathAdminNodes       = "/v1/admin/nodes"
	PathAdminInvitations = "/v1/admin/invitations"
	HostLoginStart       = "/auth/relay/start"
	HostLoginCallback    = "/auth/relay/callback"
	HostICE              = "/api/relay/ice"
)

type EnrollmentRequest struct {
	Version   Version   `json:"version"`
	PublicKey PublicKey `json:"publicKey"`
	Rebind    bool      `json:"rebind"`
}

func (r EnrollmentRequest) Validate() error {
	if r.Version != V1 {
		return Error{Code: Incompatible}
	}
	_, err := r.PublicKey.Parse()
	return err
}

type Enrollment struct {
	Version             Version `json:"version"`
	Handle              Token   `json:"handle"`
	UserCode            string  `json:"userCode"`
	VerificationURL     string  `json:"verificationUrl"`
	ExpiresAt           int64   `json:"expiresAt"`
	PollIntervalSeconds uint32  `json:"pollIntervalSeconds"`
}

type EnrollmentPoll struct {
	Version Version `json:"version"`
	Handle  Token   `json:"handle"`
}

type Binding struct {
	HostID            HostID  `json:"hostId"`
	OwnerID           string  `json:"ownerId"`
	BindingGeneration Counter `json:"bindingGeneration"`
	PublicURL         string  `json:"publicUrl"`
}

func (b Binding) Validate() error {
	if err := b.HostID.Validate(); err != nil {
		return err
	}
	if !decimalID(b.OwnerID) || b.BindingGeneration == 0 {
		return Error{Code: Invalid}
	}
	u, err := secureURL(b.PublicURL, "https", true)
	if err != nil || !strings.HasPrefix(u.Hostname(), b.HostID.Label()+".") {
		return Error{Code: Invalid}
	}
	return nil
}

// EnrollmentResult is returned once, on successful atomic handle consumption.
// Pending polls return Error{Code:Pending}, not a partial binding.
type EnrollmentResult struct {
	Version Version `json:"version"`
	Binding Binding `json:"binding"`
}

type Assignment struct {
	Version         Version      `json:"version"`
	Revision        Counter      `json:"revision"`
	ControlEpoch    Token        `json:"controlEpoch"`
	ControlAudience string       `json:"controlAudience"`
	Source          string       `json:"source"`
	ExpiresAt       int64        `json:"expiresAt"`
	Binding         Binding      `json:"binding"`
	NodeID          NodeID       `json:"nodeId"`
	Capabilities    Capabilities `json:"capabilities"`
	ConfigRevision  Counter      `json:"configRevision"`
	ConnectorURL    string       `json:"connectorUrl"`
	DataOrigin      string       `json:"dataOrigin"`
	NodeAudience    string       `json:"nodeAudience"`
	Trust           ServiceTrust `json:"trust"`
}

// Validate checks structure. Consumers must additionally compare the binding,
// configured control audience, highest revision and control epoch with their
// authenticated control exchange; a decoded assignment is not a trust source.
func (a Assignment) Validate(now time.Time) error {
	if a.Version != V1 {
		return Error{Code: Incompatible}
	}
	if a.Revision == 0 || a.ConfigRevision == 0 || a.Source != "authenticated-control" || !audience(a.ControlAudience) || !audience(a.NodeAudience) {
		return Error{Code: Invalid}
	}
	if err := a.ControlEpoch.Validate(); err != nil {
		return err
	}
	if !validUntil(now, a.ExpiresAt, ControlLease) {
		return Error{Code: Expired}
	}
	if err := a.Binding.Validate(); err != nil {
		return err
	}
	if err := a.NodeID.Validate(); err != nil {
		return err
	}
	if err := a.Capabilities.Validate(); err != nil {
		return err
	}
	if !a.Capabilities.Has(CapabilityWSS) {
		return Error{Code: Invalid}
	}
	c, err := secureURL(a.ConnectorURL, "wss", false)
	if err != nil || c.Path != PathConnector {
		return Error{Code: Invalid}
	}
	d, err := secureURL(a.DataOrigin, "https", true)
	if err != nil || c.Host != d.Host {
		return Error{Code: Invalid}
	}
	return a.Trust.Validate()
}

type LoginAuthorize struct {
	Version             Version `json:"version"`
	HostID              HostID  `json:"hostId"`
	State               Token   `json:"state"`
	RedirectURI         string  `json:"redirectUri"`
	CodeChallenge       string  `json:"codeChallenge"`
	CodeChallengeMethod string  `json:"codeChallengeMethod"`
}

func (r LoginAuthorize) Validate() error {
	if r.Version != V1 {
		return Error{Code: Incompatible}
	}
	if err := r.HostID.Validate(); err != nil {
		return err
	}
	if err := r.State.Validate(); err != nil {
		return err
	}
	if _, err := decodeFixed(r.CodeChallenge, 32); err != nil {
		return err
	}
	u, err := secureURL(r.RedirectURI, "https", false)
	if err != nil || u.Path != HostLoginCallback || r.CodeChallengeMethod != "S256" {
		return Error{Code: Invalid}
	}
	return nil
}

type LoginExchange struct {
	Code        Token  `json:"code"`
	Verifier    string `json:"verifier"`
	RedirectURI string `json:"redirectUri"`
}

func (r LoginExchange) Validate() error {
	if err := r.Code.Validate(); err != nil {
		return err
	}
	if len(r.Verifier) < 43 || len(r.Verifier) > 128 {
		return Error{Code: Invalid}
	}
	for _, c := range r.Verifier {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.ContainsRune("-._~", c)) {
			return Error{Code: Invalid}
		}
	}
	u, err := secureURL(r.RedirectURI, "https", false)
	if err != nil || u.Path != HostLoginCallback {
		return Error{Code: Invalid}
	}
	return nil
}

// BrowserGrant is a control-only reference, NEVER a browser Cookie or bearer
// session. Only a successful LoginExchange can mint it. All ICE refreshes must
// revalidate its server-side console session, invitation and current connector.
type BrowserGrant struct {
	ID                  Token   `json:"id"`
	ConsoleSessionID    Token   `json:"consoleSessionId"`
	OwnerID             string  `json:"ownerId"`
	HostID              HostID  `json:"hostId"`
	BindingGeneration   Counter `json:"bindingGeneration"`
	ConnectorGeneration Counter `json:"connectorGeneration"`
	ControlEpoch        Token   `json:"controlEpoch"`
	NodeEpoch           Token   `json:"nodeEpoch"`
	Origin              string  `json:"origin"`
	ExpiresAt           int64   `json:"expiresAt"`
}

func (g BrowserGrant) Validate(now time.Time) error {
	for _, t := range []Token{g.ID, g.ConsoleSessionID, g.ControlEpoch, g.NodeEpoch} {
		if err := t.Validate(); err != nil {
			return err
		}
	}
	if err := g.HostID.Validate(); err != nil {
		return err
	}
	if !decimalID(g.OwnerID) || g.BindingGeneration == 0 || g.ConnectorGeneration == 0 {
		return Error{Code: Invalid}
	}
	if !validUntil(now, g.ExpiresAt, BrowserSessionTTL) {
		return Error{Code: Expired}
	}
	u, err := secureURL(g.Origin, "https", true)
	if err != nil || !strings.HasPrefix(u.Hostname(), g.HostID.Label()+".") {
		return Error{Code: Invalid}
	}
	return nil
}

type ICERequest struct {
	GrantID          Token `json:"grantId"`
	PeerConnectionID Token `json:"peerConnectionId"`
}

type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username"`
	Credential string   `json:"credential"`
}

type ICEEndpoint struct {
	Role      string      `json:"role"`
	ExpiresAt int64       `json:"expiresAt"`
	Servers   []ICEServer `json:"servers"`
}

// ICEConfig goes ONLY to the host control channel. The host returns Browser
// alone to the requesting authenticated browser and retains Host in memory.
type ICEConfig struct {
	NodeID           NodeID      `json:"nodeId"`
	ConfigRevision   Counter     `json:"configRevision"`
	Secret           SecretRef   `json:"secret"`
	GrantID          Token       `json:"grantId"`
	PeerConnectionID Token       `json:"peerConnectionId"`
	ExpiresAt        int64       `json:"expiresAt"`
	Browser          ICEEndpoint `json:"browser"`
	Host             ICEEndpoint `json:"host"`
}

func (c ICEConfig) Validate(now time.Time) error {
	if err := c.NodeID.Validate(); err != nil {
		return err
	}
	if c.ConfigRevision == 0 || c.Secret.Purpose != SecretTURN {
		return Error{Code: Invalid}
	}
	if err := c.Secret.Validate(); err != nil {
		return err
	}
	if err := c.GrantID.Validate(); err != nil {
		return err
	}
	if err := c.PeerConnectionID.Validate(); err != nil {
		return err
	}
	if !validUntil(now, c.ExpiresAt, MaxICETTL) {
		return Error{Code: Expired}
	}
	if c.Browser.Role != "browser" || c.Host.Role != "host" {
		return Error{Code: Invalid}
	}
	for _, e := range []ICEEndpoint{c.Browser, c.Host} {
		if len(e.Servers) != 1 {
			return Error{Code: Invalid}
		}
		if !validUntil(now, e.ExpiresAt, MaxICETTL) {
			return Error{Code: Expired}
		}
		for _, s := range e.Servers {
			if len(s.Username) < 1 || len(s.Username) > 256 || len(s.Credential) < 1 || len(s.Credential) > 256 {
				return Error{Code: Invalid}
			}
			expires, _, err := ParseTURNUsername(s.Username)
			if err != nil || expires != e.ExpiresAt {
				return Error{Code: Invalid}
			}
			if err := validateTURNURLs(s.URLs); err != nil {
				return err
			}
		}
	}
	if c.Browser.Servers[0].Username == c.Host.Servers[0].Username || c.Browser.Servers[0].Credential == c.Host.Servers[0].Credential {
		return Error{Code: Invalid}
	}
	_, browserKey, err := ParseTURNUsername(c.Browser.Servers[0].Username)
	if err != nil {
		return err
	}
	_, hostKey, err := ParseTURNUsername(c.Host.Servers[0].Username)
	if err != nil || browserKey != hostKey || c.Browser.ExpiresAt == c.Host.ExpiresAt {
		return Error{Code: Invalid}
	}
	earliest := c.Browser.ExpiresAt
	if c.Host.ExpiresAt < earliest {
		earliest = c.Host.ExpiresAt
	}
	if c.ExpiresAt != earliest {
		return Error{Code: Invalid}
	}
	return nil
}

type ConsoleHost struct {
	Binding Binding `json:"binding"`
	Online  bool    `json:"online"`
	OpenURL string  `json:"openUrl,omitempty"`
}

type ConsoleHosts struct {
	Version Version       `json:"version"`
	Hosts   []ConsoleHost `json:"hosts"`
}

type LogoutResult struct {
	Version      Version `json:"version"`
	RevocationID Token   `json:"revocationId"`
	Complete     bool    `json:"complete"`
}

func decimalID(s string) bool {
	if len(s) == 0 || len(s) > 20 || s[0] == '0' {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func secureURL(s, scheme string, origin bool) (*url.URL, error) {
	if len(s) == 0 || len(s) > MaxURLBytes {
		return nil, Error{Code: Invalid}
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != scheme || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" || strings.ContainsAny(s, "\r\n\t\\") {
		return nil, Error{Code: Invalid}
	}
	if origin && u.Path != "" {
		return nil, Error{Code: Invalid}
	}
	if !dnsName(u.Hostname()) || u.Port() != "" && u.Port() != "443" {
		return nil, Error{Code: Invalid}
	}
	return u, nil
}

func dnsName(s string) bool {
	if len(s) == 0 || len(s) > 253 || strings.ToLower(s) != s || net.ParseIP(s) != nil {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

func validateTURNURLs(urls []string) error {
	if len(urls) < 1 || len(urls) > MaxTURNURLs {
		return Error{Code: Invalid}
	}
	for i, raw := range urls {
		if len(raw) > MaxURLBytes || strings.ContainsAny(raw, "\r\n\t#@/\\") {
			return Error{Code: Invalid}
		}
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "turn" && u.Scheme != "turns" || u.Opaque == "" || u.Host != "" || u.Fragment != "" {
			return Error{Code: Invalid}
		}
		host, port, err := net.SplitHostPort(u.Opaque)
		if err != nil || !dnsName(host) || port == "" {
			return Error{Code: Invalid}
		}
		var n uint64
		for _, c := range port {
			if c < '0' || c > '9' {
				return Error{Code: Invalid}
			}
			n = n*10 + uint64(c-'0')
			if n > 65535 {
				return Error{Code: Invalid}
			}
		}
		if n == 0 {
			return Error{Code: Invalid}
		}
		if u.RawQuery != "transport=udp" && u.RawQuery != "transport=tcp" || u.Scheme == "turns" && u.RawQuery != "transport=tcp" {
			return Error{Code: Invalid}
		}
		for _, prior := range urls[:i] {
			if raw == prior {
				return Error{Code: Invalid}
			}
		}
	}
	return nil
}

// TURNUsername preserves coturn 4.19.0 get_real_username grouping. NO role,
// grant, session or random suffix may follow the stable opaque account key.
// Expiry seconds are reserved transactionally, distinct for the two roles and
// across live issuances, within the configured TTL. Exhaustion is RateLimited.
func TURNUsername(expiresAt int64, accountKey Token) (string, error) {
	if expiresAt <= 0 {
		return "", Error{Code: Invalid}
	}
	if err := accountKey.Validate(); err != nil {
		return "", err
	}
	return strconv.FormatInt(expiresAt, 10) + ":" + string(accountKey), nil
}

func ParseTURNUsername(username string) (int64, Token, error) {
	expiry, key, ok := strings.Cut(username, ":")
	if !ok || len(expiry) > 19 {
		return 0, "", Error{Code: Invalid}
	}
	n, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil || n <= 0 || strconv.FormatInt(n, 10) != expiry {
		return 0, "", Error{Code: Invalid}
	}
	accountKey := Token(key)
	if err := accountKey.Validate(); err != nil {
		return 0, "", err
	}
	return n, accountKey, nil
}

// BrowserICERequest is the same-origin host request. GrantID is deliberately
// absent: the host derives it from its authenticated owner browser session.
type BrowserICERequest struct {
	PeerConnectionID Token `json:"peerConnectionId"`
}

func (r BrowserICERequest) Validate() error { return r.PeerConnectionID.Validate() }
