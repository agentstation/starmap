package remote

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogremote"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
)

func TestSubscriberReadersObserveCompleteGenerationsDuringActivation(t *testing.T) {
	t.Parallel()

	oldProviders := concurrentProviderIDs("old", 8)
	newProviders := concurrentProviderIDs("new", 8)
	first := concurrentSubscriberGeneration(
		t,
		"generation-concurrent-first",
		oldProviders,
		time.Date(2026, time.July, 29, 18, 30, 0, 0, time.UTC),
	)
	second := concurrentSubscriberGeneration(
		t,
		"generation-concurrent-second",
		newProviders,
		first.Manifest.GeneratedAt.Add(time.Minute),
	)

	var (
		serverMu sync.RWMutex
		current  = first
		events   = make(chan string, 1)
	)
	generations := map[string]catalogstore.Generation{
		first.Manifest.GenerationID:  first,
		second.Manifest.GenerationID: second,
	}
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			resourcePath := request.URL.Path[len("/api/v1"):]
			serverMu.RLock()
			selected := current
			serverMu.RUnlock()
			switch resourcePath {
			case catalogremote.ManifestPath:
				writeSubscriberManifest(t, writer, selected)
				return
			case catalogremote.EventStreamPath:
				writer.Header().Set(
					"Content-Type",
					catalogremote.EventStreamMediaType,
				)
				_, _ = fmt.Fprint(writer, ": connected\n\n")
				writer.(http.Flusher).Flush()
				for {
					select {
					case <-request.Context().Done():
						return
					case frame := <-events:
						_, _ = fmt.Fprint(writer, frame)
						writer.(http.Flusher).Flush()
					}
				}
			}
			for id, generation := range generations {
				switch resourcePath {
				case catalogremote.GenerationManifestPath(id):
					writeSubscriberManifest(t, writer, generation)
					return
				case catalogremote.PayloadPath(id):
					writer.Header().Set(
						"Content-Type",
						catalogs.CatalogPayloadMediaType,
					)
					_, _ = writer.Write(generation.Payload)
					return
				}
			}
			http.NotFound(writer, request)
		},
	))
	defer server.Close()

	subscriber, err := New(Config{
		BaseURL:         server.URL + "/api/v1",
		HTTPClient:      server.Client(),
		CatalogStore:    catalogstore.NewMemory(),
		ShutdownTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := subscriber.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = subscriber.Close() }()
	oldCatalog := subscriber.Catalog()

	var (
		oldReads atomic.Uint64
		newReads atomic.Uint64
		stop     = make(chan struct{})
		readers  sync.WaitGroup
		readErr  = make(chan string, 1)
	)
	for range 32 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				state := subscriber.State()
				kind := classifyConcurrentCatalog(
					state.Catalog, oldProviders, newProviders,
				)
				switch {
				case state.GenerationID == first.Manifest.GenerationID &&
					state.PayloadChecksum == first.Manifest.Payload.Checksum &&
					kind == "old":
					oldReads.Add(1)
				case state.GenerationID == second.Manifest.GenerationID &&
					state.PayloadChecksum == second.Manifest.Payload.Checksum &&
					kind == "new":
					newReads.Add(1)
				default:
					select {
					case readErr <- fmt.Sprintf(
						"reader observed a partial or mixed generation: id=%q checksum=%q kind=%q",
						state.GenerationID,
						state.PayloadChecksum,
						kind,
					):
					default:
					}
					return
				}
			}
		}()
	}
	waitForSubscriberCondition(t, func() bool {
		return oldReads.Load() >= 1_000
	})

	serverMu.Lock()
	current = second
	serverMu.Unlock()
	events <- "id: 1\n" +
		"event: catalog.published\n" +
		"data: {\"generation_id\":\"generation-concurrent-second\",\"sequence\":1}\n\n"
	waitForSubscriberCondition(t, func() bool {
		return newReads.Load() >= 1_000
	})
	close(stop)
	readers.Wait()
	select {
	case message := <-readErr:
		t.Fatal(message)
	default:
	}
	if oldReads.Load() == 0 || newReads.Load() == 0 {
		t.Fatalf(
			"generation observations old=%d new=%d, want both",
			oldReads.Load(),
			newReads.Load(),
		)
	}
	newCatalog := subscriber.Catalog()
	if oldCatalog == newCatalog {
		t.Fatal("remote activation retained the old immutable catalog pointer")
	}
	if kind := classifyConcurrentCatalog(
		newCatalog,
		oldProviders,
		newProviders,
	); kind != "new" {
		t.Fatalf("final catalog kind = %q, want new", kind)
	}
}

func concurrentProviderIDs(prefix string, count int) []catalogs.ProviderID {
	ids := make([]catalogs.ProviderID, count)
	for index := range ids {
		ids[index] = catalogs.ProviderID(fmt.Sprintf("%s-provider-%02d", prefix, index))
	}
	return ids
}

func concurrentSubscriberGeneration(
	t testing.TB,
	generationID string,
	providerIDs []catalogs.ProviderID,
	generatedAt time.Time,
) catalogstore.Generation {
	t.Helper()
	generation := subscriberTestGeneration(
		t,
		generationID,
		providerIDs[0],
		generatedAt,
	)
	catalog, err := catalogstore.DecodeCatalogPayload(generation.Payload)
	if err != nil {
		t.Fatalf("DecodeCatalogPayload: %v", err)
	}
	builder, err := catalogs.NewBuilderFrom(catalog)
	if err != nil {
		t.Fatalf("NewBuilderFrom: %v", err)
	}
	for _, providerID := range providerIDs[1:] {
		if err := builder.SetProvider(catalogs.Provider{
			ID: providerID, Name: string(providerID),
		}); err != nil {
			t.Fatalf("SetProvider(%s): %v", providerID, err)
		}
	}
	catalog, err = builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	generation.Payload, err = catalogstore.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	descriptor := catalogs.DescribeCatalogPayload(generation.Payload)
	generation.Manifest.Payload = descriptor
	generation.Manifest.SourceObservations[0].Revision.Value = descriptor.Checksum
	generation.Manifest.SourceObservations[0].EvidenceChecksum = descriptor.Checksum
	if err := generation.Validate(); err != nil {
		t.Fatalf("generation fixture: %v", err)
	}
	return generation
}

func classifyConcurrentCatalog(
	catalog *catalogs.Catalog,
	oldProviders []catalogs.ProviderID,
	newProviders []catalogs.ProviderID,
) string {
	oldCount := countConcurrentProviders(catalog, oldProviders)
	newCount := countConcurrentProviders(catalog, newProviders)
	switch {
	case oldCount == len(oldProviders) && newCount == 0:
		return "old"
	case newCount == len(newProviders) && oldCount == 0:
		return "new"
	default:
		return "mixed"
	}
}

func countConcurrentProviders(
	catalog *catalogs.Catalog,
	providerIDs []catalogs.ProviderID,
) int {
	count := 0
	for _, providerID := range providerIDs {
		if _, err := catalog.Provider(providerID); err == nil {
			count++
		}
	}
	return count
}
