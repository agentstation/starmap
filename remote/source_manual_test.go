package remote

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime"
)

func TestManualRuntimeRefreshNeverOpensStream(t *testing.T) {
	generation := subscriberTestGeneration(t, "manual-source", "manual-provider", time.Date(2026, 9, 11, 20, 0, 0, 0, time.UTC))
	var streams atomic.Int64
	var manifests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1" + protocol.ManifestPath:
			manifests.Add(1)
			writeSubscriberManifest(t, w, generation)
		case "/api/v1" + protocol.PayloadPath(generation.Manifest.GenerationID):
			w.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
			_, _ = w.Write(generation.Payload)
		case "/api/v1" + protocol.SourceChainPath:
			writeSourceTestChain(t, w, generation)
		case "/api/v1" + protocol.EventStreamPath:
			streams.Add(1)
			w.Header().Set("Content-Type", protocol.EventStreamMediaType)
			_, _ = fmt.Fprint(w, ": connected\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	source, err := NewSource(t.Context(), SourceConfig{Subscriber: Config{
		BaseURL: server.URL + "/api/v1", HTTPClient: server.Client(), CatalogStore: storage.NewMemory(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	connected, err := runtime.Open(t.Context(),
		runtime.WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())),
		runtime.WithStateDirectory(filepath.Join(t.TempDir(), "runtime")),
		runtime.WithSource(source),
		runtime.WithSourceRefreshMode("manual"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connected.Close() })
	if manifests.Load() != 0 {
		t.Fatal("manual runtime fetched the source during construction")
	}
	for range 2 {
		if _, err := connected.RefreshSource(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if got := streams.Load(); got != 0 {
		t.Fatalf("manual refresh opened %d event streams", got)
	}
	if got := manifests.Load(); got != 2 {
		t.Fatalf("two manual refreshes fetched %d manifests, want two", got)
	}
}

func TestSourceManualReadCancellation(t *testing.T) {
	for _, stage := range []string{"manifest", "payload", "chain"} {
		for _, action := range []string{"close", "cancel"} {
			t.Run(stage+"/"+action, func(t *testing.T) {
				generation := subscriberTestGeneration(t, "cancel-read", "manual-provider", time.Date(2026, 9, 11, 20, 0, 0, 0, time.UTC))
				paths := map[string]string{"manifest": protocol.ManifestPath, "payload": protocol.PayloadPath(generation.Manifest.GenerationID), "chain": protocol.SourceChainPath}
				entered := make(chan struct{}, 1)
				stopped := make(chan struct{}, 1)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/api/v1"+paths[stage] {
						entered <- struct{}{}
						<-r.Context().Done()
						stopped <- struct{}{}
						return
					}
					switch r.URL.Path {
					case "/api/v1" + protocol.ManifestPath:
						writeSubscriberManifest(t, w, generation)
					case "/api/v1" + protocol.PayloadPath(generation.Manifest.GenerationID):
						w.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
						_, _ = w.Write(generation.Payload)
					default:
						t.Errorf("manual read requested unexpected route %s", r.URL.Path)
						http.NotFound(w, r)
					}
				}))
				t.Cleanup(server.Close)
				source, err := NewSource(t.Context(), SourceConfig{Subscriber: Config{
					BaseURL: server.URL + "/api/v1", HTTPClient: server.Client(), ShutdownTimeout: time.Second,
				}})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = source.Close() })
				ctx, cancel := context.WithCancel(t.Context())
				t.Cleanup(cancel)
				result := make(chan error, 1)
				go func() {
					_, err := source.ReadOnce(ctx)
					result <- err
				}()
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					t.Fatal("manual read did not reach its HTTP boundary")
				}
				var conflict *pkgerrors.ConflictError
				if _, err := source.ReadOnce(t.Context()); !stderrors.As(err, &conflict) {
					t.Fatalf("concurrent manual read returned %v, want conflict", err)
				}
				if err := source.subscriber.Start(t.Context()); !stderrors.As(err, &conflict) {
					t.Fatalf("concurrent subscription returned %v, want conflict", err)
				}
				if action == "close" {
					if err := source.Close(); err != nil {
						t.Fatal(err)
					}
				} else {
					cancel()
				}
				select {
				case err := <-result:
					if !stderrors.Is(err, context.Canceled) {
						t.Fatalf("manual read returned %v, want cancellation", err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("manual read did not stop")
				}
				select {
				case <-stopped:
				case <-time.After(5 * time.Second):
					t.Fatal("manual HTTP request did not stop")
				}
				source.subscriber.mu.Lock()
				state := source.subscriber.state
				source.subscriber.mu.Unlock()
				want := stateIdle
				if action == "close" {
					want = stateStopped
				}
				if state != want {
					t.Fatalf("manual read left lifecycle %v, want %v", state, want)
				}
			})
		}
	}
}
