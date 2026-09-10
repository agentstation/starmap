package permission

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const partsPerMillion = 1_000_000

// ClockCacheConfig selects qualified observations and a process-local elapsed counter.
// The host qualifies the observation source, counter, and error bounds on its platform.
type ClockCacheConfig struct {
	// Observe reads UTC with a known error bound. Its timestamp falls within the call.
	// It stops when the context ends and changes no clock or time-service setting.
	Observe func(context.Context) (ClockReading, error)
	// Elapsed reads a nonnegative counter that includes system sleep, with no I/O.
	// It supports concurrent calls and reports false after an unqualified discontinuity.
	Elapsed func() (time.Duration, bool)
	// MaxAge bounds observation age, including query delay, to at most five minutes.
	MaxAge time.Duration
	// MaxDriftPPM bounds counter rate error relative to actual elapsed time.
	// It must be positive and less than one million parts per million.
	MaxDriftPPM uint32
	// CounterUncertainty bounds the absolute error of each counter reading.
	CounterUncertainty time.Duration
}

// ClockCache ages qualified UTC evidence without consulting a time service during reads.
// NewClockCache and Read never call Observe. The host schedules explicit Refresh calls.
// Permission issuers and connected runtimes can use Read as their complete clock callback.
type ClockCache struct {
	config  ClockCacheConfig
	refresh chan struct{}
	current atomic.Pointer[clockCacheState]
}

type clockCacheState struct {
	reading ClockReading
	before  time.Duration
	after   time.Duration
}

// NewClockCache constructs an unqualified cache without calling either clock function.
func NewClockCache(config ClockCacheConfig) (*ClockCache, error) {
	if config.Observe == nil || config.Elapsed == nil {
		return nil, clockCacheError("observation and elapsed-counter functions are required")
	}
	if config.MaxAge <= 0 || config.MaxAge > catalogs.MaxCatalogPermissionValidity {
		return nil, clockCacheError("maximum age must be positive and within five minutes")
	}
	if config.MaxDriftPPM == 0 || config.MaxDriftPPM >= partsPerMillion {
		return nil, clockCacheError("counter drift bound must be positive and below one million parts per million")
	}
	if config.CounterUncertainty < 0 || config.CounterUncertainty > catalogs.MaxCatalogPermissionClockUncertainty {
		return nil, clockCacheError("counter uncertainty is outside the supported permission bound")
	}
	cache := &ClockCache{config: config, refresh: make(chan struct{}, 1)}
	cache.current.Store(&clockCacheState{})
	return cache, nil
}

// Refresh replaces clock evidence after one bounded observation.
// Failed or invalid observations clear the previous evidence. Concurrent refreshes run in order.
// Invalidate or an unsafe counter read during observation prevents its publication.
func (c *ClockCache) Refresh(ctx context.Context) error {
	if c == nil || ctx == nil || c.config.Observe == nil || c.config.Elapsed == nil {
		return clockCacheError("a constructed cache and context are required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case c.refresh <- struct{}{}:
		defer func() { <-c.refresh }()
	case <-ctx.Done():
		return ctx.Err()
	}
	previous := c.current.Load()
	next, err := c.observe(ctx)
	if err != nil {
		c.current.CompareAndSwap(previous, &clockCacheState{})
		return err
	}
	if !c.current.CompareAndSwap(previous, next) {
		return &errors.ConflictError{Resource: "permission clock", Message: "clock evidence changed during observation"}
	}
	return nil
}

func (c *ClockCache) observe(ctx context.Context) (*clockCacheState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	before, known := c.config.Elapsed()
	if !known || before < 0 {
		return nil, clockCacheError("elapsed counter is unqualified")
	}
	observationContext, cancel := context.WithTimeout(ctx, c.config.MaxAge)
	defer cancel()
	reading, err := c.config.Observe(observationContext)
	if err != nil {
		return nil, errors.WrapResource("observe", "permission clock", "", err)
	}
	if err := observationContext.Err(); err != nil {
		return nil, err
	}
	after, known := c.config.Elapsed()
	if !known || after < before || !reading.valid() {
		return nil, clockCacheError("clock observation or elapsed interval is unqualified")
	}
	// UTC removes process monotonic data before the elapsed counter advances this time.
	state := &clockCacheState{reading: reading, before: before, after: after}
	state.reading.Time = reading.Time.UTC()
	if !c.readAt(state, after).Known {
		return nil, clockCacheError("observation delay exhausted the age or uncertainty bound")
	}
	return state, nil
}

// Read returns one cached UTC estimate and its complete uncertainty bound.
// Expiry, counter failure, or regression clears that evidence until a successful refresh.
func (c *ClockCache) Read() ClockReading {
	if c == nil || c.config.Elapsed == nil {
		return ClockReading{}
	}
	state := c.current.Load()
	if state == nil || !state.reading.Known {
		return ClockReading{}
	}
	now, known := c.config.Elapsed()
	if known {
		if reading := c.readAt(state, now); reading.Known {
			return reading
		}
	}
	c.current.CompareAndSwap(state, &clockCacheState{})
	return ClockReading{}
}

func (c *ClockCache) readAt(state *clockCacheState, now time.Duration) ClockReading {
	if now < state.after {
		return ClockReading{}
	}
	age := now - state.before
	if age >= c.config.MaxAge {
		return ClockReading{}
	}
	// Rate error bounds actual duration by measured duration divided by (1 - rate).
	// MaxAge bounds multiplication below int64 overflow, even near the rate limit.
	counterInterval := age + 2*c.config.CounterUncertainty
	numerator := int64(counterInterval) * int64(c.config.MaxDriftPPM)
	denominator := int64(partsPerMillion - c.config.MaxDriftPPM)
	drift := time.Duration((numerator + denominator - 1) / denominator)
	if counterInterval+drift >= c.config.MaxAge {
		return ClockReading{}
	}
	uncertainty := state.reading.Uncertainty + (state.after - state.before) + 2*c.config.CounterUncertainty + drift
	if uncertainty > catalogs.MaxCatalogPermissionClockUncertainty {
		return ClockReading{}
	}
	return ClockReading{Time: state.reading.Time.Add(now - state.after), Uncertainty: uncertainty, Known: true}
}

// Invalidate clears clock evidence and prevents an earlier observation from restoring it.
// The host calls this method when native clock evidence becomes unqualified.
func (c *ClockCache) Invalidate() {
	if c != nil {
		c.current.Store(&clockCacheState{})
	}
}

func clockCacheError(message string) error {
	return &errors.ValidationError{Field: "permission_clock", Message: message}
}
