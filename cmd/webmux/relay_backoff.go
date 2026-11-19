package main

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

const (
	minRelayReconnectBackoff = time.Second
	maxRelayReconnectBackoff = 30 * time.Second
	relayStableThreshold     = protocol.ControlLease
)

type relayReconnectBackoff struct {
	current   time.Duration
	min       time.Duration
	max       time.Duration
	threshold time.Duration
}

func newRelayReconnectBackoff(threshold time.Duration) relayReconnectBackoff {
	if threshold <= 0 {
		threshold = relayStableThreshold
	}
	return relayReconnectBackoff{
		current:   minRelayReconnectBackoff,
		min:       minRelayReconnectBackoff,
		max:       maxRelayReconnectBackoff,
		threshold: threshold,
	}
}

func (b *relayReconnectBackoff) observeReady(uptime time.Duration) {
	if uptime >= b.threshold {
		b.current = b.min
	}
}

func (b *relayReconnectBackoff) nextDelay(err error, jitter byte) time.Duration {
	base := b.current
	b.current = min(b.current*2, b.max)
	delay := base + time.Duration(jitter)*base/512
	var remote protocol.Error
	if errors.As(err, &remote) && remote.RetryAfterSeconds > 0 {
		retryAfter := min(time.Duration(remote.RetryAfterSeconds)*time.Second, 24*time.Hour)
		if delay < retryAfter {
			span := min(retryAfter, b.max)
			delay = retryAfter + time.Duration(jitter)*span/512
		}
	}
	return delay
}

func (rc *relayController) clock() time.Time {
	if rc.pairingNow != nil {
		return rc.pairingNow()
	}
	return time.Now()
}

func (rc *relayController) waitReconnect(ctx context.Context, backoff *relayReconnectBackoff, err error) bool {
	var jitter [1]byte
	if _, randErr := rand.Read(jitter[:]); randErr != nil {
		return false
	}
	delay := backoff.nextDelay(err, jitter[0])
	if rc.reconnectWait != nil {
		return rc.reconnectWait(ctx, delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
