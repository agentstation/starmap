package remote

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogremote"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestNewRequiresCallerOwnedStoreBeforeRemoteWork(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()

	var typedNil *catalogstore.Memory
	for name, store := range map[string]catalogstore.Store{
		"nil interface": nil,
		"typed nil":     typedNil,
	} {
		t.Run(name, func(t *testing.T) {
			subscriber, err := New(Config{BaseURL: server.URL, CatalogStore: store})
			if subscriber != nil {
				t.Fatal("New returned a subscriber without a caller-owned store")
			}
			var configErr *pkgerrors.ConfigError
			if !stderrors.As(err, &configErr) || configErr.Component != "catalog store" {
				t.Fatalf("New error = %T: %v, want catalog-store ConfigError", err, err)
			}
		})
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("constructor remote requests = %d, want 0", got)
	}
}

func TestPinnedBootstrapSeedsOnlyAnEmptyCallerStore(t *testing.T) {
	t.Parallel()

	pinned := subscriberTestGeneration(
		t,
		"generation-pinned",
		"provider-pinned",
		time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC),
	)
	store := catalogstore.NewMemory()
	subscriber, err := New(Config{
		BaseURL:         "https://starmap.invalid",
		CatalogStore:    store,
		PinnedBootstrap: &pinned,
	})
	if err != nil {
		t.Fatalf("New pinned bootstrap: %v", err)
	}
	state := subscriber.State()
	if state.GenerationID != pinned.Manifest.GenerationID ||
		state.PayloadChecksum != pinned.Manifest.Payload.Checksum ||
		!state.GeneratedAt.Equal(pinned.Manifest.GeneratedAt) {
		t.Fatalf("pinned state = %#v, want manifest %#v", state, pinned.Manifest)
	}
	if _, err := state.Catalog.Provider("provider-pinned"); err != nil {
		t.Fatalf("pinned provider: %v", err)
	}
	durable, err := store.Current(t.Context())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if durable.Manifest.GenerationID != pinned.Manifest.GenerationID {
		t.Fatalf("durable generation = %q, want %q", durable.Manifest.GenerationID, pinned.Manifest.GenerationID)
	}
}

func TestEmbeddedEquivalentRemoteGenerationBecomesDurable(t *testing.T) {
	t.Parallel()

	source, err := starmap.New()
	if err != nil {
		t.Fatalf("New source: %v", err)
	}
	generation, err := source.CurrentGeneration(t.Context())
	if err != nil {
		t.Fatalf("CurrentGeneration: %v", err)
	}
	store := catalogstore.NewMemory()
	subscriber, err := New(Config{
		BaseURL: "https://starmap.invalid", CatalogStore: store,
	})
	if err != nil {
		t.Fatalf("New subscriber: %v", err)
	}
	initial := subscriber.State()
	published, err := subscriber.activate(t.Context(), generation)
	if err != nil {
		t.Fatalf("activate embedded-equivalent generation: %v", err)
	}
	if published {
		t.Fatal("embedded-equivalent generation reported a catalog publication")
	}
	state := subscriber.State()
	if state.Catalog != initial.Catalog ||
		state.GenerationID != generation.Manifest.GenerationID ||
		state.PayloadChecksum != generation.Manifest.Payload.Checksum {
		t.Fatalf("atomic state = %#v, want manifest %#v", state, generation.Manifest)
	}
	durable, err := store.Current(t.Context())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if durable.Manifest.GenerationID != generation.Manifest.GenerationID {
		t.Fatalf("durable generation = %q, want %q", durable.Manifest.GenerationID, generation.Manifest.GenerationID)
	}
}

