// Package handlers provides HTTP request handlers for the Starmap API.
package handlers

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/internal/server/cache"
	"github.com/agentstation/starmap/internal/server/operations"
	"github.com/agentstation/starmap/internal/server/sse"
)

// Handlers provides access to all HTTP handlers.
type Handlers struct {
	app            application
	cache          *cache.Cache
	sseBroadcaster *sse.Broadcaster
	operations     *operations.Registry
	logger         *zerolog.Logger
	startTime      time.Time
}

// log returns the handler logger. A handler without a configured logger
// discards its output, so a test needs no logger.
func (h *Handlers) log() *zerolog.Logger {
	if h.logger != nil {
		return h.logger
	}
	discard := zerolog.Nop()
	return &discard
}

// New creates a new Handlers instance.
func New(
	app application,
	cache *cache.Cache,
	sseBroadcaster *sse.Broadcaster,
	operationRegistry *operations.Registry,
	logger *zerolog.Logger,
	startTime time.Time,
) *Handlers {
	return &Handlers{
		app:            app,
		cache:          cache,
		sseBroadcaster: sseBroadcaster,
		operations:     operationRegistry,
		logger:         logger,
		startTime:      startTime,
	}
}
