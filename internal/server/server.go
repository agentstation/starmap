// Package server provides HTTP server implementation for the Starmap API.
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/cmd/application"
	"github.com/agentstation/starmap/internal/server/cache"
	"github.com/agentstation/starmap/internal/server/sse"
	ws "github.com/agentstation/starmap/internal/server/websocket"
)

// Server holds the HTTP server state and dependencies.
type Server struct {
	app            application.Application
	cache          *cache.Cache
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

	server := &Server{
		app:            app,
		cache:          cache.New(cfg.CacheTTL, cfg.CacheTTL*2),
		wsHub:          ws.NewHub(logger),
		sseBroadcaster: sse.NewBroadcaster(logger),
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

	return server, nil
}

// Start starts background services (WebSocket hub, SSE broadcaster).
func (s *Server) Start() {
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

// BroadcastEvent sends an event to both WebSocket and SSE clients.
func (s *Server) BroadcastEvent(eventType string, data any) {
	timestamp := time.Now()

	// WebSocket
	s.wsHub.Broadcast(ws.Message{
		Type:      eventType,
		Timestamp: timestamp,
		Data:      data,
	})

	// SSE
	s.sseBroadcaster.Broadcast(sse.Event{
		Event: eventType,
		ID:    fmt.Sprintf("%d", timestamp.Unix()),
		Data:  data,
	})
}
