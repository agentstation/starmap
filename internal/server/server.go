// Package server provides HTTP server implementation for the Starmap API.
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/cmd/application"
	"github.com/agentstation/starmap/internal/server/cache"
	"github.com/agentstation/starmap/internal/server/events"
	"github.com/agentstation/starmap/internal/server/events/adapters"
	"github.com/agentstation/starmap/internal/server/sse"
	ws "github.com/agentstation/starmap/internal/server/websocket"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Server holds the HTTP server state and dependencies.
type Server struct {
	app            application.Application
	cache          *cache.Cache
	broker         *events.Broker
	wsHub          *ws.Hub
	sseBroadcaster *sse.Broadcaster
	upgrader       websocket.Upgrader
	logger         *zerolog.Logger
	config         Config
}

// New creates a new server instance with the given configuration.
func New(app application.Application, cfg Config) (*Server, error) {
	logger := app.Logger()

	// Set defaults
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 5 * time.Minute
	}

	// Create unified event broker
	broker := events.NewBroker(logger)

	// Create transport layers
	wsHub := ws.NewHub(logger)
	sseBroadcaster := sse.NewBroadcaster(logger)

	// Subscribe transports to broker
	broker.Subscribe(adapters.NewWebSocketSubscriber(wsHub))
	broker.Subscribe(adapters.NewSSESubscriber(sseBroadcaster))

	server := &Server{
		app:            app,
		cache:          cache.New(cfg.CacheTTL, cfg.CacheTTL*2),
		broker:         broker,
		wsHub:          wsHub,
		sseBroadcaster: sseBroadcaster,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				return true // Allow all origins for WebSocket
			},
		},
		logger: logger,
		config: cfg,
	}

	// Connect Starmap hooks to event broker
	if err := server.connectHooks(); err != nil {
		return nil, err
	}

	return server, nil
}

// connectHooks registers Starmap event hooks to publish to the broker.
func (s *Server) connectHooks() error {
	sm, err := s.app.Starmap()
	if err != nil {
		return err
	}

	// Model added
	sm.OnModelAdded(func(model catalogs.Model) {
		s.broker.Publish(events.ModelAdded, map[string]any{
			"model": model,
		})
		s.logger.Debug().
			Str("model_id", model.ID).
			Msg("Model added event published")
	})

	// Model updated
	sm.OnModelUpdated(func(old, updated catalogs.Model) {
		s.broker.Publish(events.ModelUpdated, map[string]any{
			"old_model": old,
			"new_model": updated,
		})
		s.logger.Debug().
			Str("model_id", updated.ID).
			Msg("Model updated event published")
	})

	// Model removed
	sm.OnModelRemoved(func(model catalogs.Model) {
		s.broker.Publish(events.ModelDeleted, map[string]any{
			"model": model,
		})
		s.logger.Debug().
			Str("model_id", model.ID).
			Msg("Model deleted event published")
	})

	s.logger.Info().Msg("Starmap hooks connected to event broker")
	return nil
}

// Start starts background services (broker, WebSocket hub, SSE broadcaster).
func (s *Server) Start() {
	go s.broker.Run()
	go s.wsHub.Run()
	go s.sseBroadcaster.Run()
}

// Handler returns the configured http.Handler with middleware chain applied.
func (s *Server) Handler() http.Handler {
	return s.setupRouter()
}

// Shutdown gracefully shuts down background services.
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop accepting new connections to WebSocket hub and SSE broadcaster
	// They will drain existing connections gracefully
	s.logger.Info().Msg("Shutting down server background services")

	// Context cancellation will be handled by the HTTP server shutdown
	// WebSocket and SSE clients will be closed when connections are terminated

	return nil
}

// Cache returns the server's cache instance.
func (s *Server) Cache() *cache.Cache {
	return s.cache
}

// WSHub returns the WebSocket hub.
func (s *Server) WSHub() *ws.Hub {
	return s.wsHub
}

// SSEBroadcaster returns the SSE broadcaster.
func (s *Server) SSEBroadcaster() *sse.Broadcaster {
	return s.sseBroadcaster
}

// Broker returns the event broker for publishing events.
func (s *Server) Broker() *events.Broker {
	return s.broker
}
