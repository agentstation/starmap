// Package server provides an embeddable HTTP server for a Starmap catalog.
//
// Alongside Starmap's native catalog and reactive-generation routes, the
// server exposes OpenRouter-compatible model discovery at
// /api/v1/model/{author}/{slug} and
// /api/v1/models/{author}/{slug}/endpoints. Those responses are server-local
// projections over the same immutable catalog; they do not create a second
// persisted catalog or make generated endpoints.yaml authoritative.
package server

import (
	"path"
	"strings"
	"time"

	internalserver "github.com/agentstation/starmap/internal/server"
	"github.com/agentstation/starmap/pkg/errors"
)

// Config configures an embeddable Starmap HTTP server.
type Config struct {
	// Host and Port form the informational HTTP server address. Serve uses the
	// caller-provided listener.
	Host string
	Port int

	// PathPrefix is the root for versioned API routes.
	PathPrefix string

	// CORSEnabled controls CORS middleware. CORSOrigins is the allowlist; an
	// empty allowlist permits every origin when CORS is enabled.
	CORSEnabled bool
	CORSOrigins []string

	// AuthEnabled controls API-key middleware. AuthHeader names the request
	// header carrying the key.
	AuthEnabled bool
	AuthHeader  string

	// RateLimit is the per-IP requests-per-minute limit; zero disables it.
	RateLimit int
	// CacheTTL bounds derived response-cache entries.
	CacheTTL time.Duration

	// ReadTimeout, WriteTimeout, and IdleTimeout configure net/http. Zero
	// delegates the corresponding timeout policy to the caller/network.
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// SSEHeartbeatInterval controls flushed comment heartbeats on publication
	// streams. SSEWriteTimeout bounds each event or heartbeat write and flush.
	SSEHeartbeatInterval time.Duration
	SSEWriteTimeout      time.Duration

	// ShutdownGracePeriod bounds internal service cleanup after HTTP draining.
	ShutdownGracePeriod time.Duration

	// MetricsEnabled exposes the process metrics endpoint.
	MetricsEnabled bool
}

// DefaultConfig returns production-oriented server defaults.
func DefaultConfig() Config {
	internal := internalserver.DefaultConfig()
	return configFromInternal(internal)
}

func (c Config) normalized() Config {
	defaults := DefaultConfig()
	if c.Host == "" {
		c.Host = defaults.Host
	}
	if c.Port == 0 {
		c.Port = defaults.Port
	}
	if c.PathPrefix == "" {
		c.PathPrefix = defaults.PathPrefix
	}
	if c.AuthHeader == "" {
		c.AuthHeader = defaults.AuthHeader
	}
	if c.CacheTTL == 0 {
		c.CacheTTL = defaults.CacheTTL
	}
	if c.ShutdownGracePeriod == 0 {
		c.ShutdownGracePeriod = defaults.ShutdownGracePeriod
	}
	if c.SSEHeartbeatInterval == 0 {
		c.SSEHeartbeatInterval = defaults.SSEHeartbeatInterval
	}
	if c.SSEWriteTimeout == 0 {
		c.SSEWriteTimeout = defaults.SSEWriteTimeout
	}
	c.CORSOrigins = append([]string(nil), c.CORSOrigins...)
	return c
}

func (c Config) validate() error {
	switch {
	case c.Port < 1 || c.Port > 65535:
		return &errors.ValidationError{Field: "server.port", Value: c.Port, Message: "must be between 1 and 65535"}
	case !strings.HasPrefix(c.PathPrefix, "/"):
		return &errors.ValidationError{Field: "server.path_prefix", Value: c.PathPrefix, Message: "must start with /"}
	case c.PathPrefix == "/" || strings.HasSuffix(c.PathPrefix, "/"):
		return &errors.ValidationError{Field: "server.path_prefix", Value: c.PathPrefix, Message: "must identify a non-root path without a trailing slash"}
	case strings.ContainsAny(c.PathPrefix, "{}?# \t\r\n"):
		return &errors.ValidationError{Field: "server.path_prefix", Value: c.PathPrefix, Message: "must be a static URL path"}
	case path.Clean(c.PathPrefix) != c.PathPrefix:
		return &errors.ValidationError{Field: "server.path_prefix", Value: c.PathPrefix, Message: "must be a clean URL path"}
	case c.RateLimit < 0:
		return &errors.ValidationError{Field: "server.rate_limit", Value: c.RateLimit, Message: "cannot be negative"}
	case c.CacheTTL <= 0:
		return &errors.ValidationError{Field: "server.cache_ttl", Value: c.CacheTTL, Message: "must be positive"}
	case c.ReadTimeout < 0:
		return &errors.ValidationError{Field: "server.read_timeout", Value: c.ReadTimeout, Message: "cannot be negative"}
	case c.WriteTimeout < 0:
		return &errors.ValidationError{Field: "server.write_timeout", Value: c.WriteTimeout, Message: "cannot be negative"}
	case c.IdleTimeout < 0:
		return &errors.ValidationError{Field: "server.idle_timeout", Value: c.IdleTimeout, Message: "cannot be negative"}
	case c.SSEHeartbeatInterval <= 0:
		return &errors.ValidationError{Field: "server.sse_heartbeat_interval", Value: c.SSEHeartbeatInterval, Message: "must be positive"}
	case c.SSEWriteTimeout <= 0:
		return &errors.ValidationError{Field: "server.sse_write_timeout", Value: c.SSEWriteTimeout, Message: "must be positive"}
	case c.ShutdownGracePeriod <= 0:
		return &errors.ValidationError{Field: "server.shutdown_grace_period", Value: c.ShutdownGracePeriod, Message: "must be positive"}
	default:
		return nil
	}
}

func (c Config) internal() internalserver.Config {
	return internalserver.Config{
		Host: c.Host, Port: c.Port, PathPrefix: c.PathPrefix,
		CORSEnabled: c.CORSEnabled, CORSOrigins: append([]string(nil), c.CORSOrigins...),
		AuthEnabled: c.AuthEnabled, AuthHeader: c.AuthHeader,
		RateLimit: c.RateLimit, CacheTTL: c.CacheTTL,
		ReadTimeout: c.ReadTimeout, WriteTimeout: c.WriteTimeout, IdleTimeout: c.IdleTimeout,
		SSEHeartbeatInterval: c.SSEHeartbeatInterval, SSEWriteTimeout: c.SSEWriteTimeout,
		ShutdownGracePeriod: c.ShutdownGracePeriod,
		MetricsEnabled:      c.MetricsEnabled,
	}
}

func configFromInternal(c internalserver.Config) Config {
	return Config{
		Host: c.Host, Port: c.Port, PathPrefix: c.PathPrefix,
		CORSEnabled: c.CORSEnabled, CORSOrigins: append([]string(nil), c.CORSOrigins...),
		AuthEnabled: c.AuthEnabled, AuthHeader: c.AuthHeader,
		RateLimit: c.RateLimit, CacheTTL: c.CacheTTL,
		ReadTimeout: c.ReadTimeout, WriteTimeout: c.WriteTimeout, IdleTimeout: c.IdleTimeout,
		SSEHeartbeatInterval: c.SSEHeartbeatInterval, SSEWriteTimeout: c.SSEWriteTimeout,
		ShutdownGracePeriod: c.ShutdownGracePeriod,
		MetricsEnabled:      c.MetricsEnabled,
	}
}
