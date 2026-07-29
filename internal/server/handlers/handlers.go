// Package handlers provides HTTP request handlers for the Starmap API.
package handlers

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/internal/server/cache"
	"github.com/agentstation/starmap/internal/server/sse"
)

// Handlers provides access to all HTTP handlers.
type Handlers struct {
	app            application
	cache          *cache.Cache
	sseBroadcaster *sse.Broadcaster
	logger         *zerolog.Logger
	startTime      time.Time
}

// New creates a new Handlers instance.
func New(
	app application,
	cache *cache.Cache,
	sseBroadcaster *sse.Broadcaster,
	logger *zerolog.Logger,
	startTime time.Time,
) *Handlers {
	return &Handlers{
		app:            app,
		cache:          cache,
		sseBroadcaster: sseBroadcaster,
		logger:         logger,
		startTime:      startTime,
	}
}
