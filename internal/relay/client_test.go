package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

func TestAssignmentRejectsChangedChallengeBeforeSigning(t *testing.T) {
	id := newTestIdentity(t)
	key := testKey(t)
	binding := testBinding()
	tests := []struct {
		name   string
		change func(*protocol.HostChallenge)
	}{
		{name: "audience", change: func(c *protocol.HostChallenge) { c.Audience = "attacker.example" }},
		{name: "host", change: func(c *protocol.HostChallenge) { c.HostID = "1123456789abcdef0123456789abcdef" }},
		{name: "binding", change: func(c *protocol.HostChallenge) { c.BindingGeneration++ }},
		{name: "purpose", change: func(c *protocol.HostChallenge) { c.Purpose = protocol.PurposeDelete }},
		{name: "expired", change: func(c *protocol.HostChallenge) { c.ExpiresAt = time.Now().Unix() }},
		{name: "version", change: func(c *protocol.HostChallenge) { c.Version++ }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var proofs atomic.Int32
			challenge := protocol.HostChallenge{Version: protocol.V1, Audience: "control.example", HostID: binding.HostID, BindingGeneration: binding.BindingGeneration, Purpose: protocol.PurposeAssignment, Nonce: testToken(t), ExpiresAt: time.Now().Add(20 * time.Second).Unix(), ControlEpoch: testToken(t)}
			tt.change(&challenge)
			srv := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == protocol.PathHostChallenge {
					mustJSON(t, w, http.StatusOK, challenge)
					return
				}
				proofs.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			c := newTestClient(t, id, srv.Listener.Addr().String())
			if _, err := c.Assignment(context.Background(), key, binding); err == nil {
				t.Fatal("changed challenge accepted")
			}
			if proofs.Load() != 0 {
				t.Fatal("signed proof sent for changed challenge")
			}
		})
	}
}

func TestSignedAssignmentAndDeletion(t *testing.T) {
	id := newTestIdentity(t)
	key := testKey(t)
	pub := protocol.PublicKey{Algorithm: protocol.HostAlgorithm, Key: base64.RawURLEncoding.EncodeToString(key.Public().(ed25519.PublicKey))}
	assignment := testAssignment(t, id)
	assignment.ExpiresAt = time.Now().Add(protocol.ControlLease + 3*time.Second).Unix()
	var challenge protocol.HostChallenge
	var issued atomic.Int32
	var checked atomic.Int32
	revocationID := testToken(t)
	srv := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == protocol.PathHostChallenge {
			var req protocol.HostChallengeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			challenge = protocol.HostChallenge{Version: protocol.V1, Audience: "control.example", HostID: req.HostID, BindingGeneration: 1, Purpose: req.Purpose, Nonce: testToken(t), ExpiresAt: time.Now().Add(protocol.NonceTTL + 3*time.Second).Unix(), ControlEpoch: assignment.ControlEpoch}
			issued.Add(1)
			mustJSON(t, w, http.StatusOK, challenge)
			return
		}
		var proof protocol.HostProof
		if err := json.NewDecoder(r.Body).Decode(&proof); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if err := protocol.VerifyHost(pub, challenge, proof, time.Now()); err != nil {
			t.Error(err)
			w.WriteHeader(401)
			return
		}
		checked.Add(1)
		if r.URL.Path == protocol.PathAssignments && challenge.Purpose == protocol.PurposeAssignment {
			mustJSON(t, w, http.StatusOK, assignment)
			return
		}
		if r.URL.Path == protocol.PathBinding && r.Method == http.MethodDelete && challenge.Purpose == protocol.PurposeDelete {
			mustJSON(t, w, http.StatusOK, protocol.BindingDeletionResult{Version: protocol.BindingAPIVersion, RevocationID: revocationID, Deleted: true, AccessClosed: false})
			return
		}
		w.WriteHeader(400)
	}))
	c := newTestClient(t, id, srv.Listener.Addr().String())
	got, err := c.Assignment(context.Background(), key, assignment.Binding)
	if err != nil || !sameAssignment(got, assignment) {
		t.Fatalf("assignment = %#v, %v", got, err)
	}
	// The returned DTO cannot mutate the authenticated client's trust snapshot.
	got.Trust.Pins[0] = string(testToken(t))
	if sameAssignment(got, *c.assignment) {
		t.Fatal("assignment trust aliases caller memory")
	}
	if err := c.Connect(context.Background(), key, got, testCallbacks()); err == nil {
		t.Fatal("caller-modified discovery accepted")
	}
	deleted, err := c.Delete(context.Background(), key, assignment.Binding)
	if err != nil || !deleted.Deleted || deleted.AccessClosed || deleted.RevocationID != revocationID {
		t.Fatalf("offline record deletion = %#v, %v", deleted, err)
	}
	if issued.Load() != 2 || checked.Load() != 2 {
		t.Fatal("signed operations were retried or skipped")
	}
}

