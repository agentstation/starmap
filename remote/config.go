// Package remote provides a reactive Starmap catalog consumer.
package remote

import (
	"net/http"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// DefaultReconnectMinDelay is the first reconnect delay.
	DefaultReconnectMinDelay = 100 * time.Millisecond
	// DefaultReconnectMaxDelay bounds reconnect delay growth.
	DefaultReconnectMaxDelay = 5 * time.Second
	// DefaultExpectedHeartbeatInterval matches the server's default heartbeat.
	DefaultExpectedHeartbeatInterval = 20 * time.Second
	// DefaultLivenessTimeout bounds a stream with no heartbeat or event.
	DefaultLivenessTimeout = 60 * time.Second
	// DefaultShutdownTimeout bounds Close while joining owned loops.
	DefaultShutdownTimeout = 5 * time.Second

	minimumLivenessIntervals = 2
)

// PollingFallbackPolicy explicitly enables bounded conditional polling after
// repeated streaming failures. Polling remains disabled when this policy is
// nil.
type PollingFallbackPolicy struct {
	// AfterFailures is the number of consecutive stream open, read, or catch-up
	// failures required before fallback polling begins.
	AfterFailures int
	// Interval is the minimum time between fallback manifest polls.
	Interval time.Duration
}

// Config defines one remote Starmap catalog source. BaseURL is the versioned
// API root, for example https://starmap.example.com/api/v1.
type Config struct {
	// BaseURL is the trusted absolute HTTPS versioned Starmap API root.
	// Plain HTTP is accepted only for a loopback publisher.
	BaseURL string
	// HTTPClient supplies transport, TLS, authentication, and fetch timeout
	// policy. A private bounded client is used when nil.
	HTTPClient *http.Client
	// ReconnectMinDelay is the first reconnect delay. Zero selects the default.
	ReconnectMinDelay time.Duration
	// ReconnectMaxDelay bounds exponential reconnect delay. Zero selects the
	// default.
	ReconnectMaxDelay time.Duration
	// ExpectedHeartbeatInterval is the configured server heartbeat interval.
	// Zero selects the server's default.
	ExpectedHeartbeatInterval time.Duration
	// LivenessTimeout is the maximum time without a comment or publication
	// frame. Zero selects the default.
	LivenessTimeout time.Duration
	// ShutdownTimeout bounds Close while it joins subscriber-owned loops. Zero
	// selects the default.
	ShutdownTimeout time.Duration
	// PollingFallback explicitly enables bounded conditional polling after
	// repeated streaming failures. Nil keeps polling disabled.
	PollingFallback *PollingFallbackPolicy
}

func (c Config) normalized() (Config, error) {
	if c.ReconnectMinDelay == 0 {
		c.ReconnectMinDelay = DefaultReconnectMinDelay
	}
	if c.ReconnectMaxDelay == 0 {
		c.ReconnectMaxDelay = DefaultReconnectMaxDelay
	}
	if c.ExpectedHeartbeatInterval == 0 {
		c.ExpectedHeartbeatInterval = DefaultExpectedHeartbeatInterval
	}
	if c.LivenessTimeout == 0 {
		c.LivenessTimeout = DefaultLivenessTimeout
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = DefaultShutdownTimeout
	}
	if c.ReconnectMinDelay < 0 {
		return Config{}, &errors.ValidationError{
			Field:   "remote.reconnect_min_delay",
			Value:   c.ReconnectMinDelay,
			Message: "must be positive",
		}
	}
	if c.ReconnectMaxDelay < 0 {
		return Config{}, &errors.ValidationError{
			Field:   "remote.reconnect_max_delay",
			Value:   c.ReconnectMaxDelay,
			Message: "must be positive",
		}
	}
	if c.ReconnectMaxDelay < c.ReconnectMinDelay {
		return Config{}, &errors.ValidationError{
			Field:   "remote.reconnect_max_delay",
			Value:   c.ReconnectMaxDelay,
			Message: "must be at least reconnect_min_delay",
		}
	}
	if c.ExpectedHeartbeatInterval < 0 {
		return Config{}, &errors.ValidationError{
			Field:   "remote.expected_heartbeat_interval",
			Value:   c.ExpectedHeartbeatInterval,
			Message: "must be positive",
		}
	}
	if c.LivenessTimeout < 0 {
		return Config{}, &errors.ValidationError{
			Field:   "remote.liveness_timeout",
			Value:   c.LivenessTimeout,
			Message: "must be positive",
		}
	}
	minimumLiveness := minimumLivenessIntervals * c.ExpectedHeartbeatInterval
	if c.LivenessTimeout < minimumLiveness {
		return Config{}, &errors.ValidationError{
			Field:   "remote.liveness_timeout",
			Value:   c.LivenessTimeout,
			Message: "must allow at least two expected heartbeat intervals",
		}
	}
	if c.ShutdownTimeout < 0 {
		return Config{}, &errors.ValidationError{
			Field:   "remote.shutdown_timeout",
			Value:   c.ShutdownTimeout,
			Message: "must be positive",
		}
	}
	if c.PollingFallback != nil {
		policy := *c.PollingFallback
		c.PollingFallback = &policy
		if policy.AfterFailures < 1 {
			return Config{}, &errors.ValidationError{
				Field:   "remote.polling_fallback.after_failures",
				Value:   policy.AfterFailures,
				Message: "must be positive",
			}
		}
		if policy.Interval <= 0 {
			return Config{}, &errors.ValidationError{
				Field:   "remote.polling_fallback.interval",
				Value:   policy.Interval,
				Message: "must be positive",
			}
		}
	}
	return c, nil
}
