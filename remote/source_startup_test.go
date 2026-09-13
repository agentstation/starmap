package remote

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
)

func TestSourceStartupCancellation(t *testing.T) {
	for _, stage := range []string{"manifest", "payload", "stream", "catch-up"} {
		for _, action := range []string{"cancel", "close"} {
			t.Run(stage+"/"+action, func(t *testing.T) {
				generation := subscriberTestGeneration(t, "startup-cancel", "provider", time.Date(2026, 9, 11, 21, 0, 0, 0, time.UTC))
				entered := make(chan struct{})
				release := make(chan struct{})
				var once sync.Once
				var blocked atomic.Bool
				blocked.Store(true)
				var mu sync.Mutex
				manifests := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					if r.URL.Path == "/api/v1"+protocol.ManifestPath {
						manifests++
					}
					count := manifests
					mu.Unlock()
					block := stage == "manifest" && r.URL.Path == "/api/v1"+protocol.ManifestPath ||
						stage == "payload" && r.URL.Path == "/api/v1"+protocol.PayloadPath(generation.Manifest.GenerationID) ||
						stage == "stream" && r.URL.Path == "/api/v1"+protocol.EventStreamPath ||
						stage == "catch-up" && count == 2 && r.URL.Path == "/api/v1"+protocol.ManifestPath
					if blocked.Load() && block {
						once.Do(func() { close(entered) })
						select {
						case <-r.Context().Done():
						case <-release:
						}
						return
					}
					switch r.URL.Path {
					case "/api/v1" + protocol.ManifestPath:
						writeSubscriberManifest(t, w, generation)
					case "/api/v1" + protocol.PayloadPath(generation.Manifest.GenerationID):
						w.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
						_, _ = w.Write(generation.Payload)
					case "/api/v1" + protocol.EventStreamPath:
						w.Header().Set("Content-Type", protocol.EventStreamMediaType)
						_, _ = fmt.Fprint(w, ": connected\n\n")
						w.(http.Flusher).Flush()
						select {
						case <-r.Context().Done():
						case <-release:
						}
					default:
						http.NotFound(w, r)
					}
				}))
				t.Cleanup(server.Close)
				t.Cleanup(func() { close(release) })
				source, err := NewSource(t.Context(), SourceConfig{Subscriber: Config{BaseURL: server.URL + "/api/v1", HTTPClient: server.Client(), ShutdownTimeout: time.Second}})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = source.Close() })
				ctx, cancel := context.WithCancel(t.Context())
				t.Cleanup(cancel)
				finished := make(chan error, 1)
				go func() { _, err := source.Read(ctx); finished <- err }()
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					t.Fatal("startup did not reach the selected request")
				}
				if action == "close" {
					closed := make(chan error, 1)
					go func() { closed <- source.Close() }()
					select {
					case err := <-closed:
						if err != nil {
							t.Fatal(err)
						}
					case <-time.After(3 * time.Second):
						t.Fatal("Close blocked behind source startup")
					}
				} else {
					cancel()
				}
				select {
				case err := <-finished:
					if !stderrors.Is(err, context.Canceled) {
						t.Fatalf("startup returned %v, want cancellation", err)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("source initialization survived cancellation")
				}
				blocked.Store(false)
				retryCtx, retryCancel := context.WithTimeout(t.Context(), 3*time.Second)
				defer retryCancel()
				_, err = source.Read(retryCtx)
				if action == "cancel" && err != nil {
					t.Fatalf("source did not recover after canceled startup: %v", err)
				}
				if action == "close" && err == nil {
					t.Fatal("closed source restarted")
				}
			})
		}
	}
}