func TestTLSAndRedirectNeverExposeEnrollment(t *testing.T) {
	id := newTestIdentity(t)
	var requests atomic.Int32
	srv := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, "https://attacker.example/steal", http.StatusTemporaryRedirect)
	}))
	key := testKey(t)
	tests := []struct {
		name   string
		change func(*Client)
	}{
		{name: "hostname", change: func(c *Client) { c.config.ControlEndpoint = "https://wrong.example" }},
		{name: "chain", change: func(c *Client) { c.config.TLSConfig.RootCAs = x509.NewCertPool() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, id, srv.Listener.Addr().String())
			tt.change(c)
			c.http.CloseIdleConnections()
			c.http = c.httpClient(nil)
			defer c.http.CloseIdleConnections()
			_, err := c.Enroll(context.Background(), key, protocol.MachineID(strings.Repeat("A", 43)))
			var failure protocol.Error
			if !errors.As(err, &failure) || failure.Code != protocol.Unauthenticated {
				t.Fatalf("TLS rejection = %v", err)
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatal("credentials sent before TLS identity verification")
	}
	c := newTestClient(t, id, srv.Listener.Addr().String())
	_, err := c.Enroll(context.Background(), key, protocol.MachineID(strings.Repeat("A", 43)))
	if err == nil || requests.Load() != 1 {
		t.Fatalf("redirect followed: requests=%d err=%v", requests.Load(), err)
	}
}

func TestEnrollmentPendingCancelAndBounds(t *testing.T) {
	id := newTestIdentity(t)
	handle := testToken(t)
	var polls atomic.Int32
	var cancels atomic.Int32
	srv := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			var poll protocol.EnrollmentPoll
			if err := json.NewDecoder(r.Body).Decode(&poll); err != nil || poll.Handle != handle || poll.Version != protocol.BindingAPIVersion {
				t.Error("cancel lost handle body")
			}
			cancels.Add(1)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if polls.Add(1) == 1 {
			mustJSON(t, w, http.StatusAccepted, protocol.Error{Code: protocol.Pending, RetryAfterSeconds: 5})
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, strings.Repeat("x", protocol.MaxControlBytes+1))
	}))
	c := newTestClient(t, id, srv.Listener.Addr().String())
	_, err := c.Poll(context.Background(), handle)
	var failure protocol.Error
	if !errors.As(err, &failure) || failure.Code != protocol.Pending || failure.RetryAfterSeconds != 5 {
		t.Fatalf("pending = %v", err)
	}
	if err := c.Cancel(context.Background(), handle); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Poll(context.Background(), handle); err == nil {
		t.Fatal("oversized response accepted")
	}
	if polls.Load() != 2 || cancels.Load() != 1 {
		t.Fatal("operation automatically replayed")
	}
}

func testCallbacks() Callbacks {
	return Callbacks{
		Ready: func(protocol.Scope) {}, TransportReady: func(protocol.Scope, bool, protocol.Code) {},
		DataObserved: func(protocol.Scope, DataObservation) {}, Disconnected: func() {},
		Revoke: func(context.Context, protocol.Revocation) error { return nil }, Origin: http.NotFoundHandler(),
	}
}

