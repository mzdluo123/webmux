package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"strconv"
	"sync"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/cmd/internal/assemble"
	"github.com/rainchan/webmux/internal/api"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/p2p"
	"github.com/rainchan/webmux/internal/relay"
	"github.com/rainchan/webmux/internal/relaysetting"
	"github.com/rainchan/webmux/internal/transport"
)

type relayControlClient interface {
	Enroll(context.Context, ed25519.PrivateKey, protocol.MachineID) (protocol.Enrollment, error)
	Poll(context.Context, protocol.Token) (protocol.EnrollmentResult, error)
	Cancel(context.Context, protocol.Token) error
	Delete(context.Context, ed25519.PrivateKey, protocol.Binding) (protocol.BindingDeletionResult, error)
	Assignment(context.Context, ed25519.PrivateKey, protocol.Binding) (protocol.Assignment, error)
	Connect(context.Context, ed25519.PrivateKey, protocol.Assignment, relay.Callbacks) error
}

type relayController struct {
	op            sync.Mutex
	mu            sync.Mutex
	ctx           context.Context
	store         *relaysetting.Service
	machine       *relaysetting.Machine
	client        relayControlClient
	auth          *relaysetting.Auth
	p2p           *p2p.Service
	public        *transport.PublicOrigins
	tokenStrong   bool
	settings      relaysetting.Settings
	status        contract.RelayStatus
	generation    uint64
	cancel        context.CancelFunc
	done          chan struct{}
	assignment    protocol.Assignment
	scope         protocol.Scope
	pairing       contract.RelayPairing
	pairingWake   chan struct{}
	pairingDone   chan struct{}
	pairingStop   context.CancelFunc
	pairingCancel bool
	pairingNow    func() time.Time
	pairingWait   func(context.Context, <-chan struct{}, time.Duration) bool
	closed        bool
}

var _ api.RelayControl = (*relayController)(nil)

func newRelayController(ctx context.Context, c config.Config, public *transport.PublicOrigins, address string, localTLS *tls.Config) (*relayController, error) {
	store := relaysetting.Open(c.StateDir, c.Relay)
	settings, web := store.Effective()
	source := "default"
	if c.Source("relay.control_endpoint") == "toml" {
		source = "toml"
	}
	if web {
		source = "web"
	}
	rc := &relayController{ctx: ctx, store: store, machine: relaysetting.OpenMachine(c.StateDir), settings: settings, public: public, tokenStrong: len(c.Token) >= 43}
	rc.pairing = contract.RelayPairing{Phase: "idle"}
	rc.pairingNow = time.Now
	rc.pairingWait = waitRelayPairing
	if settings.PendingSeed != "" {
		settings.PendingSeed = ""
		rc.settings = settings
		rc.pairing = contract.RelayPairing{Phase: "interrupted", Error: "pairing-interrupted"}
		if err := store.Save(settings); err != nil {
			rc.pairing.Error = "storage-unavailable"
		}
	}
	rc.status = contract.RelayStatus{State: "off", Transport: "wss", TCPAlgorithm: "unknown", ControlSource: source, ControlReachability: "unknown", DataReachability: "unassigned", AssignmentRevision: "0", ConfigRevision: "0", AppliedRevision: "0", AssignmentSource: "unassigned", Capabilities: []string{}, Sessions: []contract.RelaySessionDiagnostic{}}
	rc.status.Operation = contract.RelayOperation{Kind: "idle", Phase: "idle"}
	rc.status.HybridUnavailableReason = "p2p-feature-unavailable"
	client, err := relay.New(assemble.RelayOptions(settings.Config, address, localTLS))
	if err != nil {
		rc.status.Error = "control bootstrap unavailable"
		rc.status.ReadinessError = "control-bootstrap-unavailable"
		return rc, nil
	}
	rc.client = client
	rc.auth, err = relaysetting.NewAuth(relaysetting.AuthConfig{}, relaysetting.AuthDependencies{Client: client, Snapshot: rc.authSnapshot})
	if err != nil {
		store.Close()
		rc.machine.Close()
		return nil, err
	}
	return rc, nil
}

func (rc *relayController) authSnapshot() (relaysetting.AuthBinding, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if !rc.status.ControlReady || rc.settings.Binding == nil {
		return relaysetting.AuthBinding{}, false
	}
	return relaysetting.AuthBinding{Binding: *rc.settings.Binding, Scope: rc.scope, ControlOrigin: rc.settings.Config.ControlEndpoint}, true
}

