package relay

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/frphelper"
)

const frpObservationTTL = 6 * time.Second

type timedFRPStatus struct {
	protocol.AllocationStatus
	until time.Time
}

// DataObservation reports a node probe for the current grant. A zero value
// invalidates freshness without replacing the last sampled time. RouteChanged
// also drops the sample from the retired registration.
type DataObservation struct {
	CheckedAt    time.Time
	ValidUntil   time.Time
	Ready        bool
	RouteChanged bool
}

func (st timedFRPStatus) observation(now time.Time, grant *protocol.AllocationGrant) (DataObservation, bool) {
	if st.AllocationID != grant.AllocationID || st.GrantRevision != grant.GrantRevision || !now.Before(st.until) {
		return DataObservation{}, false
	}
	if st.CheckedAt == 0 {
		return DataObservation{}, true
	}
	checkedAt := time.UnixMilli(st.CheckedAt).UTC()
	until := minTime(st.until, time.Unix(grant.ExpiresAt, 0))
	until = minTime(until, minTime(checkedAt.Add(frpObservationTTL), now.Add(frpObservationTTL)))
	return DataObservation{CheckedAt: checkedAt, ValidUntil: until, Ready: st.DataReady}, true
}

type frpGrantResult struct {
	grant     *protocol.AllocationGrant
	requested *protocol.AllocationGrant
	err       error
}

func (s *connection) requestFRP(ctx context.Context, csr string, current *protocol.AllocationGrant) <-chan frpGrantResult {
	result := make(chan frpGrantResult, 1)
	go func() {
		if current != nil && current.Transport != nil {
			r := protocol.TransportRequest{Action: protocol.TransportRenew, OperationID: current.Transport.OperationID, RouteVersion: current.Transport.RouteVersion,
				AllocationRenew: &protocol.AllocationRenew{AllocationID: current.AllocationID, GrantRevision: current.GrantRevision}}
			reply, err := s.callTransport(ctx, r)
			if err == nil && (reply.State != protocol.TransportCommitted || reply.Active != current.Protocol() ||
				reply.ActiveAllocationID != current.AllocationID || reply.Grant == nil) {
				err = protocol.Error{Code: protocol.StaleGeneration}
			}
			result <- frpGrantResult{grant: reply.Grant, requested: current, err: err}
			return
		}
		id, err := newToken()
		if err != nil {
			result <- frpGrantResult{requested: current, err: err}
			return
		}
		message := s.rpc.message(protocol.ControlFRPAllocate, id)
		if current == nil {
			message.AllocationRequest = &protocol.AllocationRequest{HostServerCSR: csr}
		} else {
			message.Type = protocol.ControlFRPRenew
			message.AllocationRenew = &protocol.AllocationRenew{AllocationID: current.AllocationID, GrantRevision: current.GrantRevision}
		}
		reply, err := s.rpc.call(ctx, message, protocol.ControlFRPGrant, nil)
		result <- frpGrantResult{grant: reply.AllocationGrant, requested: current, err: err}
	}()
	return result
}

func (s *connection) validateFRPGrant(g, current *protocol.AllocationGrant) error {
	if g == nil || g.NodeOrigin != nil || g.ExpiresAt-g.IssuedAt > 120 || g.ExpiresAt-g.NotBefore > 125 ||
		g.RemotePort < protocol.FRPPortMin || g.RemotePort > protocol.FRPPortMax {
		return protocol.Error{Code: protocol.Invalid}
	}
	if g.Protocol() == protocol.FRPTransportWSS {
		connector, _ := url.Parse(s.assignment.ConnectorURL)
		endpoint, err := url.Parse(g.TransportURL)
		if err != nil || endpoint.Hostname() != connector.Hostname() {
			return protocol.Error{Code: protocol.WrongAudience}
		}
	} else if s.control == nil || !s.control.selection || g.Protocol() != protocol.FRPTransportQUIC || g.Transport.Validate() != nil ||
		g.TransportURL != "" || g.BridgeKey != "" {
		return protocol.Error{Code: protocol.Forbidden}
	}
	// The management channel authenticates this grant. Check that the opaque
	// credential actually describes the same binding; stock frps and node
	// admission independently verify the signature, issuer, and active JTI.
	_, pieces, _ := strings.Cut(g.JWT, ".")
	body, _, _ := strings.Cut(pieces, ".")
	raw, err := base64.RawURLEncoding.Strict().DecodeString(body)
	var claims protocol.FRPClaims
	if err != nil || protocol.Decode(raw, &claims, protocol.MaxControlBytes) != nil || claims.ID.Validate() != nil || claims.Issuer == "" ||
		!g.Transport.Equal(claims.Transport) {
		return protocol.Error{Code: protocol.Forbidden}
	}
	claims.Transport = nil
	if claims != (protocol.FRPClaims{Issuer: claims.Issuer, Audience: protocol.FRPAudience(s.assignment.NodeID), Subject: string(g.AllocationID), ID: claims.ID,
		IssuedAt: g.IssuedAt, NotBefore: g.NotBefore, ExpiresAt: g.ExpiresAt, Scope: s.scope, NodeID: s.assignment.NodeID,
		AllocationID: g.AllocationID, GrantRevision: g.GrantRevision, Role: protocol.FRPRole, ProxyName: g.ProxyName, RemotePort: g.RemotePort}) {
		return protocol.Error{Code: protocol.Forbidden}
	}
	if current == nil {
		return nil
	}
	if g.AllocationID != current.AllocationID || g.GrantRevision != current.GrantRevision+1 ||
		g.ProxyName != current.ProxyName || g.RemotePort != current.RemotePort || g.TransportURL != current.TransportURL ||
		g.BridgeKey != current.BridgeKey || g.Origin != current.Origin || g.JWT == current.JWT || !g.Transport.Equal(current.Transport) {
		return protocol.Error{Code: protocol.StaleGeneration}
	}
	return nil
}

