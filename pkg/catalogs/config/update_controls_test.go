package config_test

import (
	"context"
	"maps"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

func TestOfflineConfigurationStopsExplicitCatalogNetwork(t *testing.T) {
	for _, operation := range []string{"source", "acquisition", "refresh", "manual-update", "preview", "observations"} {
		t.Run(operation, func(t *testing.T) {
			var requests atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(server.Close)
			probe := &controlNetworkProbe{client: server.Client(), address: server.URL}
			connected := openControlRuntime(t, map[string]string{
				"STARMAP_CATALOG_NETWORK_MODE": "offline",
			}, runtime.WithSource(probe), runtime.WithAcquirer(probe))

			var err error
			switch operation {
			case "source":
				_, err = connected.RefreshSource(t.Context())
			case "acquisition":
				_, err = connected.Sync(t.Context())
			case "refresh":
				_, err = connected.Refresh(t.Context())
			case "manual-update":
				_, err = connected.UpdateAcquisition(t.Context(), func(ctx context.Context, _ runtime.ObservationInputs) (runtime.ObservationUpdate, error) {
					_, err := probe.Read(ctx)
					return runtime.ObservationUpdate{}, err
				})
			case "preview":
				_, err = connected.PreviewAcquisition(t.Context(), func(ctx context.Context, _ runtime.ObservationInputs) (runtime.ObservationUpdate, error) {
					_, err := probe.Read(ctx)
					return runtime.ObservationUpdate{}, err
				})
			case "observations":
				_, err = connected.UpdateObservations(t.Context(), func(ctx context.Context, _ runtime.ObservationInputs) ([]sources.Observation, error) {
					_, err := probe.Read(ctx)
					return nil, err
				})
			}
			if err == nil {
				t.Error("offline catalog operation returned no refusal")
			}
			if got := requests.Load(); got != 0 {
				t.Fatalf("offline catalog operation sent %d HTTP requests", got)
			}
		})
	}
}

func TestManualSourceConfigurationDoesNotSubscribe(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	probe := &controlWatcherProbe{
		controlNetworkProbe: controlNetworkProbe{client: server.Client(), address: server.URL},
		changes:             make(chan struct{}, 1),
	}
	connected := openControlRuntime(t, map[string]string{
		"STARMAP_CATALOG_SOURCE_REFRESH_MODE": "manual",
	}, runtime.WithSource(probe))
	if got := probe.subscriptions.Load(); got != 0 {
		t.Fatalf("manual source mode subscribed to %d watcher channels", got)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("manual source mode sent %d startup requests", got)
	}
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatalf("explicit manual source refresh: %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("explicit manual source refresh sent %d requests, want one", got)
	}
}

func openControlRuntime(t *testing.T, overrides map[string]string, extra ...runtime.Option) *runtime.Runtime {
	t.Helper()
	values := map[string]string{
		config.Source:             "public",
		config.SourcePollInterval: "0s",
		config.AcquisitionEnabled: "false",
		config.StateDirectory:     filepath.Join(t.TempDir(), "runtime"),
	}
	maps.Copy(values, overrides)
	parsed, err := config.Load(func(name string) (string, bool) {
		value, present := values[name]
		return value, present
	})
	if err != nil {
		t.Fatal(err)
	}
	options := append(parsed.Options(), runtime.WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())))
	options = append(options, extra...)
	connected, err := runtime.Open(t.Context(), options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := connected.Close(); err != nil {
			t.Error(err)
		}
	})
	return connected
}

type controlNetworkProbe struct {
	client  *http.Client
	address string
}

func (*controlNetworkProbe) Identity() string { return "control-probe" }

func (p *controlNetworkProbe) Read(ctx context.Context) (runtime.SourceRead, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.address, nil)
	if err != nil {
		return runtime.SourceRead{}, err
	}
	response, err := p.client.Do(request)
	if err != nil {
		return runtime.SourceRead{}, err
	}
	return runtime.SourceRead{Health: runtime.HealthOK}, response.Body.Close()
}

func (p *controlNetworkProbe) AcquireProviders(ctx context.Context, _ runtime.AcquisitionRequest) (runtime.AcquisitionResult, error) {
	_, err := p.Read(ctx)
	return runtime.AcquisitionResult{}, err
}

type controlWatcherProbe struct {
	controlNetworkProbe
	subscriptions atomic.Int64
	changes       chan struct{}
}

func (p *controlWatcherProbe) Changes() <-chan struct{} {
	p.subscriptions.Add(1)
	return p.changes
}

func (p *controlWatcherProbe) ReadOnce(ctx context.Context) (runtime.SourceRead, error) {
	return p.controlNetworkProbe.Read(ctx)
}