func (rc *relayController) Status() contract.RelayStatus {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	s := rc.status
	s.Enabled = rc.settings.Config.Enabled
	s.Paired = rc.settings.Binding != nil
	s.Pairing = activeRelayPairing(rc.pairing.Phase)
	s.StorageAvailable = rc.store.Unavailable() == nil
	s.TokenStrong = rc.tokenStrong
	s.ControlEndpoint = rc.settings.Config.ControlEndpoint
	if rc.settings.Binding != nil {
		s.OwnerID = rc.settings.Binding.OwnerID
		s.OwnerLogin = rc.settings.OwnerLogin
		s.BoundPublicURL = rc.settings.Binding.PublicURL
	}
	if !s.StorageAvailable {
		s.ControlSource = "safety"
		s.Error = "relay settings storage unavailable"
		s.ReadinessError = "storage-unavailable"
	}
	s.Capabilities = append([]string{}, s.Capabilities...)
	s.Sessions = append([]contract.RelaySessionDiagnostic{}, s.Sessions...)
	if rc.p2p != nil {
		s.HybridSupported = rc.p2p.Supported()
		if s.HybridSupported {
			s.HybridUnavailableReason = ""
		} else {
			s.HybridUnavailableReason = "p2p-build-unavailable"
		}
		for _, peer := range rc.p2p.Status().Peers {
			if peer.OwnerID == "" {
				continue
			}
			// Negotiation without a selected ICE pair is not a candidate-path
			// observation. Selection here does not imply browser business use.
			switch peer.Path {
			case "webrtc-direct", "webrtc-turn":
				s.Sessions = append(s.Sessions, contract.RelaySessionDiagnostic{ID: peer.OwnerID, Path: peer.Path, CandidateType: peer.CandidateType, Transport: peer.Transport})
			}
		}
	}
	return s
}

func (rc *relayController) Update(_ context.Context, req contract.RelayUpdate) (contract.RelayStatus, error) {
	rc.op.Lock()
	defer rc.op.Unlock()
	if req.Enabled == nil {
		return rc.Status(), relaysetting.ErrInvalid
	}
	kind := "disable"
	if *req.Enabled {
		kind = "enable"
	}
	rc.beginOperation(kind)
	rc.mu.Lock()
	next := rc.settings
	rc.mu.Unlock()
	if *req.Enabled && (!req.ConfirmPublic || !rc.tokenStrong || next.Binding == nil || rc.client == nil || rc.store.Unavailable() != nil) {
		return rc.finishOperation(relaysetting.ErrEnableBlocked, false)
	}
	ctx, cancel := context.WithTimeout(rc.ctx, protocol.RequestTimeout)
	defer cancel()
	next.Config.Enabled = *req.Enabled
	if *req.Enabled {
		next.Confirmed = true
	}
	rc.stop(ctx)
	if err := rc.store.Save(next); err != nil {
		if !*req.Enabled {
			return rc.finishOperation(relaysetting.ErrNotPersisted, false)
		}
		return rc.finishOperation(err, false)
	}
	rc.mu.Lock()
	rc.settings = next
	rc.mu.Unlock()
	if *req.Enabled {
		rc.restart()
	}
	return rc.finishOperation(nil, false)
}

func (rc *relayController) Reconnect(_ context.Context) (contract.RelayStatus, error) {
	rc.op.Lock()
	defer rc.op.Unlock()
	rc.beginOperation("reconnect")
	rc.mu.Lock()
	enabled := rc.settings.Config.Enabled
	rc.mu.Unlock()
	if !enabled || rc.client == nil || rc.store.Unavailable() != nil {
		return rc.finishOperation(relaysetting.ErrEnableBlocked, false)
	}
	ctx, cancel := context.WithTimeout(rc.ctx, protocol.RequestTimeout)
	defer cancel()
	rc.stop(ctx)
	rc.mu.Lock()
	rc.settings.Config.Enabled = true
	rc.mu.Unlock()
	rc.restart()
	return rc.finishOperation(nil, false)
}

func (rc *relayController) beginOperation(kind string) {
	rc.mu.Lock()
	rc.status.Operation = contract.RelayOperation{Kind: kind, Phase: "submitting"}
	rc.mu.Unlock()
}

