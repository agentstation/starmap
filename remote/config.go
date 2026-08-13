// Package remote provides a reactive Starmap catalog consumer.
package remote

import (
	"net/http"
	"reflect"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
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
	// AfterFailures sets how many consecutive stream open, read, or catch-up
	// failures can occur before the subscriber runs fallback polling.
	AfterFailures int
	// Interval is the minimum time between fallback manifest polls.
	Interval time.Duration
}

// Config defines one remote Starmap catalog source. BaseURL is the versioned
// API root, for example https://starmap.example.com/api/v1.
type Config struct {
	// BaseURL is the trusted absolute HTTPS versioned Starmap API root.
	// Only a loopback publisher can use plain HTTP.
	BaseURL string
	// HTTPClient supplies transport, TLS, authentication, and fetch timeout
	// policy. If nil, Starmap creates a private client with bounded timeouts.
	HTTPClient *http.Client
	// CatalogStore holds verified generations in durable storage. The caller
	// must supply it and owns its resources and lifecycle.
	CatalogStore storage.Store
	// PinnedBootstrap supplies an optional verified offline generation.
	// NewContext commits it only when CatalogStore has no current generation.
	PinnedBootstrap *catalogs.Generation
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
	if isNilStore(c.CatalogStore) {
		return Config{}, &errors.ConfigError{
			Component: "catalog store",
			Message:   "an explicit caller-owned store is required",
		}
	}
	if c.PinnedBootstrap != nil {
		pinned := c.PinnedBootstrap.Copy()
		if err := pinned.Validate(); err != nil {
			return Config{}, errors.WrapResource(
				"validate",
				"pinned bootstrap generation",
				pinned.Manifest.GenerationID,
				err,
			)
		}
		c.PinnedBootstrap = &pinned
	}
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

func isNilStore(store storage.Store) bool {
	if store == nil {
		return true
	}
	value := reflect.ValueOf(store)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
