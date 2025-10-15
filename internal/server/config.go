package server

import "time"

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

	// Features
	MetricsEnabled bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Host:           "localhost",
		Port:           8080,
		PathPrefix:     "/api/v1",
		CORSEnabled:    false,
		CORSOrigins:    []string{},
		AuthEnabled:    false,
		AuthHeader:     "X-API-Key",
		RateLimit:      100,
		CacheTTL:       5 * time.Minute,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    120 * time.Second,
		MetricsEnabled: true,
	}
}