func TestEnrollmentCarriesMachineIdentityAcrossKeyRotation(t *testing.T) {
	id := newTestIdentity(t)
	machineID := protocol.MachineID(strings.Repeat("A", 43))
	keys := []ed25519.PrivateKey{testKey(t), testKey(t)}
	handle := testToken(t)
	var requests atomic.Int32
	srv := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(requests.Add(1)) - 1
		if n >= len(keys) {
			t.Error("unexpected enrollment retry")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != protocol.PathEnrollments || strings.Contains(r.URL.String(), string(machineID)) {
			t.Errorf("unexpected enrollment URL: %s %s", r.Method, r.URL)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var request protocol.EnrollmentRequest
		if err := json.Unmarshal(body, &request); err != nil || request.Validate() != nil {
			t.Errorf("invalid enrollment request: %v", err)
		}
		if request.MachineID != machineID || request.Version != protocol.BindingAPIVersion {
			t.Error("enrollment lost stable identity or binding API version")
		}
		wantKey := base64.RawURLEncoding.EncodeToString(keys[n].Public().(ed25519.PublicKey))
		if request.PublicKey.Key != wantKey || strings.Contains(string(body), base64.RawURLEncoding.EncodeToString(keys[n].Seed())) {
			t.Error("enrollment lost public key or exposed private seed")
		}
		mustJSON(t, w, http.StatusCreated, protocol.Enrollment{Version: protocol.BindingAPIVersion, Handle: handle, UserCode: "ABCD", VerificationURL: "https://relay.example/activate", ExpiresAt: time.Now().Add(time.Minute).Unix(), PollIntervalSeconds: 1})
	}))
	c := newTestClient(t, id, srv.Listener.Addr().String())
	for _, key := range keys {
		if _, err := c.Enroll(t.Context(), key, machineID); err != nil {
			t.Fatal(err)
		}
	}
	if requests.Load() != 2 {
		t.Fatal("enrollment was skipped or replayed")
	}
}

func TestEnrollmentExpiryAcrossClocks(t *testing.T) {
	tests := []struct {
		name string
		ttl  time.Duration
		want protocol.Code
	}{
		{name: "future", ttl: time.Hour},
		{name: "expired", ttl: -time.Minute, want: protocol.Invalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := newTestIdentity(t)
			expiry := time.Now().Add(tt.ttl).Unix()
			srv := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mustJSON(t, w, http.StatusCreated, protocol.Enrollment{Version: protocol.BindingAPIVersion, Handle: testToken(t), UserCode: "ABCD", VerificationURL: "https://github.com/login/device", ExpiresAt: expiry, PollIntervalSeconds: 1})
			}))
			c := newTestClient(t, id, srv.Listener.Addr().String())
			got, err := c.Enroll(t.Context(), testKey(t), protocol.MachineID(testToken(t)))
			if tt.want != "" {
				var failure protocol.Error
				if !errors.As(err, &failure) || failure.Code != tt.want {
					t.Fatalf("enrollment expiry error = %v, want %s", err, tt.want)
				}
				return
			}
			if err != nil || got.ExpiresAt != expiry {
				t.Fatalf("enrollment expiry = %d, %v; want %d", got.ExpiresAt, err, expiry)
			}
		})
	}
}

func TestEnrollmentRejectsInvalidMachineIdentityBeforeRequest(t *testing.T) {
	id := newTestIdentity(t)
	var requests atomic.Int32
	srv := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	c := newTestClient(t, id, srv.Listener.Addr().String())
	key := testKey(t)
	for _, machineID := range []protocol.MachineID{"", "invalid", protocol.MachineID(strings.Repeat("A", 42) + "B")} {
		_, err := c.Enroll(t.Context(), key, machineID)
		var failure protocol.Error
		if !errors.As(err, &failure) || failure.Code != protocol.Invalid {
			t.Fatalf("invalid machine identity accepted: %v", err)
		}
	}
	if requests.Load() != 0 {
		t.Fatal("invalid identity reached the network")
	}
}

