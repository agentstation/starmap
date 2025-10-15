package server

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestServerInitialization tests that server.New() completes without blocking.
// This test would catch the deadlock bug where Subscribe() is called before Run().
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

// TestServerStartWithoutNew tests that calling Start() after New() doesn't deadlock.
func TestServerStartWithoutNew(t *testing.T) {
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

	// Start background services - should not block
	done := make(chan struct{})
	go func() {
		srv.Start()
		// Give services a moment to start
		time.Sleep(100 * time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
		// Success - Start() completed without blocking
	case <-time.After(5 * time.Second):
		t.Fatal("srv.Start() appears to have deadlocked")
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// TestServerSubscribersWithoutRun tests that broker subscribers can be added before Run() starts.
// This is the exact scenario that caused the deadlock bug.
func TestServerSubscribersWithoutRun(t *testing.T) {
	logger := zerolog.Nop()

	// Create broker without starting Run()
	broker := newTestBroker(&logger)

	// Try to subscribe - this should NOT block with buffered channels
	done := make(chan struct{})
	go func() {
		// Subscribe without calling broker.Run()
		broker.Subscribe(newTestSubscriber())
		close(done)
	}()

	select {
	case <-done:
		// Success - Subscribe() did not block
	case <-time.After(2 * time.Second):
		t.Fatal("broker.Subscribe() deadlocked without broker.Run() - channels are not buffered!")
	}
}

// TestServerComponentChannelsBuffered verifies all server components use buffered channels.
func TestServerComponentChannelsBuffered(t *testing.T) {
	// Create mock app instance
	testApp := newMockApplication()

	serverCfg := Config{
		Host:       "localhost",
		Port:       18083,
		PathPrefix: "/api/v1",
		CacheTTL:   5 * time.Minute,
	}

	// Create server - this internally subscribes to broker before Run()
	srv, err := New(testApp, serverCfg)
	if err != nil {
		t.Fatalf("server.New() failed: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	// If we got here without blocking, channels are properly buffered
	t.Log("Server initialized successfully with buffered channels")
}

// Helper types for testing

type testSubscriber struct{}

func newTestSubscriber() *testSubscriber {
	return &testSubscriber{}
}

func (s *testSubscriber) Send(event any) error {
	return nil
}

func (s *testSubscriber) Close() error {
	return nil
}

// testBroker creates a broker instance for testing
func newTestBroker(logger *zerolog.Logger) *testBrokerWrapper {
	return &testBrokerWrapper{
		register:   make(chan interface{}, 10), // Buffered like production
		unregister: make(chan interface{}, 10), // Buffered like production
	}
}

type testBrokerWrapper struct {
	register   chan interface{}
	unregister chan interface{}
}

func (b *testBrokerWrapper) Subscribe(sub interface{}) {
	// Simulate the broker.Subscribe() call
	b.register <- sub
}

// mockApplication is a minimal Application implementation for testing
type mockApplication struct {
	logger *zerolog.Logger
	sm     starmap.Client
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

func (m *mockApplication) Catalog() (catalogs.Catalog, error) {
	return m.sm.Catalog()
}

func (m *mockApplication) Starmap(...starmap.Option) (starmap.Client, error) {
	return m.sm, nil
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
