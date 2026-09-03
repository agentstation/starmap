package server

import (
	"time"

	"github.com/agentstation/starmap/internal/server/sse"
)

// Config holds server configuration.
type Config struct {
	// Server settings
	Host string
	Port int

	// API settings
	PathPrefix string

	// CORS settings
	CORSEnabled bool
	CORSOrigins []string

	// Authentication settings
	AuthEnabled bool
	AuthHeader  string

	// Performance settings
	RateLimit int // Requests per minute per IP (0 to disable)
	CacheTTL  time.Duration

	// HTTP timeouts
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	// SSEHeartbeatInterval keeps otherwise-idle publication streams alive.
	SSEHeartbeatInterval time.Duration
	// SSEWriteTimeout bounds each publication or heartbeat write and flush.
	SSEWriteTimeout time.Duration

	// Shutdown settings
	ShutdownGracePeriod time.Duration // Time to wait for background services to shutdown gracefully

	// Features
	MetricsEnabled bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Host:                 "localhost",
		Port:                 8080,
		PathPrefix:           "/api/v1",
		CORSEnabled:          false,
		CORSOrigins:          []string{},
		AuthEnabled:          false,
		AuthHeader:           "X-API-Key",
		RateLimit:            100,
		CacheTTL:             5 * time.Minute,
		ReadTimeout:          10 * time.Second,
		WriteTimeout:         10 * time.Second,
		IdleTimeout:          120 * time.Second,
		SSEHeartbeatInterval: sse.DefaultHeartbeatInterval,
		SSEWriteTimeout:      sse.DefaultWriteTimeout,
		ShutdownGracePeriod:  100 * time.Millisecond,
		MetricsEnabled:       true,
	}
}
