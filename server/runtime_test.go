package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/catalog/settings"
	"github.com/agentstation/starmap/internal/test/channel"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
	"github.com/agentstation/starmap/server"
)

// shutdownBound is the grace period that the application gives a shutdown.
const shutdownBound = 5 * time.Second

// TestServerServesEmbeddedStateThenPullsChannel proves the server application
// rules. The server answers before the first upstream reply. Readiness reports
// the runtime status. The runtime pulls a synthetic channel from a local test
// server. Acquisition observes only the provider that holds a credential.
// Shutdown joins the runtime inside the five-second bound, and the durable
// state survives the outage.
func TestServerServesEmbeddedStateThenPullsChannel(t *testing.T) {
	t.Setenv(channel.ConfiguredEnvironment, "test-key")

	upstream, err := channel.Start()
	if err != nil {
		t.Fatalf("channel.Start: %v", err)
	}
	t.Cleanup(upstream.Close)
	statePath := t.TempDir()
	storePath := t.TempDir()

	connected := openRuntime(t, upstream, statePath, storePath)
	srv := newServer(t, connected)
	endpoint := httptest.NewServer(srv.Handler())
	t.Cleanup(endpoint.Close)

	// The server answers with the verified embedded state before any upstream
	// reply, so the listener never waits for the network.
	if upstream.ChannelReads() != 0 {
		t.Fatal("the server read the channel before it served a request")
	}
	assertStatus(t, endpoint.URL+"/health", http.StatusOK)

	// Readiness reports the connected runtime status.
	assertRuntimeReadiness(t, endpoint.URL+"/api/v1/ready", connected.Status())

	// The runtime pulls the synthetic channel in the foreground of this test.
	report, err := connected.RefreshSource(context.Background())
	if err != nil {
		t.Fatalf("RefreshSource: %v", err)
	}
	if !report.Changed {
		t.Fatal("the first source read reported no change")
	}
	assertStatus(t, endpoint.URL+"/api/v1/providers/"+string(channel.ConfiguredProvider),
		http.StatusOK)

	// Acquisition observes only the provider whose credential resolves.
	acquired, err := connected.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	assertEligibility(t, acquired.Attempts, upstream)

	// The source goes away and the server stops. Shutdown joins the runtime
	// inside the application grace period.
	upstream.Close()
	endpoint.Close()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownBound)
	defer cancel()
	started := time.Now()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if elapsed := time.Since(started); elapsed >= shutdownBound {
		t.Fatalf("shutdown took %s, want less than %s", elapsed, shutdownBound)
	}
	assertRuntimeClosed(t, connected)
	if err := connected.Close(); err != nil {
		t.Fatalf("Close after shutdown: %v", err)
	}

	// A restarted server serves the retained catalog with no reachable source.
	restarted := openRuntime(t, upstream, statePath, storePath)
	t.Cleanup(func() {
		if err := restarted.Close(); err != nil {
			t.Errorf("Close after restart: %v", err)
		}
	})
	restartedServer := newServer(t, restarted)
	restartedEndpoint := httptest.NewServer(restartedServer.Handler())
	t.Cleanup(restartedEndpoint.Close)
	assertStatus(t,
		restartedEndpoint.URL+"/api/v1/providers/"+string(channel.ConfiguredProvider),
		http.StatusOK)
}

// newServer composes the embeddable server the way the serve command does.
func newServer(t *testing.T, connected *runtime.Runtime) *server.Server {
	t.Helper()
	syncer, err := acquisition.New(connected.Client())
	if err != nil {
		t.Fatalf("acquisition.New: %v", err)
	}
	srv, err := server.New(
		connected.Client(),
		server.DefaultConfig(),
		server.WithRuntime(connected),
		server.WithSyncer(syncer),
	)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return srv
}

