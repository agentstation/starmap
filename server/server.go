package server

import (
	"context"
	stderrors "errors"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"sync"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	internalserver "github.com/agentstation/starmap/internal/server"
	"github.com/agentstation/starmap/pkg/errors"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// Syncer is the optional acquisition capability used by the update endpoint.
// Read-only servers do not need one.
type Syncer interface {
	Sync(context.Context, ...pkgsync.Option) (*pkgsync.Result, error)
}

// Option configures a Server dependency.
type Option func(*options) error

type options struct {
	logger *zerolog.Logger
	syncer Syncer
}

// WithLogger configures server diagnostics. The default logger discards output.
func WithLogger(logger *zerolog.Logger) Option {
	return func(options *options) error {
		if logger == nil {
			return &errors.ValidationError{Field: "server.logger", Message: "is required"}
		}
		options.logger = logger
		return nil
	}
}

// WithSyncer enables explicit source acquisition through the update endpoint.
func WithSyncer(syncer Syncer) Option {
	return func(options *options) error {
		if isNilSyncer(syncer) {
			return &errors.ValidationError{Field: "server.syncer", Message: "is required"}
		}
		options.syncer = syncer
		return nil
	}
}

func isNilSyncer(syncer Syncer) bool {
	if syncer == nil {
		return true
	}
	value := reflect.ValueOf(syncer)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// Server serves one Starmap client's immutable catalog over HTTP.
//
// Construction starts no listener or background goroutine. Serve starts the
// server-owned services and blocks until the listener fails or Shutdown drains
// the HTTP server.
type Server struct {
	implementation *internalserver.Server
	httpServer     *http.Server
	startOnce      sync.Once
}

// New constructs an embeddable server for client.
func New(client *starmap.Client, config Config, serverOptions ...Option) (*Server, error) {
	if client == nil {
		return nil, &errors.ValidationError{Field: "server.client", Message: "is required"}
	}
	logger := zerolog.Nop()
	options := options{logger: &logger}
	for _, option := range serverOptions {
		if option == nil {
			return nil, &errors.ValidationError{Field: "server.option", Message: "is required"}
		}
		if err := option(&options); err != nil {
			return nil, err
		}
	}

	config = config.normalized()
	if err := config.validate(); err != nil {
		return nil, err
	}
	implementation, err := internalserver.New(
		&clientApplication{client: client, logger: options.logger, syncer: options.syncer},
		config.internal(),
	)
	if err != nil {
		return nil, errors.WrapResource("construct", "starmap server", "", err)
	}
	handler := implementation.Handler()
	return &Server{
		implementation: implementation,
		httpServer: &http.Server{
			Addr:         net.JoinHostPort(config.Host, strconv.Itoa(config.Port)),
			Handler:      handler,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			IdleTimeout:  config.IdleTimeout,
		},
	}, nil
}

// Handler returns the configured HTTP handler. Call Start before serving this
// handler through a caller-owned http.Server. The caller must drain that
// http.Server before calling Shutdown to stop Starmap's background services.
func (s *Server) Handler() http.Handler {
	if s == nil || s.httpServer == nil {
		return nil
	}
	return s.httpServer.Handler
}

// Start starts server-owned background services exactly once.
func (s *Server) Start() error {
	if s == nil || s.implementation == nil {
		return &errors.ValidationError{Field: "server", Message: "is required"}
	}
	s.startOnce.Do(s.implementation.Start)
	return nil
}

// Serve starts server-owned services and serves listener until Shutdown or a
// listener failure. A normal Shutdown returns nil.
func (s *Server) Serve(listener net.Listener) error {
	if s == nil || s.httpServer == nil {
		return &errors.ValidationError{Field: "server", Message: "is required"}
	}
	if listener == nil {
		return &errors.ValidationError{Field: "server.listener", Message: "is required"}
	}
	if err := s.Start(); err != nil {
		return err
	}
	err := s.httpServer.Serve(listener)
	if stderrors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown drains the HTTP server used by Serve and then stops server-owned
// background services within ctx. A caller serving Handler through its own
// http.Server must drain that server first.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.httpServer == nil || s.implementation == nil {
		return &errors.ValidationError{Field: "server", Message: "is required"}
	}
	if ctx == nil {
		return &errors.ValidationError{Field: "context", Message: "is required"}
	}
	httpErr := s.httpServer.Shutdown(ctx)
	serviceErr := s.implementation.Shutdown(ctx)
	return stderrors.Join(httpErr, serviceErr)
}
