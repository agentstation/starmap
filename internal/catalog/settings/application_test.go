package settings_test

import (
	"context"
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
)

// TestApplicationPullsSyntheticChannelAndRetainsState composes the application
// runtime from the canonical settings and proves four application rules. It
// pulls a synthetic channel from a local test server. It observes the provider
// that holds a credential. It sends no request for the provider that holds
// none. It keeps the durable catalog across a source outage and a restart.
func TestApplicationPullsSyntheticChannelAndRetainsState(t *testing.T) {
	t.Setenv(channel.ConfiguredEnvironment, "test-key")

	upstream := startUpstream(t)
	statePath := t.TempDir()
	storePath := t.TempDir()

	runtime := openApplicationRuntime(t, upstream, statePath, storePath)

	// The runtime reads the synthetic channel from the local test server.
	report, err := runtime.RefreshSource(context.Background())
	if err != nil {
		t.Fatalf("RefreshSource: %v", err)
	}
	if !report.Changed {
		t.Fatal("the first source read reported no change")
	}
	if upstream.ChannelReads() == 0 {
		t.Fatal("the runtime read no channel from the local test server")
	}
	if _, found := runtime.Catalog().Providers().Get(channel.ConfiguredProvider); !found {
		t.Fatal("the effective catalog holds no provider from the channel")
	}

	// Acquisition observes only the provider whose credential resolves.
	acquired, err := runtime.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	assertCredentialEligibility(t, acquired.Attempts, upstream)

	// The source goes away and the process restarts. The durable state must
	// still serve the last effective catalog.
	if err := runtime.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	upstream.Close()

	restarted := openApplicationRuntime(t, upstream, statePath, storePath)
	t.Cleanup(func() {
		if err := restarted.Close(); err != nil {
			t.Errorf("Close after restart: %v", err)
		}
	})
	provider, found := restarted.Catalog().Providers().Get(channel.ConfiguredProvider)
	if !found {
		t.Fatal("the restarted runtime lost the channel catalog")
	}
	if _, held := provider.Models[channel.ObservedModel]; !held {
		t.Fatal("the restarted runtime lost the retained provider layer")
	}
}

// startUpstream starts the synthetic channel server for one test.
func startUpstream(t *testing.T) *channel.Upstream {
	t.Helper()
	upstream, err := channel.Start()
	if err != nil {
		t.Fatalf("channel.Start: %v", err)
	}
	t.Cleanup(upstream.Close)
	return upstream
}

// assertCredentialEligibility proves that the eligible provider answered and
// that the provider without a credential sent no request.
func assertCredentialEligibility(
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
	if configured.Outcome != sources.ProviderOutcomeSucceeded {
		t.Fatalf("configured outcome = %q, want %q",
			configured.Outcome, sources.ProviderOutcomeSucceeded)
	}
	if !configured.Requested {
		t.Fatal("the eligible provider reported no request")
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

	// Exactly one provider holds a credential, so the test server may answer
	// exactly one model request.
	if requests := upstream.ModelReads(); requests != 1 {
		t.Fatalf("provider model requests = %d, want 1", requests)
	}
}

// openApplicationRuntime composes the runtime the way the application does. It
// reads the canonical settings, injects the process roles, and opens.
func openApplicationRuntime(
	t *testing.T,
	upstream *channel.Upstream,
	statePath, storePath string,
) *starmap.Runtime {
	t.Helper()

	values := map[string]string{
		settings.Source:              string(starmap.SourceEmbedded),
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

	composition := settings.Composition{
		Config:   config,
		Source:   upstream,
		Acquirer: acquirer,
		Base: []starmap.Option{
			starmap.WithCatalogStore(store),
			starmap.WithCatalogPath(filepath.Join(t.TempDir(), "workspace")),
		},
	}
	runtime, err := composition.Open(context.Background())
	if err != nil {
		t.Fatalf("Composition.Open: %v", err)
	}
	return runtime
}
