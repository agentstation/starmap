package remote

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

// TestSourceRetriesAStartThatTheUpstreamRefused proves that one refused
// credential never disables the cascaded source. An operator who rotates the
// key must recover on the next poll. The second read opens the subscriber
// lifecycle again instead of replaying the first error.
func TestSourceRetriesAStartThatTheUpstreamRefused(t *testing.T) {
	t.Parallel()

	generation := subscriberTestGeneration(
		t,
		"generation-retry-start",
		"provider-retry-start",
		time.Date(2026, time.September, 2, 10, 0, 0, 0, time.UTC),
	)
	var refuse atomic.Bool
	refuse.Store(true)
	var streamRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path[len("/api/v1"):] {
			case protocol.ManifestPath:
				writeSubscriberManifest(t, writer, generation)
			case protocol.PayloadPath(generation.Manifest.GenerationID):
				writer.Header().Set(
					"Content-Type",
					catalogs.CatalogPayloadMediaType,
				)
				_, _ = writer.Write(generation.Payload)
			case protocol.SourceChainPath:
				writeSourceTestChain(t, writer, generation)
			case protocol.EventStreamPath:
				streamRequests.Add(1)
				if refuse.Load() {
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				writer.Header().Set(
					"Content-Type",
					protocol.EventStreamMediaType,
				)
				_, _ = fmt.Fprint(writer, ": connected\n\n")
				writer.(http.Flusher).Flush()
				<-request.Context().Done()
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()

	source, err := NewSource(context.Background(), SourceConfig{
		Subscriber: Config{
			BaseURL:         server.URL + "/api/v1",
			HTTPClient:      server.Client(),
			CatalogStore:    storage.NewMemory(),
			ShutdownTimeout: time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	defer func() {
		if err := source.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()

	ctx := context.Background()
	_, err = source.Read(ctx)
	var apiErr *pkgerrors.APIError
	if !stderrors.As(err, &apiErr) ||
		apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("first Read error = %v, want a typed unauthorized error", err)
	}

	// The operator rotated the credential, so the next poll must recover.
	refuse.Store(false)
	read, err := source.Read(ctx)
	if err != nil {
		t.Fatalf("second Read: %v", err)
	}
	if !read.Changed {
		t.Fatal("the second read reported no change after the recovery")
	}
	if got := read.Generation.Manifest.GenerationID; got !=
		generation.Manifest.GenerationID {
		t.Fatalf("generation = %q, want %q",
			got, generation.Manifest.GenerationID)
	}
	if got := streamRequests.Load(); got < 2 {
		t.Fatalf("stream requests = %d, want a second start attempt", got)
	}
}

// writeSourceTestChain serves the minimal chain of an origin.
func writeSourceTestChain(
	t testing.TB,
	writer http.ResponseWriter,
	generation catalogs.Generation,
) {
	t.Helper()
	data, err := protocol.MarshalSourceChain(protocol.SourceChain{
		SchemaVersion:    protocol.SourceChainSchemaVersion,
		Identity:         "origin",
		Health:           protocol.SourceChainHealthOK,
		UpstreamHealth:   protocol.SourceChainHealthOK,
		GenerationID:     generation.Manifest.GenerationID,
		ChannelUpdatedAt: generation.Manifest.GeneratedAt,
		ObservedAt:       generation.Manifest.GeneratedAt,
	})
	if err != nil {
		t.Fatalf("MarshalSourceChain: %v", err)
	}
	writer.Header().Set("Content-Type", protocol.SourceChainMediaType)
	_, _ = writer.Write(data)
}