func (rc *relayController) finishOperation(err error, uncertain bool) (contract.RelayStatus, error) {
	rc.mu.Lock()
	rc.status.Operation.Phase = "completed"
	rc.status.Operation.Error = ""
	if err != nil {
		rc.status.Operation.Phase = "failed"
		if uncertain {
			rc.status.Operation.Phase = "uncertain"
		}
		rc.status.Operation.Error = safeRelayError(err)
	}
	rc.mu.Unlock()
	return rc.Status(), err
}

func (rc *relayController) stop(ctx context.Context) {
	rc.mu.Lock()
	rc.generation++
	rc.public.Clear("relay")
	rc.settings.Config.Enabled = false
	if rc.cancel != nil {
		rc.cancel()
		rc.cancel = nil
	}
	rc.status.ControlReady = false
	rc.status.BulkReady = false
	rc.status.ReadyConnections = 0
	rc.status.PublicURL = ""
	rc.status.State = "off"
	rc.scope = protocol.Scope{}
	rc.mu.Unlock()
	if rc.auth != nil {
		_ = rc.auth.InvalidateAll(ctx)
	}
}

func (rc *relayController) restart() {
	rc.mu.Lock()
	if !rc.settings.Config.Enabled || !rc.settings.Confirmed || !rc.tokenStrong || rc.settings.Binding == nil || rc.client == nil || rc.store.Unavailable() != nil {
		rc.status.State = "failed"
		rc.status.ReadinessError = "enable-precondition"
		rc.mu.Unlock()
		return
	}
	rc.status.ControlReady = false
	rc.status.BulkReady = false
	rc.status.ReadyConnections = 0
	rc.status.PublicURL = ""
	if rc.cancel != nil {
		rc.cancel()
	}
	rc.public.Clear("relay")
	rc.generation++
	gen := rc.generation
	ctx, cancel := context.WithCancel(rc.ctx)
	rc.cancel = cancel
	previous := rc.done
	done := make(chan struct{})
	rc.done = done
	settings := rc.settings
	rc.status.State = "connecting"
	rc.status.Error = ""
	rc.status.ReadinessError = ""
	rc.mu.Unlock()
	go func() {
		defer close(done)
		// Even a cancelled intermediate worker joins its predecessor; otherwise
		// a third update could race an older connector still shutting down.
		if previous != nil {
			<-previous
		}
		if ctx.Err() != nil {
			return
		}
		rc.run(ctx, gen, settings)
	}()
}

