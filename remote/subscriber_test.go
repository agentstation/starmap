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

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogremote"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
)

func TestSubscriberReconnectCatchesUpWithoutEventAndDeduplicatesReplay(t *testing.T) {
	t.Parallel()

	first := subscriberTestGeneration(
		t,
		"generation-first",
		"provider-first",
		time.Date(2026, time.July, 29, 14, 0, 0, 0, time.UTC),
	)
	second := subscriberTestGeneration(
		t,
		"generation-second",
		"provider-second",
		time.Date(2026, time.July, 29, 14, 1, 0, 0, time.UTC),
	)

	var (
		mu              sync.RWMutex
		current         = first
		requestCounts   = make(map[string]int)
		streamCount     atomic.Int32
		firstFrames     = make(chan string, 1)
		closeFirst      = make(chan struct{})
		secondConnected = make(chan string, 1)
	)
	generations := map[string]catalogstore.Generation{
		first.Manifest.GenerationID:  first,
		second.Manifest.GenerationID: second,
	}
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			resourcePath := request.URL.Path[len("/api/v1"):]
			mu.Lock()
			requestCounts[resourcePath]++
			selected := current
			mu.Unlock()

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
				connection := streamCount.Add(1)
				if connection == 1 {
					for {
						select {
						case frame := <-firstFrames:
							_, _ = fmt.Fprint(writer, frame)
							writer.(http.Flusher).Flush()
						case <-closeFirst:
							return
						case <-request.Context().Done():
							return
						}
					}
				}
				select {
				case secondConnected <- request.Header.Get("Last-Event-ID"):
				default:
				}
				<-request.Context().Done()
				return
			}

			for id, generation := range generations {
				switch resourcePath {
				case catalogremote.GenerationManifestPath(id):
					writeSubscriberManifest(t, writer, generation)
					return
				case catalogremote.SnapshotPath(id):
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
		BaseURL:           server.URL + "/api/v1",
		HTTPClient:        server.Client(),
		ReconnectMinDelay: time.Millisecond,
		ReconnectMaxDelay: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	subscriber.retryDelay = func(int) time.Duration { return 0 }

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := subscriber.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer subscriber.Close()
	if _, err := subscriber.Catalog().Provider("provider-first"); err != nil {
		t.Fatalf("initial remote catalog not activated: %v", err)
	}

	initialCatalog := subscriber.Catalog()
	firstFrames <- "id: 1\n" +
		"event: catalog.published\n" +
		"data: {\"generation_id\":\"generation-first\",\"sequence\":1}\n\n"
	waitForSubscriberCondition(t, func() bool {
		return subscriber.currentLastEventID() == "1"
	})
	if subscriber.Catalog() != initialCatalog {
		t.Fatal("duplicate generation event republished the immutable catalog")
	}
	mu.RLock()
	addressedDuplicateGets :=
		requestCounts[catalogremote.GenerationManifestPath(
			first.Manifest.GenerationID,
		)]
	mu.RUnlock()
	if addressedDuplicateGets != 0 {
		t.Fatalf(
			"duplicate generation caused %d addressed manifest requests",
			addressedDuplicateGets,
		)
	}

	mu.Lock()
	current = second
	mu.Unlock()
	close(closeFirst)

	select {
	case lastEventID := <-secondConnected:
		if lastEventID != "1" {
			t.Fatalf("reconnect Last-Event-ID = %q, want 1", lastEventID)
		}
	case <-ctx.Done():
		t.Fatal("subscriber did not reconnect")
	}
	waitForSubscriberCondition(t, func() bool {
		_, err := subscriber.Catalog().Provider("provider-second")
		return err == nil
	})
	if _, err := subscriber.Catalog().Provider("provider-first"); err == nil {
		t.Fatal("catch-up exposed a partial old/new catalog")
	}
}

func TestSubscriberDeduplicatesNewIdentityWithSamePayload(t *testing.T) {
	t.Parallel()

	generation := subscriberTestGeneration(
		t,
		"generation-one",
		"provider-one",
		time.Date(2026, time.July, 29, 15, 0, 0, 0, time.UTC),
	)
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	subscriber, err := New(Config{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if published, err := subscriber.activate(context.Background(), generation); err != nil ||
		!published {
		t.Fatalf("activate initial = %t/%v", published, err)
	}
	initialCatalog := subscriber.Catalog()

	duplicate := generation.Copy()
	duplicate.Manifest.GenerationID = "generation-two"
	duplicate.Manifest.GeneratedAt = generation.Manifest.GeneratedAt.Add(time.Minute)
	if err := duplicate.Validate(); err != nil {
		t.Fatalf("duplicate fixture: %v", err)
	}
	if published, err := subscriber.activate(context.Background(), duplicate); err != nil ||
		published {
		t.Fatalf("activate duplicate payload = %t/%v", published, err)
	}
	if subscriber.Catalog() != initialCatalog {
		t.Fatal("identical payload under a newer identity copied the catalog")
	}
	if !subscriber.isActiveGeneration(duplicate.Manifest.GenerationID) {
		t.Fatal("deduplication state did not advance to the newer identity")
	}
}

func subscriberTestGeneration(
	t testing.TB,
	generationID string,
	providerID catalogs.ProviderID,
	generatedAt time.Time,
) catalogstore.Generation {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{
		ID: providerID, Name: string(providerID),
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	payload, err := catalogstore.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	descriptor := catalogs.DescribeCatalogPayload(payload)
	generation := catalogstore.Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.CurrentGenerationManifestVersion,
			SchemaVersion:   catalogs.CurrentCatalogSchemaVersion,
			GenerationID:    generationID,
			GeneratedAt:     generatedAt,
			Payload:         descriptor,
			Validation: catalogs.GenerationValidationReport{
				ValidatorVersion: "remote-subscriber-test/v1",
				ValidatedAt:      generatedAt,
				Status:           catalogs.GenerationValidationPassed,
				Checks: []catalogs.GenerationValidationCheck{{
					Name:   "test",
					Status: catalogs.GenerationValidationCheckPassed,
				}},
			},
			SyncRunID: "sync-" + generationID,
			SourceObservations: []catalogs.SourceObservationLink{{
				Source:        catalogmeta.LocalCatalogID,
				ObservationID: "observation-" + generationID,
				ObservedAt:    generatedAt,
				Revision: catalogmeta.ObservationRevision{
					Kind:  catalogmeta.ObservationRevisionKindContentDigest,
					Value: descriptor.Checksum,
				},
				Completeness:     catalogmeta.ObservationCompletenessComplete,
				Status:           catalogmeta.ObservationStatusSucceeded,
				EvidenceChecksum: descriptor.Checksum,
			}},
			Completeness: catalogs.GenerationCompletenessComplete,
			ConsumerCompatibility: catalogs.ConsumerCompatibility{
				MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
				MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
			},
		},
		Payload: payload,
	}
	if err := generation.Validate(); err != nil {
		t.Fatalf("generation fixture: %v", err)
	}
	return generation
}

func writeSubscriberManifest(
	t testing.TB,
	writer http.ResponseWriter,
	generation catalogstore.Generation,
) {
	t.Helper()
	data, err := catalogremote.MarshalManifest(generation.Manifest)
	if err != nil {
		t.Fatalf("MarshalManifest: %v", err)
	}
	writer.Header().Set("Content-Type", catalogremote.ManifestMediaType)
	_, _ = writer.Write(data)
}

func waitForSubscriberCondition(t testing.TB, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for subscriber condition")
}
