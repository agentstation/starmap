package remote

import (
	"time"

	"github.com/agentstation/starmap/internal/fleet"
)

// Fleet controller names. A stable phase and a startup offset separate the two
// controllers of one subscriber, so the reconnect spread and the fallback poll
// phase never land on one instant.
const (
	controllerStream = "source_stream"
	controllerPoll   = "source_poll"
)

// reconnectState paces stream reconnects for one subscriber.
//
// It holds three rules. A delay grows with decorrelated jitter and never
// exceeds the configured maximum. A backoff resets only after a stream stayed
// open for a healthy liveness window, never after a TCP connection or the
// first response header. A declared refusal boundary is a hard not-before that
// replaces the computed delay.
type reconnectState struct {
	policy   fleet.RetryPolicy
	random   fleet.Random
	identity fleet.Identity
	window   time.Duration
	spread   time.Duration

	// delay is the previous reconnect delay. A reset returns it to zero, and
	// the next delay then starts at the minimum again.
	delay time.Duration

	// openedAt is when the current stream opened. A zero value means no stream
	// is open.
	openedAt time.Time

	// notBefore is the hard boundary a refusal declared.
	notBefore time.Time

	// admitted reports whether this reconnect burst already spent its startup
	// offset. An outage clears it, so a fleet spreads its return.
	admitted bool
}

// newReconnectState builds the pacing state of one subscriber.
func newReconnectState(config Config) reconnectState {
	return reconnectState{
		policy: fleet.RetryPolicy{
			MinDelay:    config.ReconnectMinDelay,
			MaxDelay:    config.ReconnectMaxDelay,
			MaxAttempts: fleet.MaxTransientRetries,
		},
		random:   config.random(),
		identity: config.controllerIdentity(controllerStream),
		window:   config.HealthyWindow,
		spread:   config.StartupSpread,
	}
}

// opened records that a stream opened. It never resets the backoff, because a
// TCP connection and a first response header prove no liveness.
func (r *reconnectState) opened(now time.Time) {
	r.openedAt = now
}

// closed records that the open stream ended. The backoff resets only when the
// stream stayed open for the healthy liveness window. A healthy stream that
// ends is an outage, so the next reconnect spreads across the startup window.
func (r *reconnectState) closed(now time.Time) bool {
	opened := r.openedAt
	r.openedAt = time.Time{}
	if opened.IsZero() || now.Sub(opened) < r.window {
		return false
	}
	r.delay = 0
	r.admitted = false
	return true
}

// refuse records the hard not-before boundary of one refusal. The boundary
// carries jitter, so a refused fleet does not return at one instant.
func (r *reconnectState) refuse(now, boundary time.Time) {
	r.notBefore = fleet.NotBefore(now, boundary, r.random)
}

// next returns the delay before the next reconnect attempt. A declared
// boundary wins over the computed delay, and the first attempt of a burst adds
// the stable startup offset of this instance. A subscriber with no instance
// identity belongs to no fleet, so it reconnects with no spread.
func (r *reconnectState) next(now time.Time) (time.Duration, error) {
	delay := time.Duration(0)
	if boundary := r.notBefore; !boundary.IsZero() {
		r.notBefore = time.Time{}
		if wait := boundary.Sub(now); wait > 0 {
			delay = wait
		}
	}
	if delay == 0 {
		next, err := r.policy.Next(r.delay, r.random)
		if err != nil {
			return 0, err
		}
		r.delay = next
		delay = next
	}
	if !r.admitted && r.identity.Instance != "" {
		r.admitted = true
		offset, err := fleet.StartupOffset(r.identity, r.spread)
		if err != nil {
			return 0, err
		}
		delay += offset
	}
	return delay, nil
}

// boundary returns the current hard not-before boundary. Health reports it, so
// an operator sees why a subscriber waits.
func (r *reconnectState) boundary() time.Time { return r.notBefore }

// fallbackPollAt returns the next fallback poll instant. A configured instance
// identity selects the stable phase of the interval, so many subscribers poll
// across the whole interval instead of together. Without an identity the first
// poll runs at once and later polls follow the interval.
func fallbackPollAt(
	identity fleet.Identity,
	interval time.Duration,
	now time.Time,
) (time.Time, error) {
	if identity.Instance == "" || interval <= 0 {
		return now, nil
	}
	phase, err := fleet.StablePhase(identity, interval)
	if err != nil {
		return time.Time{}, err
	}
	at := now.Truncate(interval).Add(phase)
	if !at.After(now) {
		at = at.Add(interval)
	}
	return at, nil
}
