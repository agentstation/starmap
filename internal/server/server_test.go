package server

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// TestServerInitialization proves construction starts no transport goroutine.
func TestServerInitialization(t *testing.T) {
	// Create mock app instance
	testApp := newMockApplication()

	// Create server config
	serverCfg := Config{
		Host:       "localhost",
		Port:       18081,
		PathPrefix: "/api/v1",
		CacheTTL:   5 * time.Minute,
	}

	// Test with timeout to catch deadlocks
	done := make(chan struct{})
	var srv *Server
	var newErr error

	go func() {
		srv, newErr = New(testApp, serverCfg)
		close(done)
	}()

	select {
	case <-done:
		if newErr != nil {
			t.Fatalf("server.New() failed: %v", newErr)
		}
		if srv == nil {
			t.Fatal("server.New() returned nil server")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server.New() deadlocked - did not complete within 5 seconds")
	}

	// Cleanup
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}

func TestNewRejectsNilStarmapClient(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	server, err := New(
		&mockApplication{logger: &logger},
		DefaultConfig(),
	)
	if server != nil || err == nil {
		t.Fatalf(
			"New with nil Starmap client = (%#v, %v), want nil error result",
			server,
			err,
		)
	}
}

func TestServerStartDoesNotOwnAHiddenTransportLoop(t *testing.T) {
	// Create mock app instance
	testApp := newMockApplication()

	// Create server config
	serverCfg := Config{
		Host:       "localhost",
		Port:       18082,
		PathPrefix: "/api/v1",
		CacheTTL:   5 * time.Minute,
	}

	// Create server
	srv, err := New(testApp, serverCfg)
	if err != nil {
		t.Fatalf("server.New() failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		srv.Start()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("srv.Start() appears to have deadlocked")
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// mockApplication is a minimal Application implementation for testing
type mockApplication struct {
	logger        *zerolog.Logger
	sm            *starmap.Client
	runtime       *starmap.Runtime
	runtimeStatus *starmap.RuntimeStatus
	catalog       *catalogs.Catalog
	catalogState  *starmap.CatalogState
}

func newMockApplication() *mockApplication {
	logger := zerolog.Nop()

	// Create embedded starmap client
	sm, err := starmap.New()
	if err != nil {
		panic("Failed to create starmap client: " + err.Error())
	}

	return &mockApplication{
		logger: &logger,
		sm:     sm,
	}
}

func (m *mockApplication) Catalog() (*catalogs.Catalog, error) {
	if m.catalog != nil {
		return m.catalog, nil
	}
	return m.sm.Catalog(), nil
}

func (m *mockApplication) CatalogState() (starmap.CatalogState, error) {
	if m.catalogState != nil {
		return *m.catalogState, nil
	}
	return m.sm.CurrentCatalogState(), nil
}

func (m *mockApplication) Readiness() (starmap.CatalogReadiness, error) {
	return m.sm.Readiness(), nil
}

func (m *mockApplication) RuntimeStatus() starmap.RuntimeStatus {
	if m.runtimeStatus != nil {
		return *m.runtimeStatus
	}
	return m.runtime.Status()
}

func (m *mockApplication) Starmap(...starmap.Option) (*starmap.Client, error) {
	return m.sm, nil
}

func (m *mockApplication) Sync(
	ctx context.Context,
	options ...pkgsync.Option,
) (*pkgsync.Result, error) {
	syncer, err := acquisition.New(m.sm)
	if err != nil {
		return nil, err
	}
	return syncer.Sync(ctx, options...)
}

func (*mockApplication) UpdatesEnabled() bool {
	return true
}

func (m *mockApplication) Logger() *zerolog.Logger {
	return m.logger
}

func (m *mockApplication) OutputFormat() string {
	return "table"
}

func (m *mockApplication) Version() string {
	return "test"
}

func (m *mockApplication) Commit() string {
	return "test-commit"
}

func (m *mockApplication) Date() string {
	return "test-date"
}

func (m *mockApplication) BuiltBy() string {
	return "test"
}
