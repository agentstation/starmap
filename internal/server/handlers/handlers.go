// Package handlers provides HTTP request handlers for the Starmap API.
package handlers

import (
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/cmd/application"
	"github.com/agentstation/starmap/internal/server/cache"
	"github.com/agentstation/starmap/internal/server/sse"
	ws "github.com/agentstation/starmap/internal/server/websocket"
)

// Handlers provides access to all HTTP handlers.
type Handlers struct {
	app            application.Application
	cache          *cache.Cache
	wsHub          *ws.Hub
	sseBroadcaster *sse.Broadcaster
	upgrader       websocket.Upgrader
	logger         *zerolog.Logger
	broadcastFn    func(string, any)
}

// New creates a new Handlers instance.
func New(
	app application.Application,
	cache *cache.Cache,
	wsHub *ws.Hub,
	sseBroadcaster *sse.Broadcaster,
	upgrader websocket.Upgrader,
	logger *zerolog.Logger,
	broadcastFn func(string, any),
) *Handlers {
	return &Handlers{
		app:            app,
		cache:          cache,
		wsHub:          wsHub,
		sseBroadcaster: sseBroadcaster,
		upgrader:       upgrader,
		logger:         logger,
		broadcastFn:    broadcastFn,
	}
}