func frpBackoff(failures int) time.Duration {
	base := min(time.Second<<min(max(failures-1, 0), 5), 20*time.Second)
	return min(20*time.Second, base+rand.N(base/2))
}

func (s *connection) runFRP(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	boot := s.beginSavedTransport()
	failInit := func(code protocol.Code) {
		s.publishTransport(false, code)
		if boot != nil {
			s.finishTransportAttempt(boot, selectionOutcome{stage: "prepare", err: protocol.Error{Code: code}})
		}
	}
	if err := frphelper.Available(); err != nil {
		failInit(protocol.InvalidConfig)
		return
	}
	dir, err := os.MkdirTemp("", "webmux-frpc-")
	if err != nil {
		failInit(protocol.InvalidConfig)
		return
	}
	defer removeRegistrationDir(dir)
	dir, err = filepath.Abs(dir)
	if err != nil {
		failInit(protocol.InvalidConfig)
		return
	}
	executable, err := frphelper.Materialize(dir)
	if err != nil {
		failInit(protocol.InvalidConfig)
		return
	}
	key, csr, err := newOriginCSR()
	if err != nil {
		failInit(protocol.Unavailable)
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	var (
		current        *protocol.AllocationGrant
		data, retiring *frpRegistration
		candidate      <-chan candidateResult
		operation      = boot
		waitProbe      bool
		recovery       bool // Explicit recovery on an old node, with no old route.
		legacy         = s.client.TransportSelection().Preferred == protocol.FRPTransportWSS
		verifiedUntil  time.Time
		retireAt       time.Time
		pending        <-chan frpGrantResult
		lost, exited   <-chan struct{}
		badTLS         <-chan struct{}
		latestStatus   timedFRPStatus
		runningSince   time.Time
		nextRequest    = time.Now()
		reportedReason protocol.Code
		failures       int
		rpcFailures    int
	)
	finish := func(outcome selectionOutcome) {
		s.finishTransportAttempt(operation, outcome)
		operation, waitProbe = nil, false
	}
	setRuntime := func(next *frpRegistration) {
		data, lost, exited, badTLS = next, nil, nil, nil
		runningSince = time.Time{}
		if data != nil {
			exited = data.exited
			if data.bridge != nil {
				lost, badTLS = data.bridge.lost, data.bridge.badTLS
			}
			runningSince = time.Now()
			reportedReason = ""
		}
	}
	reportFailure := func(reason protocol.Code) {
		reportedReason = reason
		s.setActiveTransport("")
		s.publishTransport(false, reason)
	}
	stop := func(reason protocol.Code) {
		if data != nil {
			data.access.end()
		}
		reportFailure(reason)
		if data != nil {
			data.close()
		}
		setRuntime(nil)
		verifiedUntil = time.Time{}
	}
	defer func() {
		cancel()
		s.setActiveTransport("")
		if data != nil {
			data.close()
		}
		if retiring != nil {
			retiring.close()
		}
		if candidate != nil {
			result := <-candidate
			if result.data != nil {
				result.data.close()
			}
		}
		if operation != nil {
			finish(unavailableSelection())
		}
		if pending != nil {
			<-pending
		}
	}()
	applyStatus := func(st timedFRPStatus) {
		now := time.Now()
		if data == nil || !data.access.active(now) {
			return
		}
		if observation, ok := st.observation(now, current); ok {
			s.publishObservation(observation)
			if observation.Ready && observation.ValidUntil.After(now) {
				verifiedUntil = observation.ValidUntil
				s.setActiveTransport(current.Protocol())
			} else {
				verifiedUntil = time.Time{}
				s.setActiveTransport("")
			}
		}
	}
	restart := func(reason protocol.Code) {
		if data != nil && (!verifiedUntil.IsZero() || data.bridge != nil && data.bridge.login.Load()) && time.Since(runningSince) >= protocol.ControlLease {
			failures = 0
		}
		stop(reason)
		failures++
		delay := frpBackoff(failures)
		if failures >= 5 {
			delay = time.Minute + rand.N(10*time.Second)
		}
		nextRequest = time.Now().Add(delay)
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for ctx.Err() == nil {
		now := time.Now()
		if data != nil {
			if data.access.ended.Load() {
				restart(protocol.Unavailable)
			} else if !data.access.active(now) {
				stop(protocol.Unavailable)
			}
		}
		if retiring != nil && !now.Before(retireAt) {
			retiring.close()
			retiring = nil
		}
		if !verifiedUntil.IsZero() && !now.Before(verifiedUntil) {
			verifiedUntil = time.Time{}
			s.setActiveTransport("")
		}
		if operation != nil && candidate == nil && pending == nil && !waitProbe {
			switch {
			case operation.ctx.Err() != nil:
				finish(selectionOutcome{stage: "prepare", err: operation.ctx.Err()})
			case current != nil && current.Protocol() == operation.desired:
				waitProbe = true
			case retiring != nil:
				finish(selectionOutcome{stage: "prepare", err: protocol.Error{Code: protocol.Conflict}})
			case s.control.selection:
				candidate = s.startTransportCandidate(ctx, operation, dir, executable, current)
			case operation.desired == protocol.FRPTransportWSS:
				recovery = !legacy && current == nil
				legacy, waitProbe = true, true
				nextRequest = now
			default:
				reportFailure(protocol.Incompatible)
				finish(selectionOutcome{stage: "prepare", err: protocol.Error{Code: protocol.Incompatible}})
			}
		}
		var wake time.Time
		var cancelProbe <-chan struct{}
		if waitProbe {
			deadline := operation.started.Add(protocol.TransportCandidateTTL)
			switch {
			case operation.ctx.Err() != nil || !now.Before(deadline):
				probeErr := operation.ctx.Err()
				if probeErr == nil {
					probeErr = protocol.Error{Code: protocol.Expired}
				}
				finish(selectionOutcome{stage: "probe", err: probeErr})
				if recovery {
					// A legacy node can revoke this unconfirmed route by
					// fencing its management generation.
					go s.stop(protocol.Error{Code: protocol.Unavailable})
					return
				}
			case verifiedUntil.After(now):
				finish(selectionOutcome{stage: "commit"})
				recovery = false
			default:
				cancelProbe = operation.ctx.Done()
				wake = deadline
			}
		}
		if pending == nil && (current != nil || legacy && candidate == nil) {
			if !now.Before(nextRequest) {
				pending = s.requestFRP(ctx, csr, current)
				// Status can overtake the corresponding grant result.
				latestStatus = timedFRPStatus{}
			} else {
				wake = earlierWake(wake, nextRequest)
			}
		}
		if data != nil {
			wake = earlierWake(wake, time.Unix(current.ExpiresAt, 0))
		}
		if retiring != nil {
			wake = earlierWake(wake, retireAt)
		}
		if !verifiedUntil.IsZero() {
			wake = earlierWake(wake, verifiedUntil)
		}
		var alarm <-chan time.Time
		if !wake.IsZero() {
			timer.Reset(time.Until(wake))
			alarm = timer.C
		} else {
			timer.Stop()
		}
		select {
		case <-ctx.Done():
			return
		case <-alarm:
		case <-cancelProbe:
		case request := <-s.transport:
			operation = request
		case <-lost:
			restart(protocol.Unavailable)
		case <-exited:
			code := protocol.Unavailable
			if reportedReason == protocol.Unauthenticated && data.bridge != nil && !data.bridge.login.Load() {
				code = protocol.Unauthenticated
			}
			select {
			case <-badTLS:
				code = protocol.Unauthenticated
			default:
			}
			restart(code)
		case <-badTLS:
			if data != nil && !data.bridge.login.Load() {
				reportedReason = protocol.Unauthenticated
			}
		case st := <-s.status:
			applyStatus(st)
			if pending != nil && (current == nil || st.AllocationID == current.AllocationID && st.GrantRevision == current.GrantRevision+1) {
				latestStatus = st
			}
		case result := <-candidate:
			candidate = nil
			if ctx.Err() != nil {
				if result.data != nil {
					result.data.close()
				}
				return
			}
			if result.outcome.err != nil {
				if data == nil {
					reportFailure(selectionErrorCode(result.outcome.err))
				}
				finish(result.outcome)
				if result.uncertain {
					go s.stop(protocol.Error{Code: protocol.Unavailable})
					return
				}
				continue
			}
			// Commit already changed the node's route. Admit another
			// candidate only after retiring the old child.
			if data != nil {
				retiring, retireAt = data, time.Now().Add(2*time.Second)
			}
			current, key = result.grant, result.key
			setRuntime(result.data)
			verifiedUntil, legacy = time.Now().Add(frpObservationTTL), false
			nextRequest = time.Now() // Extend the short candidate grant promptly.
			rpcFailures, failures = 0, 0
			s.publishObservation(DataObservation{RouteChanged: true})
			s.setActiveTransport(current.Protocol())
			s.publishTransport(true, "")
			finish(result.outcome)
		case result := <-pending:
			pending = nil
			if ctx.Err() != nil {
				return
			}
			if result.requested != current {
				continue // An old route's renewal raced a successful commit.
			}
			if result.err != nil {
				if data == nil && reportedReason != protocol.Unauthenticated {
					reportFailure(protocol.Unavailable)
				}
				rpcFailures++
				delay := frpBackoff(rpcFailures)
				var remote protocol.Error
				if errors.As(result.err, &remote) {
					if current != nil && current.Transport != nil && remote.Code == protocol.StaleGeneration && data == nil {
						go s.stop(protocol.Error{Code: protocol.Unavailable})
						return
					}
					delay = max(delay, min(time.Duration(remote.RetryAfterSeconds)*time.Second, time.Minute))
				}
				nextRequest = maxTime(nextRequest, time.Now().Add(delay))
				continue
			}
			g := result.grant
			if err := s.validateFRPGrant(g, current); err != nil {
				stop(protocol.Unauthenticated)
				return
			}
			if data != nil && !data.access.active(time.Now()) {
				// Nothing from the expired ticket may be carried into the
				// new revision, even if its reply and the timer raced.
				if data.access.ended.Load() {
					restart(protocol.Unavailable)
				} else {
					stop(protocol.Unavailable)
				}
			}
			rpcFailures = 0
			if data == nil && time.Now().Before(nextRequest) {
				// A renewal crossed a failure and its restart cooldown. This
				// revision is accepted, but no child runs until the cooldown ends.
				current = g
				continue
			}
			current = g
			nextRequest = time.Unix(g.RenewAfter, 0)
			if data == nil {
				next, err := s.startRegistration(ctx, dir, executable, key, *g)
				if err != nil {
					code := protocolCode(err)
					if code == protocol.InvalidConfig || code == protocol.Unauthenticated {
						reportFailure(code)
						return
					}
					restart(code)
					continue
				}
				setRuntime(next)
				if g.Transport != nil {
					s.publishTransport(true, "")
				}
			} else {
				if time.Unix(g.ExpiresAt, 0).After(data.originUntil) || replaceFRPToken(data.tokenFile, g.JWT) != nil {
					stop(protocol.InvalidConfig)
					return
				}
				if !data.access.active(time.Now()) {
					restart(protocol.Unavailable)
					continue
				}
				// Ordinary renewal preserves origin authorization and sockets.
				// The next diagnostic sample must belong to the new revision.
				data.access.grant.Store(time.Unix(g.ExpiresAt, 0).UnixNano())
				s.publishObservation(DataObservation{})
			}
			applyStatus(latestStatus)
		}
	}
}

type frpRegistration struct {
	*frpRuntime
	dir string
}

func (s *connection) startRegistration(ctx context.Context, dir, executable string, key *ecdsa.PrivateKey, g protocol.AllocationGrant) (*frpRegistration, error) {
	child, err := os.MkdirTemp(dir, "registration-")
	if err != nil {
		return nil, protocol.Error{Code: protocol.InvalidConfig}
	}
	data, err := s.startFRP(ctx, child, executable, key, g)
	if err != nil {
		removeRegistrationDir(child)
		return nil, err
	}
	return &frpRegistration{frpRuntime: data, dir: child}, nil
}

func (r *frpRegistration) close() {
	r.frpRuntime.close()
	removeRegistrationDir(r.dir)
}

func removeRegistrationDir(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("relay could not remove the private frpc runtime directory")
	}
}

func earlierWake(wake, next time.Time) time.Time {
	if wake.IsZero() || next.Before(wake) {
		return next
	}
	return wake
}

func selectionErrorCode(err error) protocol.Code {
	if errors.Is(err, context.DeadlineExceeded) {
		return protocol.Expired
	}
	if errors.Is(err, context.Canceled) {
		return protocol.Unavailable
	}
	return protocolCode(err)
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
