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
)

// Config defines one remote Starmap catalog source. BaseURL is the versioned
// API root, for example https://starmap.example.com/api/v1.
type Config struct {
	// BaseURL is the absolute HTTP(S) versioned Starmap API root.
	BaseURL string
	// HTTPClient supplies transport, TLS, authentication, and fetch timeout
	// policy. A private bounded client is used when nil.
	HTTPClient *http.Client
	// ReconnectMinDelay is the first reconnect delay. Zero selects the default.
	ReconnectMinDelay time.Duration
	// ReconnectMaxDelay bounds exponential reconnect delay. Zero selects the
	// default.
	ReconnectMaxDelay time.Duration
}

func (c Config) normalized() (Config, error) {
	if c.ReconnectMinDelay == 0 {
		c.ReconnectMinDelay = DefaultReconnectMinDelay
	}
	if c.ReconnectMaxDelay == 0 {
		c.ReconnectMaxDelay = DefaultReconnectMaxDelay
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
	return c, nil
}
