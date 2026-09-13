package remote

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime"
)

type stalledReconnectTransport struct {
	base      http.RoundTripper
	manifests atomic.Int64
	entered   chan struct{}
	release   chan struct{}
	stalled   sync.Once
}

func (s *stalledReconnectTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Path == "/api/v1"+protocol.ManifestPath && s.manifests.Add(1) == 3 {
		s.stalled.Do(func() { close(s.entered) })
		// The injected transport delays cancellation until the test releases it.
		<-s.release
	}
	return s.base.RoundTrip(request)
}

func TestOwnedSourceCloseTimeoutKeepsDirectoryOwned(t *testing.T) {
	generation := subscriberTestGeneration(t, "shutdown-source", "provider", time.Date(2026, 9, 11, 21, 0, 0, 0, time.UTC))
	disconnect := make(chan struct{})
	var streams atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1" + protocol.ManifestPath:
			writeSubscriberManifest(t, w, generation)
		case "/api/v1" + protocol.PayloadPath(generation.Manifest.GenerationID):
			w.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
			_, _ = w.Write(generation.Payload)
		case "/api/v1" + protocol.SourceChainPath:
			writeSourceTestChain(t, w, generation)
		case "/api/v1" + protocol.EventStreamPath:
			first := streams.Add(1) == 1
			w.Header().Set("Content-Type", protocol.EventStreamMediaType)
			_, _ = fmt.Fprint(w, ": connected\n\n")
			w.(http.Flusher).Flush()
			if first {
				select {
				case <-disconnect:
				case <-r.Context().Done():
				}
			} else {
				<-r.Context().Done()
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client := server.Client()
	transport := &stalledReconnectTransport{base: client.Transport, entered: make(chan struct{}), release: make(chan struct{})}
	release := sync.OnceFunc(func() { close(transport.release) })
	defer release()
	client.Transport = transport
	source, err := NewSource(t.Context(), SourceConfig{Subscriber: Config{
		BaseURL: server.URL + "/api/v1", HTTPClient: client, CatalogStore: storage.NewMemory(), ShutdownTimeout: 20 * time.Millisecond,
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	source.subscriber.retryDelay = func(int) time.Duration { return 0 }
	directory := filepath.Join(t.TempDir(), "runtime")
	connected, err := runtime.Open(t.Context(), runtime.WithStateDirectory(directory), runtime.WithOwnedSource(source),
		runtime.WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())), runtime.WithAcquisitionEnabled(false),
		runtime.WithSourcePollInterval(0), runtime.WithStartupSpread(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connected.Close() })
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	close(disconnect)
	select {
	case <-transport.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("subscriber did not enter the stalled reconnect fetch")
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer shutdownCancel()
	if err := source.Shutdown(shutdownCtx); !stderrors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("source shutdown error=%v, want its context deadline", err)
	}
	if err := connected.Close(); !stderrors.Is(err, pkgerrors.ErrTimeout) {
		t.Fatalf("Close error=%v, want bounded timeout", err)
	}
	lock := flock.New(filepath.Join(directory, ".owner.lock"))
	defer func() { _ = lock.Close() }()
	held, err := lock.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if held {
		t.Fatal("runtime released its directory while the source worker was still active")
	}
	release()
	deadline, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for {
		held, err := lock.TryLock()
		if err != nil {
			t.Fatal(err)
		}
		if held {
			return
		}
		select {
		case <-deadline.Done():
			t.Fatal("runtime retained its directory after the source worker stopped")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