func TestDurableCurrentWinsOverPinnedBootstrap(t *testing.T) {
	t.Parallel()

	durable := subscriberTestGeneration(
		t,
		"generation-durable",
		"provider-durable",
		time.Date(2026, time.August, 1, 13, 0, 0, 0, time.UTC),
	)
	pinned := subscriberTestGeneration(
		t,
		"generation-pin-not-selected",
		"provider-pin-not-selected",
		durable.Manifest.GeneratedAt.Add(time.Minute),
	)
	store := catalogstore.NewMemory()
	if err := store.Commit(t.Context(), durable, ""); err != nil {
		t.Fatalf("Commit durable current: %v", err)
	}
	subscriber, err := New(Config{
		BaseURL:         "https://starmap.invalid",
		CatalogStore:    store,
		PinnedBootstrap: &pinned,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	state := subscriber.State()
	if state.GenerationID != durable.Manifest.GenerationID ||
		state.PayloadChecksum != durable.Manifest.Payload.Checksum {
		t.Fatalf("selected state = %#v, want durable manifest %#v", state, durable.Manifest)
	}
	if _, err := state.Catalog.Provider("provider-durable"); err != nil {
		t.Fatalf("durable provider: %v", err)
	}
	if _, err := state.Catalog.Provider("provider-pin-not-selected"); err == nil {
		t.Fatal("pinned bootstrap replaced the durable current generation")
	}
}

func TestRestartServesDurableGenerationDuringRemoteRecovery(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "catalog-store")
	store, err := catalogstore.NewFilesystem(root)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	generation := subscriberTestGeneration(
		t,
		"generation-restart",
		"provider-restart",
		time.Date(2026, time.August, 1, 14, 0, 0, 0, time.UTC),
	)
	seed, err := New(Config{
		BaseURL: "https://starmap.invalid", CatalogStore: store,
	})
	if err != nil {
		t.Fatalf("New seed subscriber: %v", err)
	}
	if _, err := seed.activate(t.Context(), generation); err != nil {
		t.Fatalf("activate seed generation: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("Close seed subscriber: %v", err)
	}

	reopened, err := catalogstore.NewFilesystem(root)
	if err != nil {
		t.Fatalf("reopen filesystem store: %v", err)
	}
	server := httptest.NewServer(http.NotFoundHandler())
	client := server.Client()
	baseURL := server.URL
	server.Close()
	subscriber, err := New(Config{
		BaseURL:           baseURL,
		HTTPClient:        client,
		CatalogStore:      reopened,
		ReconnectMinDelay: time.Second,
		ReconnectMaxDelay: time.Second,
		ShutdownTimeout:   time.Second,
	})
	if err != nil {
		t.Fatalf("New restart subscriber: %v", err)
	}
	state := subscriber.State()
	if state.GenerationID != generation.Manifest.GenerationID ||
		state.PayloadChecksum != generation.Manifest.Payload.Checksum {
		t.Fatalf("restart state = %#v, want manifest %#v", state, generation.Manifest)
	}
	if _, err := state.Catalog.Provider("provider-restart"); err != nil {
		t.Fatalf("restart provider: %v", err)
	}
	if err := subscriber.Start(t.Context()); err != nil {
		t.Fatalf("Start during remote outage: %v", err)
	}
	if health := subscriber.Health(); health.StreamState != StreamStateRetrying {
		t.Fatalf("outage health = %#v, want retrying", health)
	}
	if err := subscriber.Close(); err != nil {
		t.Fatalf("Close recovering subscriber: %v", err)
	}
}

func TestInitialRemoteFailureRecoversWithoutPollingFallback(t *testing.T) {
	t.Parallel()

	generation := subscriberTestGeneration(
		t,
		"generation-recovered",
		"provider-recovered",
		time.Date(2026, time.August, 1, 14, 30, 0, 0, time.UTC),
	)
	var manifestRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case catalogremote.ManifestPath:
				if manifestRequests.Add(1) == 1 {
					writer.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				writeSubscriberManifest(t, writer, generation)
			case catalogremote.PayloadPath(generation.Manifest.GenerationID):
				writer.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
				_, _ = writer.Write(generation.Payload)
			case catalogremote.EventStreamPath:
				writer.Header().Set("Content-Type", catalogremote.EventStreamMediaType)
				_, _ = writer.Write([]byte(": connected\n\n"))
				writer.(http.Flusher).Flush()
				<-request.Context().Done()
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()
	subscriber, err := New(Config{
		BaseURL:           server.URL,
		HTTPClient:        server.Client(),
		CatalogStore:      catalogstore.NewMemory(),
		ReconnectMinDelay: time.Millisecond,
		ReconnectMaxDelay: time.Millisecond,
		ShutdownTimeout:   time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	subscriber.retryDelay = func(int) time.Duration { return 0 }
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err := subscriber.Start(ctx); err != nil {
		t.Fatalf("Start degraded lifecycle: %v", err)
	}
	waitForSubscriberCondition(t, func() bool {
		state := subscriber.State()
		if state.GenerationID != generation.Manifest.GenerationID {
			return false
		}
		_, providerErr := state.Catalog.Provider("provider-recovered")
		return providerErr == nil && subscriber.Health().StreamState == StreamStateStreaming
	})
	if status := subscriber.PollingFallbackStatus(); status.Enabled ||
		status.Active || status.Polls != 0 {
		t.Fatalf("normal streaming recovery used polling: %#v", status)
	}
	if err := subscriber.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestDurableCatalogDoesNotMaskTerminalRemoteAuthentication(t *testing.T) {
	t.Parallel()

	generation := subscriberTestGeneration(
		t,
		"generation-auth-durable",
		"provider-auth-durable",
		time.Date(2026, time.August, 1, 15, 0, 0, 0, time.UTC),
	)
	store := catalogstore.NewMemory()
	if err := store.Commit(t.Context(), generation, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	subscriber, err := New(Config{
		BaseURL: server.URL, HTTPClient: server.Client(), CatalogStore: store,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = subscriber.Start(t.Context())
	var apiErr *pkgerrors.APIError
	if !stderrors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Start error = %T: %v, want unauthorized APIError", err, err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("terminal authentication requests = %d, want 1", got)
	}
}

func TestNewContextFailsClosedForStoreErrorsAndCancellation(t *testing.T) {
	t.Parallel()

	storeErr := &pkgerrors.IOError{
		Operation: "read", Path: "fault-store", Err: stderrors.New("unavailable"),
	}
	if subscriber, err := NewContext(t.Context(), Config{
		BaseURL: "https://starmap.invalid", CatalogStore: faultStore{err: storeErr},
	}); subscriber != nil || !stderrors.Is(err, storeErr) {
		t.Fatalf("NewContext store failure = (%v, %v), want nil and store error", subscriber, err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	subscriber, err := NewContext(ctx, Config{
		BaseURL: "https://starmap.invalid", CatalogStore: blockingStore{},
	})
	if subscriber != nil || !stderrors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("NewContext cancellation = (%v, %v), want deadline", subscriber, err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("canceled construction took %s", elapsed)
	}
}

func TestNewRejectsInvalidPinnedBootstrapBeforeRemoteWork(t *testing.T) {
	t.Parallel()

	invalid := subscriberTestGeneration(
		t,
		"generation-invalid-pin",
		"provider-invalid-pin",
		time.Date(2026, time.August, 1, 16, 0, 0, 0, time.UTC),
	)
	invalid.Payload[0] ^= 1
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	subscriber, err := New(Config{
		BaseURL: server.URL, CatalogStore: catalogstore.NewMemory(), PinnedBootstrap: &invalid,
	})
	if subscriber != nil || err == nil {
		t.Fatalf("New invalid pin = (%v, %v), want nil and validation error", subscriber, err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("invalid-pin remote requests = %d, want 0", got)
	}
}

type faultStore struct {
	err error
}

func (s faultStore) Current(context.Context) (catalogstore.Generation, error) {
	return catalogstore.Generation{}, s.err
}

func (s faultStore) Get(context.Context, string) (catalogstore.Generation, error) {
	return catalogstore.Generation{}, s.err
}

func (s faultStore) Commit(context.Context, catalogstore.Generation, string) error {
	return s.err
}

type blockingStore struct{}

func (blockingStore) Current(ctx context.Context) (catalogstore.Generation, error) {
	<-ctx.Done()
	return catalogstore.Generation{}, ctx.Err()
}

func (blockingStore) Get(ctx context.Context, _ string) (catalogstore.Generation, error) {
	<-ctx.Done()
	return catalogstore.Generation{}, ctx.Err()
}

func (blockingStore) Commit(ctx context.Context, _ catalogstore.Generation, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}