// openRuntime composes the runtime from the canonical catalog settings.
func openRuntime(
	t *testing.T,
	upstream *channel.Upstream,
	statePath, storePath string,
) *runtime.Runtime {
	t.Helper()
	values := map[string]string{
		settings.Source:              string(runtime.SourceEmbedded),
		settings.AcquisitionEnabled:  "false",
		settings.SourcePollInterval:  "1h",
		settings.StateDirectory:      statePath,
		settings.CoalesceWindow:      "10ms",
		settings.TransferMaxDuration: "5s",
	}
	config, err := settings.Load(func(name string) (string, bool) {
		value, found := values[name]
		return value, found
	})
	if err != nil {
		t.Fatalf("settings.Load: %v", err)
	}
	store, err := storage.NewFilesystem(storePath)
	if err != nil {
		t.Fatalf("storage.NewFilesystem: %v", err)
	}
	acquirer, err := acquisition.NewAcquirer(
		acquisition.WithAcquirerCredentialResolver(auth.NewResolver()),
		acquisition.WithAcquirerCoalesceWindow(10*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("acquisition.NewAcquirer: %v", err)
	}
	connected, err := settings.Composition{
		Config:   config,
		Source:   upstream,
		Acquirer: acquirer,
		Base: []runtime.Option{
			runtime.WithClientOptions(
				starmap.WithCatalogStore(store),
				starmap.WithCatalogPath(filepath.Join(t.TempDir(), "workspace")),
			),
		},
	}.Open(context.Background())
	if err != nil {
		t.Fatalf("Composition.Open: %v", err)
	}
	return connected
}

// assertStatus reads one endpoint and compares the status code.
func assertStatus(t *testing.T, url string, want int) {
	t.Helper()
	request, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request for %s: %v", url, err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != want {
		t.Fatalf("GET %s status = %d, want %d", url, response.StatusCode, want)
	}
}

// assertRuntimeReadiness proves that the readiness body reports the runtime.
func assertRuntimeReadiness(t *testing.T, url string, status runtime.Status) {
	t.Helper()
	request, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build readiness request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("readiness status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var body struct {
		Data struct {
			Runtime struct {
				Usable       bool   `json:"usable"`
				SourceKind   string `json:"source_kind"`
				GenerationID string `json:"generation_id"`
			} `json:"runtime"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode readiness body: %v", err)
	}
	if !body.Data.Runtime.Usable {
		t.Fatal("readiness reports an unusable runtime")
	}
	if body.Data.Runtime.SourceKind != string(status.SourceKind) {
		t.Fatalf("readiness source kind = %q, want %q",
			body.Data.Runtime.SourceKind, status.SourceKind)
	}
	if body.Data.Runtime.GenerationID != status.GenerationID {
		t.Fatalf("readiness generation = %q, want %q",
			body.Data.Runtime.GenerationID, status.GenerationID)
	}
}

// assertEligibility proves that only the provider with a credential answered.
func assertEligibility(
	t *testing.T,
	attempts []sources.ProviderAttempt,
	upstream *channel.Upstream,
) {
	t.Helper()
	byProvider := make(map[catalogs.ProviderID]sources.ProviderAttempt, len(attempts))
	for _, attempt := range attempts {
		byProvider[attempt.ProviderID] = attempt
	}
	configured := byProvider[channel.ConfiguredProvider]
	if configured.Outcome != sources.ProviderOutcomeSucceeded || !configured.Requested {
		t.Fatalf("configured attempt = %+v, want a requested success", configured)
	}
	unconfigured := byProvider[channel.UnconfiguredProvider]
	if unconfigured.Outcome != sources.ProviderOutcomeSkippedNotConfigured {
		t.Fatalf("unconfigured outcome = %q, want %q",
			unconfigured.Outcome, sources.ProviderOutcomeSkippedNotConfigured)
	}
	if unconfigured.Reason != sources.ProviderReasonCredentialUnavailable {
		t.Fatalf("unconfigured reason = %q, want %q",
			unconfigured.Reason, sources.ProviderReasonCredentialUnavailable)
	}
	if unconfigured.Requested {
		t.Fatal("a provider without a credential sent a request")
	}
	if requests := upstream.ModelReads(); requests != 1 {
		t.Fatalf("provider model requests = %d, want 1", requests)
	}
}

// assertRuntimeClosed proves that the server shutdown joined the runtime. Close
// owns the publication channel, so a closed channel proves the join.
func assertRuntimeClosed(t *testing.T, connected *runtime.Runtime) {
	t.Helper()
	updates := connected.Updates()
	for {
		select {
		case _, open := <-updates:
			if !open {
				return
			}
		default:
			t.Fatal("the server shutdown did not close the runtime")
		}
	}
}
