package catalogdistribution

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogartifact"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
)

func TestHostedDistributionClientDefaultHTTPTimeout(t *testing.T) {
	client, err := NewClient("https://starmap.agentstation.ai", nil, catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.httpClient == http.DefaultClient || client.httpClient.Timeout != constants.DefaultHTTPTimeout {
		t.Fatalf("default HTTP client = %#v, want isolated timeout %s", client.httpClient, constants.DefaultHTTPTimeout)
	}
}

func TestHostedDistributionClientRejectsCrossOriginRedirect(t *testing.T) {
	var redirectedRequests atomic.Int32
	redirected := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectedRequests.Add(1)
	}))
	defer redirected.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", redirected.URL)
		writer.WriteHeader(http.StatusFound)
	}))
	defer origin.Close()

	client, err := NewClient(origin.URL, origin.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.FetchLatest(context.Background()); err == nil {
		t.Fatal("FetchLatest followed a cross-origin redirect")
	}
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("cross-origin requests = %d, want 0", got)
	}
}

func TestHostedDistributionVerifiedLatestPointerReturnsExactGeneration(t *testing.T) {
	published := hostedFixture(t)
	repository := NewMemoryRepository()
	if err := repository.Publish(published); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	promoteStableForTest(t, repository, published)
	handler, err := NewHandler(repository)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	got, err := client.FetchLatest(context.Background())
	if err != nil {
		t.Fatalf("FetchLatest: %v", err)
	}
	if got.Manifest.GenerationID != published.Generation.Manifest.GenerationID ||
		got.Manifest.Payload != published.Generation.Manifest.Payload || string(got.Payload) != string(published.Generation.Payload) {
		t.Fatalf("hosted generation mismatch: %#v", got.Manifest)
	}
}

