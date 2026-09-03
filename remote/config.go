// Package remote provides a reactive Starmap catalog consumer.
package remote

import (
	"crypto/tls"
	"net/http"
	"reflect"
	"time"

	"github.com/agentstation/starmap/internal/fleet"
	"github.com/agentstation/starmap/pkg/catalogs"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// DefaultReconnectMinDelay is the first reconnect delay. It matches the
	// fleet minimum retry delay, so one subscriber paces like every other
	// automatic Starmap worker.
	DefaultReconnectMinDelay = fleet.MinRetryDelay
	// DefaultReconnectMaxDelay bounds reconnect delay growth. It matches the
	// fleet maximum retry delay.
	DefaultReconnectMaxDelay = fleet.MaxRetryDelay
	// DefaultExpectedHeartbeatInterval matches the server's default heartbeat.
	DefaultExpectedHeartbeatInterval = 20 * time.Second
	// DefaultLivenessTimeout bounds a stream with no heartbeat or event.
	DefaultLivenessTimeout = 60 * time.Second
	// DefaultShutdownTimeout bounds Close while joining owned loops.
	DefaultShutdownTimeout = 5 * time.Second
	// DefaultStartupSpread is the admission window of an initial or
	// post-outage reconnect. Spreading needs a configured Identity.
	DefaultStartupSpread = fleet.DefaultStartupSpread
	// DefaultFallbackPollInterval is the fallback poll interval that a
	// composed Starmap source selects. A subscriber policy stays explicit, so
	// the caller always states the interval it wants.
	DefaultFallbackPollInterval = 15 * time.Minute

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
	// TransferPolicy bounds each stage of one transfer and bounds the wait for
	// the event-stream response headers. It applies only when HTTPClient is
	// nil, because a supplied client already owns its transport. Nil selects
	// the shared default policy.
	TransferPolicy *protocol.TransferPolicy
	// TLSConfig pins the TLS origin policy of the private transport, such as a
	// minimum version or a private root pool. It applies only when HTTPClient
	// is nil. Nil selects the platform policy.
	TLSConfig *tls.Config
	// Identity names this subscriber inside a fleet. An empty instance
	// identity disables startup spreading and stable-phase polling, so a
	// single process reconnects and polls at once.
	Identity fleet.Identity
	// StartupSpread is the admission window of an initial or post-outage
	// reconnect. Zero selects DefaultStartupSpread, and a negative value
	// admits every reconnect at once.
	StartupSpread time.Duration
	// HealthyWindow is how long a stream must stay open before the subscriber
	// resets its reconnect backoff. Zero selects LivenessTimeout, because a
	// stream that outlives one liveness window proved its liveness.
	HealthyWindow time.Duration
	// Random supplies the jitter of the reconnect delay and of a refusal
	// boundary. Nil selects the system source.
	Random fleet.Random
	// CredentialChanges reports that the caller replaced the credential of
	// HTTPClient. An authentication failure then waits for a change instead of
	// stopping the subscriber. Nil keeps an authentication failure terminal.
	CredentialChanges <-chan struct{}
}

// random returns the configured jitter source, or the system source.
func (c Config) random() fleet.Random {
	if c.Random != nil {
		return c.Random
	}
	return fleet.SystemRandom()
}

// controllerIdentity returns the fleet identity of one subscriber controller.
func (c Config) controllerIdentity(controller string) fleet.Identity {
	identity := c.Identity
	identity.Controller = controller
	return identity
}

// transferClient builds the private transport of a subscriber that supplied no
// HTTP client. The transfer policy bounds the wait for response headers, which
// is the only bound that applies to opening an event stream.
func (c Config) transferClient() (*http.Client, error) {
	policy := protocol.DefaultTransferPolicy()
	if c.TransferPolicy != nil {
		policy = *c.TransferPolicy
	}
	transport, err := protocol.NewTransport(policy)
	if err != nil {
		return nil, err
	}
	if c.TLSConfig != nil {
		transport.TLSClientConfig = c.TLSConfig.Clone()
	}
	return &http.Client{Transport: transport}, nil
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
	if c.StartupSpread == 0 {
		c.StartupSpread = DefaultStartupSpread
	}
	if c.HealthyWindow == 0 {
		c.HealthyWindow = c.LivenessTimeout
	}
	if c.HealthyWindow < 0 {
		return Config{}, &errors.ValidationError{
			Field:   "remote.healthy_window",
			Value:   c.HealthyWindow,
			Message: "must be positive",
		}
	}
	if c.TransferPolicy != nil {
		policy := *c.TransferPolicy
		if err := policy.Validate(); err != nil {
			return Config{}, err
		}
		c.TransferPolicy = &policy
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