func TestBindingDeletionRequiresV2RecordRemovalEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*protocol.BindingDeletionResult)
		valid  bool
	}{
		{name: "offline-record-deleted", valid: true},
		{name: "access-closed", change: func(v *protocol.BindingDeletionResult) { v.AccessClosed = true }, valid: true},
		{name: "old-version", change: func(v *protocol.BindingDeletionResult) { v.Version = protocol.V1 }},
		{name: "missing-revocation", change: func(v *protocol.BindingDeletionResult) { v.RevocationID = "" }},
		{name: "record-retained", change: func(v *protocol.BindingDeletionResult) { v.Deleted = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := newTestIdentity(t)
			key := testKey(t)
			binding := testBinding()
			result := protocol.BindingDeletionResult{Version: protocol.BindingAPIVersion, RevocationID: testToken(t), Deleted: true}
			if tc.change != nil {
				tc.change(&result)
			}
			var deletions atomic.Int32
			srv := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == protocol.PathHostChallenge {
					mustJSON(t, w, http.StatusOK, protocol.HostChallenge{Version: protocol.V1, Audience: "control.example", HostID: binding.HostID, BindingGeneration: binding.BindingGeneration, Purpose: protocol.PurposeDelete, Nonce: testToken(t), ExpiresAt: time.Now().Add(20 * time.Second).Unix(), ControlEpoch: testToken(t)})
					return
				}
				deletions.Add(1)
				mustJSON(t, w, http.StatusOK, result)
			}))
			c := newTestClient(t, id, srv.Listener.Addr().String())
			got, err := c.Delete(t.Context(), key, binding)
			if tc.valid {
				if err != nil || got != result {
					t.Fatalf("valid deletion changed: %+v, %v", got, err)
				}
			} else if err == nil {
				t.Fatalf("unproven record deletion accepted: %+v", got)
			}
			if deletions.Load() != 1 {
				t.Fatal("deletion was retried or skipped")
			}
		})
	}
}

func TestEnrollmentRejectsRetiredResponseVersions(t *testing.T) {
	id := newTestIdentity(t)
	handle := testToken(t)
	srv := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == protocol.PathEnrollments {
			mustJSON(t, w, http.StatusCreated, protocol.Enrollment{Version: protocol.V1, Handle: handle, UserCode: "ABCD", VerificationURL: "https://relay.example/activate", ExpiresAt: time.Now().Add(time.Minute).Unix(), PollIntervalSeconds: 1})
			return
		}
		var poll protocol.EnrollmentPoll
		if err := json.NewDecoder(r.Body).Decode(&poll); err != nil || poll.Version != protocol.BindingAPIVersion || poll.Handle != handle {
			t.Error("poll did not use the binding API version")
		}
		mustJSON(t, w, http.StatusOK, protocol.EnrollmentResult{Version: protocol.V1, Binding: testBinding(), OwnerLogin: "owner"})
	}))
	c := newTestClient(t, id, srv.Listener.Addr().String())
	if _, err := c.Enroll(t.Context(), testKey(t), protocol.MachineID(strings.Repeat("A", 43))); err == nil {
		t.Fatal("retired enrollment response accepted")
	}
	if _, err := c.Poll(t.Context(), handle); err == nil {
		t.Fatal("retired poll response accepted")
	}
}

func TestRelayRetryAfter(t *testing.T) {
	now := time.Date(2026, time.November, 17, 12, 0, 0, 500000000, time.UTC)
	for _, tt := range []struct {
		name string
		give string
		want uint32
	}{
		{"seconds", "7", 7},
		{"date", now.Add(8 * time.Second).Format(http.TimeFormat), 8},
		{"past", now.Add(-time.Minute).Format(http.TimeFormat), 0},
		{"negative", "-1", 0},
		{"invalid", "later", 0},
		{"bounded", "999999", 24 * 60 * 60},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := relayRetryAfter(tt.give, now); got != tt.want {
				t.Fatalf("Retry-After %q=%d want %d", tt.give, got, tt.want)
			}
		})
	}
}

func TestEnrollmentPollRetainsHTTPRetryAfterWithoutReplay(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want uint32
		code protocol.Code
	}{
		{"header", `{"code":"rate_limited","retryAfterSeconds":3}`, 7, protocol.RateLimited},
		{"body", `{"code":"rate_limited","retryAfterSeconds":9}`, 9, protocol.RateLimited},
		{"unstructured", `upstream busy`, 7, protocol.Unavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			id := newTestIdentity(t)
			var requests atomic.Int32
			srv := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Retry-After", "7")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, tt.body)
			}))
			client := newTestClient(t, id, srv.Listener.Addr().String())
			_, err := client.Poll(t.Context(), testToken(t))
			var remote protocol.Error
			if !errors.As(err, &remote) || remote.Code != tt.code || remote.RetryAfterSeconds != tt.want || requests.Load() != 1 {
				t.Fatalf("poll error=%v Retry-After=%d requests=%d", err, remote.RetryAfterSeconds, requests.Load())
			}
		})
	}
}
