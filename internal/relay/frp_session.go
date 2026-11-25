package relay

import (
	"context"
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

const frpStatusTTL = 6 * time.Second

type timedFRPStatus struct {
	protocol.AllocationStatus
	until time.Time
}

type frpGrantResult struct {
	grant *protocol.AllocationGrant
	err   error
}

func (s *connection) requestFRP(ctx context.Context, csr string, current *protocol.AllocationGrant) <-chan frpGrantResult {
	result := make(chan frpGrantResult, 1)
	go func() {
		id, err := newToken()
		if err != nil {
			result <- frpGrantResult{err: err}
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
		result <- frpGrantResult{grant: reply.AllocationGrant, err: err}
	}()
	return result
}

func (s *connection) validateFRPGrant(g, current *protocol.AllocationGrant) error {
	connector, _ := url.Parse(s.assignment.ConnectorURL)
	endpoint, err := url.Parse(g.TransportURL)
	if err != nil || endpoint.Hostname() != connector.Hostname() || g.RemotePort < protocol.FRPPortMin || g.RemotePort > protocol.FRPPortMax {
		return protocol.Error{Code: protocol.WrongAudience}
	}
	if g.NodeOrigin != nil || g.ExpiresAt-g.IssuedAt > 120 || g.ExpiresAt-g.NotBefore > 125 {
		return protocol.Error{Code: protocol.Invalid}
	}
	// The management channel authenticates this grant. Check that the opaque
	// credential actually describes the same binding; stock frps and node
	// admission independently verify the signature, issuer, and active JTI.
	_, pieces, _ := strings.Cut(g.JWT, ".")
	body, _, _ := strings.Cut(pieces, ".")
	raw, err := base64.RawURLEncoding.Strict().DecodeString(body)
	var claims protocol.FRPClaims
	if err != nil || protocol.Decode(raw, &claims, protocol.MaxControlBytes) != nil || claims.ID.Validate() != nil || claims.Issuer == "" ||
		claims != (protocol.FRPClaims{Issuer: claims.Issuer, Audience: protocol.FRPAudience(s.assignment.NodeID), Subject: string(g.AllocationID), ID: claims.ID,
			IssuedAt: g.IssuedAt, NotBefore: g.NotBefore, ExpiresAt: g.ExpiresAt, Scope: s.scope, NodeID: s.assignment.NodeID,
			AllocationID: g.AllocationID, GrantRevision: g.GrantRevision, Role: protocol.FRPRole, ProxyName: g.ProxyName, RemotePort: g.RemotePort}) {
		return protocol.Error{Code: protocol.Forbidden}
	}
	if current == nil {
		return nil
	}
	if g.AllocationID != current.AllocationID || g.GrantRevision != current.GrantRevision+1 ||
		g.ProxyName != current.ProxyName || g.RemotePort != current.RemotePort || g.TransportURL != current.TransportURL ||
		g.BridgeKey != current.BridgeKey || g.Origin != current.Origin || g.JWT == current.JWT {
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
	if err := frphelper.Available(); err != nil {
		s.publishData(false, protocol.InvalidConfig)
		return
	}
	dir, err := os.MkdirTemp("", "webmux-frpc-")
	if err != nil {
		s.publishData(false, protocol.InvalidConfig)
		return
	}
	defer func(owned string) {
		if err := os.RemoveAll(owned); err != nil {
			slog.Warn("relay could not remove the private frpc runtime directory")
		}
	}(dir)
	dir, err = filepath.Abs(dir)
	if err != nil {
		s.publishData(false, protocol.InvalidConfig)
		return
	}
	executable, err := frphelper.Materialize(dir)
	if err != nil {
		s.publishData(false, protocol.InvalidConfig)
		return
	}
	key, csr, err := newOriginCSR()
	if err != nil {
		s.publishData(false, protocol.Unavailable)
		return
	}
	var (
		current        *protocol.AllocationGrant
		data           *frpRuntime
		pending        <-chan frpGrantResult
		lost, exited   <-chan struct{}
		badTLS         <-chan struct{}
		latestStatus   timedFRPStatus
		readyUntil     time.Time
		readySince     time.Time
		nextRequest    = time.Now()
		reportedReady  bool
		reportedReason protocol.Code
		failures       int
		rpcFailures    int
	)
	report := func(ready bool, reason protocol.Code, until time.Time) {
		readyUntil = until
		if data != nil && !ready {
			data.access.status.Store(0)
		}
		if ready && readySince.IsZero() {
			readySince = time.Now()
		} else if !ready {
			readySince = time.Time{}
		}
		if ready != reportedReady || reason != reportedReason {
			reportedReady, reportedReason = ready, reason
			s.publishData(ready, reason)
		}
		if data != nil && ready {
			data.access.status.Store(until.UnixNano())
		}
	}
	stop := func(reason protocol.Code) {
		if data != nil {
			data.access.end()
		}
		report(false, reason, time.Time{})
		if data != nil {
			data.close()
		}
		data, lost, exited, badTLS = nil, nil, nil, nil
	}
	defer func() {
		stop(protocol.Unavailable)
		if pending != nil {
			<-pending // The only in-flight request observes ctx cancellation.
		}
	}()
	applyStatus := func(st timedFRPStatus) {
		if current == nil || st.AllocationID != current.AllocationID || st.GrantRevision != current.GrantRevision || data == nil {
			return
		}
		now := time.Now()
		until := minTime(st.until, time.Unix(current.ExpiresAt, 0))
		if !now.Before(until) || !data.access.active(now) {
			return
		}
		if st.DataReady {
			if data.bridge.login.Load() {
				report(true, "", until)
			}
		} else {
			reason := protocol.Unavailable
			if st.Reason == protocol.AllocationStatusProbeFailed {
				reason = protocol.UpstreamFailed
			}
			report(false, reason, time.Time{})
		}
	}
	restart := func(reason protocol.Code) {
		if !readySince.IsZero() && time.Since(readySince) >= protocol.ControlLease {
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
			} else if reportedReady && !now.Before(readyUntil) {
				report(false, protocol.Unavailable, time.Time{})
			}
		}
		if pending == nil && !now.Before(nextRequest) {
			pending = s.requestFRP(ctx, csr, current)
			// A status may overtake delivery of the correlated grant result to
			// this worker. Keep one candidate while the request is pending.
			latestStatus = timedFRPStatus{}
		}
		var wake time.Time
		if pending == nil {
			wake = nextRequest
		}
		if data != nil {
			expires := time.Unix(current.ExpiresAt, 0)
			if wake.IsZero() || expires.Before(wake) {
				wake = expires
			}
			if reportedReady {
				wake = minTime(wake, readyUntil)
			}
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
		case <-lost:
			restart(protocol.Unavailable)
		case <-exited:
			code := protocol.Unavailable
			if reportedReason == protocol.Unauthenticated {
				code = protocol.Unauthenticated
			}
			select {
			case <-badTLS:
				code = protocol.Unauthenticated
			default:
			}
			restart(code)
		case <-badTLS:
			if !reportedReady {
				report(false, protocol.Unauthenticated, time.Time{})
			}
		case st := <-s.status:
			applyStatus(st)
			if pending != nil && (current == nil || st.AllocationID == current.AllocationID && st.GrantRevision == current.GrantRevision+1) {
				latestStatus = st
			}
		case result := <-pending:
			pending = nil
			if ctx.Err() != nil {
				return
			}
			if result.err != nil {
				if data == nil && reportedReason != protocol.Unauthenticated {
					report(false, protocol.Unavailable, time.Time{})
				}
				rpcFailures++
				delay := frpBackoff(rpcFailures)
				var remote protocol.Error
				if errors.As(result.err, &remote) {
					delay = max(delay, min(time.Duration(remote.RetryAfterSeconds)*time.Second, time.Minute))
				}
				nextRequest = maxTime(nextRequest, time.Now().Add(delay))
				continue
			}
			g := result.grant
			if err := s.validateFRPGrant(g, current); err != nil {
				stop(protocol.Unauthenticated)
				<-ctx.Done()
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
				data, err = s.startFRP(ctx, dir, executable, key, *g)
				if err != nil {
					code := protocolCode(err)
					if code == protocol.InvalidConfig || code == protocol.Unauthenticated {
						report(false, code, time.Time{})
						<-ctx.Done()
						return
					}
					restart(code)
					continue
				}
				lost, exited, badTLS = data.bridge.lost, data.exited, data.bridge.badTLS
			} else {
				if time.Unix(g.ExpiresAt, 0).After(data.originUntil) || replaceFRPToken(data.tokenFile, g.JWT) != nil {
					stop(protocol.InvalidConfig)
					<-ctx.Done()
					return
				}
				if !data.access.active(time.Now()) {
					restart(protocol.Unavailable)
					continue
				}
				// Keep the child and existing application sockets. Only the
				// new revision's mTLS probe can enable fresh origin requests.
				report(false, protocol.Unavailable, time.Time{})
				data.access.grant.Store(time.Unix(g.ExpiresAt, 0).UnixNano())
			}
			applyStatus(latestStatus)
		}
	}
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