func TestHostedDistributionRejectsCrossOriginLatestAsset(t *testing.T) {
	pointer := LatestPointer{
		Version: PointerVersion, Channel: ChannelStable, GenerationID: "generation", SchemaVersion: catalogs.CurrentCatalogSchemaVersion,
		Artifact:    AssetDescriptor{URL: "https://attacker.example/catalog.tar.gz", MediaType: catalogartifact.MediaType},
		Attestation: AssetDescriptor{URL: "/attestation", MediaType: "application/vnd.in-toto+json"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(pointer)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.FetchLatest(context.Background()); err == nil {
		t.Fatal("FetchLatest accepted a cross-origin artifact pointer")
	}
}

func TestHostedDistributionGenerationIdentityCannotBeRebound(t *testing.T) {
	first := hostedFixture(t)
	repository := NewMemoryRepository()
	if err := repository.Publish(first); err != nil {
		t.Fatalf("Publish first: %v", err)
	}
	secondGeneration := first.Generation.Copy()
	secondGeneration.Manifest.GeneratedAt = secondGeneration.Manifest.GeneratedAt.AddDate(0, 0, 1)
	secondArtifact, err := catalogartifact.Build(secondGeneration)
	if err != nil {
		t.Fatalf("Build second: %v", err)
	}
	if err := repository.Publish(PublishedGeneration{Generation: secondGeneration, Artifact: secondArtifact}); err == nil {
		t.Fatal("Publish rebound an immutable generation ID")
	}
}

func TestHostedDistributionRejectsCredentialScopedGenerationBeforeRepositoryWrite(t *testing.T) {
	published := hostedFixture(t)
	published.Generation.Manifest.SourceObservations[0].Metrics.Scope = catalogmeta.ObservationScopeCredentialScoped
	repository := NewMemoryRepository()
	if err := repository.Publish(published); err == nil {
		t.Fatal("Publish accepted credential-scoped generation")
	}
	if _, err := repository.Get(published.Generation.Manifest.GenerationID); err == nil {
		t.Fatal("rejected credential-scoped generation was retained")
	}
}

func TestHostedDistributionRejectsArtifactChecksumDrift(t *testing.T) {
	published := hostedFixture(t)
	repository := NewMemoryRepository()
	if err := repository.Publish(published); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	promoteStableForTest(t, repository, published)
	handler, err := NewHandler(repository)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == APIPrefix+"/"+published.Generation.Manifest.GenerationID {
			writer.Header().Set("Content-Type", catalogartifact.MediaType)
			_, _ = writer.Write([]byte("tampered artifact"))
			return
		}
		handler.ServeHTTP(writer, request)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.FetchLatest(context.Background()); err == nil {
		t.Fatal("FetchLatest accepted hosted artifact checksum drift")
	}
}

func TestDistributionNegotiationRequiresExactCurrentSchema(t *testing.T) {
	published := hostedFixture(t)
	repository := NewMemoryRepository()
	if err := repository.Publish(published); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	promoteStableForTest(t, repository, published)
	handler, err := NewHandler(repository)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	compatible, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient compatible: %v", err)
	}
	if _, err := compatible.FetchLatest(context.Background()); err != nil {
		t.Fatalf("exact-schema consumer rejected current payload: %v", err)
	}
	for _, schemaVersion := range []uint64{1, catalogs.CurrentCatalogSchemaVersion + 1} {
		if _, err := NewClient(server.URL, server.Client(), schemaVersion); err == nil {
			t.Fatalf("NewClient accepted schema %d", schemaVersion)
		}
	}

	typeOfPointer := reflect.TypeFor[LatestPointer]()
	for _, forbidden := range []string{"ConsumerCompatibility", "StarmapVersion", "StarportVersion", "BinaryVersion", "ReleaseVersion"} {
		if _, found := typeOfPointer.FieldByName(forbidden); found {
			t.Fatalf("latest pointer couples catalog compatibility to %s", forbidden)
		}
	}
}

func TestETagImmutableCacheAndRollbackRetainPriorGenerations(t *testing.T) {
	first := hostedFixture(t)
	secondGeneration := first.Generation.Copy()
	secondGeneration.Manifest.GenerationID = "embedded-bootstrap-second"
	secondArtifact, err := catalogartifact.Build(secondGeneration)
	if err != nil {
		t.Fatalf("Build second: %v", err)
	}
	second := PublishedGeneration{Generation: secondGeneration, Artifact: secondArtifact}
	repository := NewMemoryRepository()
	for _, published := range []PublishedGeneration{first, second} {
		if err := repository.Publish(published); err != nil {
			t.Fatalf("Publish %s: %v", published.Generation.Manifest.GenerationID, err)
		}
	}
	promoteStableForTest(t, repository, first)
	promoteStableForTest(t, repository, second)
	handler, err := NewHandler(repository)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	assertConditionalCache := func(path, wantCache string) {
		t.Helper()
		request, requestErr := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+path, nil)
		if requestErr != nil {
			t.Fatalf("NewRequest %s: %v", path, requestErr)
		}
		response, requestErr := server.Client().Do(request)
		if requestErr != nil {
			t.Fatalf("GET %s: %v", path, requestErr)
		}
		_ = response.Body.Close()
		etag := response.Header.Get("ETag")
		if response.StatusCode != http.StatusOK || etag == "" || response.Header.Get("Cache-Control") != wantCache {
			t.Fatalf("GET %s status/cache/etag = %d/%q/%q", path, response.StatusCode, response.Header.Get("Cache-Control"), etag)
		}
		request, requestErr = http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+path, nil)
		if requestErr != nil {
			t.Fatalf("NewRequest conditional %s: %v", path, requestErr)
		}
		request.Header.Set("If-None-Match", etag)
		response, requestErr = server.Client().Do(request)
		if requestErr != nil {
			t.Fatalf("conditional GET %s: %v", path, requestErr)
		}
		body, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusNotModified || len(body) != 0 {
			t.Fatalf("conditional GET %s status/body/error = %d/%d/%v", path, response.StatusCode, len(body), readErr)
		}
	}

	assertConditionalCache(fmt.Sprintf("%s/latest?schema_version=%d", APIPrefix, catalogs.CurrentCatalogSchemaVersion), LatestCacheControl)
	assertConditionalCache(APIPrefix+"/"+second.Generation.Manifest.GenerationID, ImmutableCacheControl)
	assertConditionalCache(APIPrefix+"/"+first.Generation.Manifest.GenerationID, ImmutableCacheControl)

	if err := repository.Rollback(ChannelStable, first.Generation.Manifest.GenerationID, "rollback fixture"); err != nil {
		t.Fatalf("Rollback first: %v", err)
	}
	client, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	rolledBack, err := client.FetchLatest(context.Background())
	if err != nil {
		t.Fatalf("FetchLatest after rollback: %v", err)
	}
	if rolledBack.Manifest.GenerationID != first.Generation.Manifest.GenerationID {
		t.Fatalf("rollback generation = %q, want %q", rolledBack.Manifest.GenerationID, first.Generation.Manifest.GenerationID)
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+APIPrefix+"/"+second.Generation.Manifest.GenerationID, nil)
	if err != nil {
		t.Fatalf("NewRequest retained second: %v", err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("GET retained second: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("retained second status = %d", response.StatusCode)
	}
}

func hostedFixture(t *testing.T) PublishedGeneration {
	t.Helper()
	generation, err := bootstrap.Generation()
	if err != nil {
		t.Fatalf("bootstrap.Generation: %v", err)
	}
	artifact, err := catalogartifact.Build(generation)
	if err != nil {
		t.Fatalf("catalogartifact.Build: %v", err)
	}
	return PublishedGeneration{Generation: generation, Artifact: artifact}
}

func promoteStableForTest(t *testing.T, repository *MemoryRepository, published PublishedGeneration) {
	t.Helper()
	observedAt := published.Generation.Manifest.GeneratedAt.Add(time.Minute)
	repository.now = func() time.Time { return observedAt }
	id := published.Generation.Manifest.GenerationID
	if err := repository.Promote(ChannelDev, id, nil); err != nil {
		t.Fatalf("Promote dev %s: %v", id, err)
	}
	if err := repository.Promote(ChannelCanary, id, nil); err != nil {
		t.Fatalf("Promote canary %s: %v", id, err)
	}
	probe := PromotionProbe{
		Channel: ChannelCanary, GenerationID: id, ArtifactChecksum: published.Artifact.Checksum,
		ObservedAt: observedAt, Latency: time.Millisecond, Available: true, Fresh: true,
	}
	if err := repository.Promote(ChannelStable, id, &probe); err != nil {
		t.Fatalf("Promote stable %s: %v", id, err)
	}
}

func TestDecodeStrictJSONRejectsDuplicateMembers(t *testing.T) {
	var destination struct {
		Version int `json:"version"`
	}
	if err := decodeStrictJSON([]byte(`{"version":1,"version":2}`), &destination); err == nil {
		t.Fatal("decodeStrictJSON accepted duplicate JSON member")
	}
}
