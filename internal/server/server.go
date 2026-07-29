// Package server provides HTTP server implementation for the Starmap API.
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/server/cache"
	"github.com/agentstation/starmap/internal/server/sse"
	"github.com/agentstation/starmap/pkg/errors"
)

// Server holds the HTTP server state and dependencies.
type Server struct {
	app            Application
	cache          *cache.Cache
	sseBroadcaster *sse.Broadcaster
	logger         *zerolog.Logger
	config         Config
	startTime      time.Time
}

// New creates a new server instance with the given configuration.
func New(app Application, cfg Config) (*Server, error) {
	logger := app.Logger()

	logger.Debug().Msg("Creating new server instance")

	// Set defaults
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 5 * time.Minute
	}

	logger.Debug().Msg("Creating SSE broadcaster")
	sseBroadcaster, err := sse.NewBroadcaster(sse.Config{
		HeartbeatInterval: cfg.SSEHeartbeatInterval,
		WriteTimeout:      cfg.SSEWriteTimeout,
	}, logger)
	if err != nil {
		return nil, err
	}
	logger.Debug().Msg("SSE broadcaster created")

	server := &Server{
		app:            app,
		cache:          cache.New(cfg.CacheTTL, cfg.CacheTTL*2),
		sseBroadcaster: sseBroadcaster,
		logger:         logger,
		config:         cfg,
		startTime:      time.Now(),
	}

	// Connect the sole post-commit publication event to SSE.
	logger.Debug().Msg("Connecting Starmap publication hook to SSE")
	if err := server.connectHooks(); err != nil {
		return nil, err
	}
	logger.Debug().Msg("Starmap publication hook connected")

	logger.Debug().Msg("Server instance created successfully")
	return server, nil
}

// connectHooks registers the one post-commit publication event.
func (s *Server) connectHooks() error {
	sm, err := s.app.Starmap()
	if err != nil {
		return err
	}

	// Generation publication is the cache/event authority. Request-side cache
	// keys also read the atomic catalog state, so hook scheduling cannot expose a
	// stale namespace after publication.
	sm.OnCatalogPublished(func(event starmap.CatalogPublishedEvent) error {
		if sm.CurrentCatalogState().GenerationID == event.GenerationID {
			s.cache.ActivateGeneration(event.Sequence, event.GenerationID)
		}
		return s.sseBroadcaster.Publish(sse.Publication{
			GenerationID: event.GenerationID,
			Sequence:     event.Sequence,
		})
	})
	s.logger.Info().Msg("Starmap publication hook connected to SSE")
	return nil
}

// Start activates server-owned services. SSE connections are request-owned, so
// no background transport goroutine is needed.
func (s *Server) Start() {
	s.logger.Debug().Msg("Server services active")
}

// Handler returns the configured http.Handler with middleware chain applied.
func (s *Server) Handler() http.Handler {
	return s.setupRouter()
}

// Shutdown terminates active SSE connections. The owning HTTP server drains
// request handlers before calling this method.
func (s *Server) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return &errors.ValidationError{
			Field: "context", Message: "is required",
		}
	}
	s.sseBroadcaster.Close()
	if err := ctx.Err(); err != nil {
		return err
	}
	s.logger.Info().Msg("Server streaming services shut down")
	return nil
}

// Cache returns the server's cache instance.
func (s *Server) Cache() *cache.Cache {
	return s.cache
}

// SSEBroadcaster returns the SSE broadcaster.
func (s *Server) SSEBroadcaster() *sse.Broadcaster {
	return s.sseBroadcaster
}

// StartTime returns the server start time for uptime calculations.
func (s *Server) StartTime() time.Time {
	return s.startTime
}
