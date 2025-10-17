// Package handlers provides HTTP request handlers for the Starmap API.
package handlers

import (
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/internal/cmd/application"
	"github.com/agentstation/starmap/internal/server/cache"
	"github.com/agentstation/starmap/internal/server/events"
	"github.com/agentstation/starmap/internal/server/sse"
	ws "github.com/agentstation/starmap/internal/server/websocket"
)

// Handlers provides access to all HTTP handlers.
type Handlers struct {
	app            application.Application
	cache          *cache.Cache
	broker         *events.Broker
	wsHub          *ws.Hub
	sseBroadcaster *sse.Broadcaster
	upgrader       websocket.Upgrader
	logger         *zerolog.Logger
}

// New creates a new Handlers instance.
func New(
	app application.Application,
	cache *cache.Cache,
	broker *events.Broker,
	wsHub *ws.Hub,
	sseBroadcaster *sse.Broadcaster,
	upgrader websocket.Upgrader,
	logger *zerolog.Logger,
) *Handlers {
	return &Handlers{
		app:            app,
		cache:          cache,
		broker:         broker,
		wsHub:          wsHub,
		sseBroadcaster: sseBroadcaster,
		upgrader:       upgrader,
		logger:         logger,
	}
}