func (rc *relayController) run(ctx context.Context, gen uint64, settings relaysetting.Settings) {
	key, err := relaysetting.PrivateKey(settings.Seed)
	if err != nil {
		return
	}
	backoff := time.Second
	for ctx.Err() == nil {
		if rc.store.Unavailable() != nil {
			rc.disconnected(gen)
			return
		}
		assignment, err := rc.client.Assignment(ctx, key, *settings.Binding)
		if err == nil {
			rc.mu.Lock()
			if gen != rc.generation {
				rc.mu.Unlock()
				return
			}
			rc.assignment = assignment
			rc.status.ControlReachability = "reachable"
			rc.status.DataReachability = "unknown"
			rc.status.ConnectorURL = assignment.ConnectorURL
			rc.status.DataOrigin = assignment.DataOrigin
			rc.status.AssignmentRevision = strconv.FormatUint(uint64(assignment.Revision), 10)
			rc.status.ConfigRevision = strconv.FormatUint(uint64(assignment.ConfigRevision), 10)
			rc.status.NodeID = string(assignment.NodeID)
			rc.status.Capabilities = []string{}
			for _, cap := range assignment.Capabilities {
				rc.status.Capabilities = append(rc.status.Capabilities, string(cap))
			}
			rc.status.AssignmentSource = "authenticated-control"
			rc.mu.Unlock()
			err = rc.client.Connect(ctx, key, assignment, relay.Callbacks{
				Ready: func(scope protocol.Scope) {
					rc.connected(ctx, gen, settings, scope)
				},
				BulkReady: func(ready bool) {
					rc.mu.Lock()
					defer rc.mu.Unlock()
					if gen != rc.generation {
						return
					}
					rc.status.BulkReady = ready
					if ready {
						rc.status.ReadyConnections = 2
						rc.status.State = "ready"
					} else if rc.status.ControlReady {
						rc.status.ReadyConnections = 1
						rc.status.State = "degraded"
					}
				},
				Disconnected: func() { rc.disconnected(gen) },
				Revoke:       func(ctx context.Context, r protocol.Revocation) error { return rc.revoke(ctx, gen, r) },
			})
		} else {
			rc.mu.Lock()
			if gen == rc.generation {
				rc.status.ControlReachability = "unavailable"
			}
			rc.mu.Unlock()
		}
		rc.disconnected(gen)
		if err != nil && ctx.Err() == nil {
			rc.mu.Lock()
			if gen == rc.generation {
				rc.status.ReconnectReason = "connection-failed"
			}
			rc.mu.Unlock()
		}
		if ctx.Err() != nil {
			return
		}
		rc.mu.Lock()
		if gen != rc.generation {
			rc.mu.Unlock()
			return
		}
		rc.status.State = "failed"
		rc.status.Error = "relay connection unavailable"
		rc.status.ReadinessError = "connection-unavailable"
		rc.status.ReconnectReason = "connection-lost"
		rc.status.DataReachability = "unavailable"
		rc.mu.Unlock()
		var jitter [1]byte
		if _, e := rand.Read(jitter[:]); e != nil {
			return
		}
		delay := backoff + time.Duration(jitter[0])*backoff/512
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

func (rc *relayController) connected(ctx context.Context, gen uint64, settings relaysetting.Settings, scope protocol.Scope) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if gen != rc.generation || ctx.Err() != nil {
		return
	}
	if rc.public.Set("relay", settings.Binding.PublicURL) != nil {
		return
	}
	rc.scope = scope
	rc.status.ControlReady = true
	rc.status.State = "degraded"
	rc.status.ReadyConnections = 1
	rc.status.PublicURL = settings.Binding.PublicURL
	rc.status.DataReachability = "reachable"
	rc.status.AppliedRevision = rc.status.ConfigRevision
	rc.status.Error = ""
	rc.status.ReadinessError = ""
	rc.status.ReconnectReason = ""
}

func (rc *relayController) disconnected(gen uint64) {
	rc.mu.Lock()
	if gen != rc.generation {
		rc.mu.Unlock()
		return
	}
	rc.public.Clear("relay")
	rc.status.ControlReady = false
	rc.status.BulkReady = false
	rc.status.ReadyConnections = 0
	rc.status.PublicURL = ""
	rc.scope = protocol.Scope{}
	rc.mu.Unlock()
	if rc.auth != nil {
		ctx, cancel := context.WithTimeout(context.Background(), protocol.RequestTimeout)
		defer cancel()
		_ = rc.auth.InvalidateAll(ctx)
	}
}

func (rc *relayController) revoke(ctx context.Context, gen uint64, r protocol.Revocation) error {
	if r.Scope == "binding" || r.Scope == "connector" {
		rc.mu.Lock()
		if gen == rc.generation {
			rc.public.Clear("relay")
			rc.status.ControlReady = false
			rc.status.BulkReady = false
			rc.status.ReadyConnections = 0
			rc.status.PublicURL = ""
			rc.scope = protocol.Scope{}
		}
		rc.mu.Unlock()
	}
	return rc.auth.Revoke(ctx, r)
}

func (rc *relayController) Start() {
	rc.op.Lock()
	defer rc.op.Unlock()
	rc.mu.Lock()
	enabled := rc.settings.Config.Enabled
	rc.mu.Unlock()
	if enabled {
		rc.restart()
	}
}

func (rc *relayController) Close() {
	rc.mu.Lock()
	rc.closed = true
	pairingDone := rc.pairingDone
	if rc.pairingStop != nil {
		rc.pairingStop()
	}
	rc.mu.Unlock()
	if pairingDone != nil {
		<-pairingDone
	}
	rc.op.Lock()
	defer rc.op.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), protocol.RequestTimeout)
	defer cancel()
	rc.stop(ctx)
	rc.mu.Lock()
	done := rc.done
	rc.mu.Unlock()
	if done != nil {
		<-done
	}
	if rc.auth != nil {
		_ = rc.auth.Close()
	}
	rc.store.Close()
	rc.machine.Close()
}
