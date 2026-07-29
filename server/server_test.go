package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestNewRejectsNilClient(t *testing.T) {
	srv, err := New(nil, DefaultConfig())
	if srv != nil {
		t.Fatal("New returned a server for a nil client")
	}
	if err == nil {
		t.Fatal("New returned nil error for a nil client")
	}
}

func TestNewRejectsInvalidConfigWithoutPanicking(t *testing.T) {
	client, err := starmap.New()
	if err != nil {
		t.Fatalf("starmap.New: %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "port", mutate: func(config *Config) { config.Port = -1 }},
		{name: "relative prefix", mutate: func(config *Config) { config.PathPrefix = "api/v1" }},
		{name: "root prefix", mutate: func(config *Config) { config.PathPrefix = "/" }},
		{name: "trailing slash", mutate: func(config *Config) { config.PathPrefix = "/api/v1/" }},
		{name: "wildcard prefix", mutate: func(config *Config) { config.PathPrefix = "/api/{version}" }},
		{name: "negative rate", mutate: func(config *Config) { config.RateLimit = -1 }},
		{name: "negative read timeout", mutate: func(config *Config) { config.ReadTimeout = -1 }},
		{
			name: "negative SSE heartbeat",
			mutate: func(config *Config) {
				config.SSEHeartbeatInterval = -time.Second
			},
		},
		{
			name: "negative SSE write timeout",
			mutate: func(config *Config) {
				config.SSEWriteTimeout = -time.Second
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := DefaultConfig()
			test.mutate(&config)
			srv, err := New(client, config)
			if srv != nil {
				t.Fatal("New returned a server for invalid config")
			}
			if err == nil {
				t.Fatal("New returned nil error for invalid config")
			}
		})
	}
}

func TestDefaultConfigIncludesSSELivenessBudgets(t *testing.T) {
	config := DefaultConfig()
	if config.SSEHeartbeatInterval != 20*time.Second {
		t.Fatalf("SSE heartbeat interval = %s, want 20s", config.SSEHeartbeatInterval)
	}
	if config.SSEWriteTimeout != 10*time.Second {
		t.Fatalf("SSE write timeout = %s, want 10s", config.SSEWriteTimeout)
	}
}

func TestReadOnlyServerDoesNotExposeUpdateRoute(t *testing.T) {
	client, err := starmap.New()
	if err != nil {
		t.Fatalf("starmap.New: %v", err)
	}
	srv, err := New(client, DefaultConfig())
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/update", nil)
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("POST /api/v1/update status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestWithSyncerRejectsTypedNil(t *testing.T) {
	client, err := starmap.New()
	if err != nil {
		t.Fatalf("starmap.New: %v", err)
	}
	var syncer *testSyncer
	srv, err := New(client, DefaultConfig(), WithSyncer(syncer))
	if srv != nil {
		t.Fatal("New returned a server for a typed-nil Syncer")
	}
	if err == nil {
		t.Fatal("New returned nil error for a typed-nil Syncer")
	}
}

type testSyncer struct{}

func (*testSyncer) Sync(
	context.Context,
	...pkgsync.Option,
) (*pkgsync.Result, error) {
	return &pkgsync.Result{}, nil
}
