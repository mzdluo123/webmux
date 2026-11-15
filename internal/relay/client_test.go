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
			}), false)
			c := newTestClient(t, id, srv.Listener.Addr().String(), "127.0.0.1:12345")
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
			challenge = protocol.HostChallenge{Version: protocol.V1, Audience: "control.example", HostID: req.HostID, BindingGeneration: 1, Purpose: req.Purpose, Nonce: testToken(t), ExpiresAt: time.Now().Add(20 * time.Second).Unix(), ControlEpoch: assignment.ControlEpoch}
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
			mustJSON(t, w, http.StatusAccepted, protocol.LogoutResult{Version: protocol.V1, RevocationID: revocationID, Complete: false})
			return
		}
		w.WriteHeader(400)
	}), false)
	c := newTestClient(t, id, srv.Listener.Addr().String(), "127.0.0.1:12345")
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
	if err != nil || deleted.Complete || deleted.RevocationID != revocationID {
		t.Fatalf("pending deletion = %#v, %v", deleted, err)
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
	}), false)
	key := testKey(t)
	tests := []struct {
		name   string
		change func(*Client)
	}{
		{name: "pin", change: func(c *Client) { c.config.InitialTrust.Pins = []string{string(testToken(t))} }},
		{name: "chain", change: func(c *Client) { c.config.TLSConfig.RootCAs = x509.NewCertPool() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, id, srv.Listener.Addr().String(), "127.0.0.1:12345")
			tt.change(c)
			c.http.CloseIdleConnections()
			c.http = c.httpClient(c.config.InitialTrust)
			defer c.http.CloseIdleConnections()
			_, err := c.Enroll(context.Background(), key, false)
			var failure protocol.Error
			if !errors.As(err, &failure) || failure.Code != protocol.Unauthenticated {
				t.Fatalf("TLS rejection = %v", err)
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatal("credentials sent before TLS identity verification")
	}
	c := newTestClient(t, id, srv.Listener.Addr().String(), "127.0.0.1:12345")
	_, err := c.Enroll(context.Background(), key, false)
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
			if err := json.NewDecoder(r.Body).Decode(&poll); err != nil || poll.Handle != handle {
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
	}), false)
	c := newTestClient(t, id, srv.Listener.Addr().String(), "127.0.0.1:12345")
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
	return Callbacks{Ready: func(protocol.Scope) {}, BulkReady: func(bool) {}, Disconnected: func() {}, Revoke: func(context.Context, protocol.Revocation) error { return nil }}
}
