// Package fleet owns the pacing primitives that keep many Starmap instances
// from hitting one source together.
//
// The package holds four pure rules. A stable phase spreads periodic work
// across a full interval. A startup spread admits cold work across a shorter
// window. A decorrelated retry policy bounds transient failure. A hard
// not-before honors a `Retry-After` header or a rate-limit reset time.
//
// Every rule is deterministic for a given identity and random source. The
// runtime and every source share these rules, so one instance keeps its phase
// across a restart and two instances rarely share one.
package fleet

import (
	"hash/fnv"
	"math"
	"math/rand/v2"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// DefaultStartupSpread is the admission window for cold automatic work.
	DefaultStartupSpread = 15 * time.Minute

	// MinRetryDelay is the first transient retry delay.
	MinRetryDelay = time.Second

	// MaxRetryDelay caps the decorrelated retry delay.
	MaxRetryDelay = 15 * time.Minute

	// MaxTransientRetries is the transient retry budget of one cycle. This
	// budget stops one automatic cycle from multiplying requests.
	MaxTransientRetries = 3

	// MaxNotBeforeJitter is the largest delay added after a hard not-before
	// boundary. It stops a fleet from retrying at the same instant.
	MaxNotBeforeJitter = 5 * time.Minute

	// retryGrowth is the decorrelated jitter growth factor.
	retryGrowth = 3
)

// Random returns a value in the half-open range from zero to one. Tests supply
// a deterministic implementation.
type Random func() float64

// SystemRandom returns the default random source.
func SystemRandom() Random {
	return rand.Float64
}

// Identity names the inputs of one stable phase. The instance identity lives
// outside the catalog store. The source identity is a safe name, never a URL,
// a token, or the host of a custom source.
type Identity struct {
	// Instance is the stable identity of this process.
	Instance string

	// Controller names the periodic worker, such as "source" or "acquisition".
	Controller string

	// Source is the safe identity of the configured source.
	Source string
}

// StablePhase returns the offset of one controller inside its interval. The
// offset is `hash(instance + controller + source) mod interval`, so a restart
// on the same host keeps its phase and two hosts rarely share one.
func StablePhase(identity Identity, interval time.Duration) (time.Duration, error) {
	if interval <= 0 {
		return 0, fleetValidation("interval", interval, "must be positive")
	}
	if identity.Instance == "" || identity.Controller == "" {
		return 0, fleetValidation("identity", identity.Controller,
			"needs an instance identity and a controller name")
	}
	digest := fnv.New64a()
	// Separate the fields, so two different field splits cannot collide.
	for _, field := range []string{identity.Instance, identity.Controller, identity.Source} {
		_, _ = digest.Write([]byte(field))
		_, _ = digest.Write([]byte{0})
	}
	// Drop the sign bit, so the hash is a non-negative int64 and the modulo
	// runs in the same signed space as the interval.
	phase := int64(digest.Sum64() & math.MaxInt64)
	return time.Duration(phase % int64(interval)), nil
}

// StartupOffset returns the stable admission offset of cold automatic work
// inside the startup spread. A zero or negative spread admits work at once.
func StartupOffset(identity Identity, spread time.Duration) (time.Duration, error) {
	if spread <= 0 {
		return 0, nil
	}
	return StablePhase(identity, spread)
}

// RetryPolicy paces transient retries with decorrelated jitter.
type RetryPolicy struct {
	// MinDelay is the first delay and the lower bound of every later delay.
	MinDelay time.Duration

	// MaxDelay caps every delay.
	MaxDelay time.Duration

	// MaxAttempts is the transient retry budget of one cycle.
	MaxAttempts int
}

// DefaultRetryPolicy returns the transient retry policy of one cycle.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MinDelay:    MinRetryDelay,
		MaxDelay:    MaxRetryDelay,
		MaxAttempts: MaxTransientRetries,
	}
}

// Validate reports whether the policy is usable.
func (p RetryPolicy) Validate() error {
	if p.MinDelay <= 0 {
		return fleetValidation("retry.min_delay", p.MinDelay, "must be positive")
	}
	if p.MaxDelay < p.MinDelay {
		return fleetValidation("retry.max_delay", p.MaxDelay, "must be at least the minimum delay")
	}
	if p.MaxAttempts <= 0 {
		return fleetValidation("retry.max_attempts", p.MaxAttempts, "must be positive")
	}
	return nil
}

// Allows reports whether one more transient retry stays inside the budget.
// The count is the number of transient retries this cycle already spent.
func (p RetryPolicy) Allows(retries int) bool {
	return retries < p.MaxAttempts
}

// Next returns the delay that follows previous. A zero previous delay returns
// the minimum delay. Later delays draw from the decorrelated jitter range
// between the minimum delay and three times the previous delay.
func (p RetryPolicy) Next(previous time.Duration, random Random) (time.Duration, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}
	if random == nil {
		random = SystemRandom()
	}
	if previous <= 0 {
		return p.MinDelay, nil
	}
	upper := previous * retryGrowth
	if upper > p.MaxDelay || upper < previous {
		upper = p.MaxDelay
	}
	if upper <= p.MinDelay {
		return p.MinDelay, nil
	}
	span := float64(upper - p.MinDelay)
	delay := p.MinDelay + time.Duration(random()*span)
	if delay > p.MaxDelay {
		delay = p.MaxDelay
	}
	return delay, nil
}

// NotBefore returns the earliest time a client may retry after a server
// boundary. The boundary is a hard floor, and the result adds up to
// MaxNotBeforeJitter after it, so a fleet does not retry at one instant.
//
// A boundary at or before now still adds the jitter, because a fleet that
// reads an expired boundary would otherwise retry together.
func NotBefore(now, boundary time.Time, random Random) time.Time {
	if random == nil {
		random = SystemRandom()
	}
	floor := boundary
	if floor.Before(now) {
		floor = now
	}
	return floor.Add(time.Duration(random() * float64(MaxNotBeforeJitter)))
}

func fleetValidation(field string, value any, message string) error {
	return &errors.ValidationError{
		Field:   "fleet." + field,
		Value:   value,
		Message: message,
	}
}
