package config_test

import (
	"context"
	stderrors "errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime"
)

type scheduledControlProbe struct {
	reads         atomic.Int64
	acquisitions  atomic.Int64
	subscriptions atomic.Int64
	changes       chan struct{}
}

func (*scheduledControlProbe) Identity() string { return "scheduled-control-probe" }

func (p *scheduledControlProbe) Read(context.Context) (runtime.SourceRead, error) {
	p.reads.Add(1)
	return runtime.SourceRead{Health: runtime.HealthOK}, nil
}

func (p *scheduledControlProbe) ReadOnce(ctx context.Context) (runtime.SourceRead, error) {
	return p.Read(ctx)
}

func (p *scheduledControlProbe) AcquireProviders(context.Context, runtime.AcquisitionRequest) (runtime.AcquisitionResult, error) {
	p.acquisitions.Add(1)
	return runtime.AcquisitionResult{}, nil
}

func (p *scheduledControlProbe) Changes() <-chan struct{} {
	p.subscriptions.Add(1)
	return p.changes
}

func TestAutomaticConfigurationRunsScheduledWorkAcrossRestart(t *testing.T) {
	testConfiguredBackgroundControls(t, "automatic")
}

func TestManualSourceConfigurationKeepsAcquisitionAcrossRestart(t *testing.T) {
	testConfiguredBackgroundControls(t, "manual")
}

func TestOfflineConfigurationGuardsScheduledWorkAcrossRestart(t *testing.T) {
	testConfiguredBackgroundControls(t, "offline")
}

func TestPinnedConfigurationBlocksPollingAcrossRestart(t *testing.T) {
	testConfiguredBackgroundControls(t, "pinned")
}

func testConfiguredBackgroundControls(t *testing.T, mode string) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		store := storage.NewMemory()
		selected, err := starmap.EmbeddedGeneration()
		if err != nil {
			t.Fatal(err)
		}
		selected.Manifest.GenerationID = "scheduled-pin-selected"
		if err := store.Commit(t.Context(), selected, ""); err != nil {
			t.Fatal(err)
		}
		newer := selected.Copy()
		newer.Manifest.GenerationID = "scheduled-pin-newer"
		if err := store.Commit(t.Context(), newer, selected.Manifest.GenerationID); err != nil {
			t.Fatal(err)
		}
		values := map[string]string{
			config.Source: "public", config.SourceRefreshMode: "automatic",
			config.SourcePollInterval: "10ms", config.AcquisitionInterval: "10ms",
			config.AcquisitionEnabled: "true", config.StateDirectory: filepath.Join(t.TempDir(), "runtime"),
		}
		switch mode {
		case "manual":
			values[config.SourceRefreshMode] = "manual"
		case "offline":
			values[config.NetworkMode] = "offline"
		case "pinned":
			values[config.GenerationPin] = selected.Manifest.GenerationID
		}
		for attempt := range 2 {
			parsed, err := config.Load(func(name string) (string, bool) { value, present := values[name]; return value, present })
			if err != nil {
				t.Fatal(err)
			}
			probe := &scheduledControlProbe{changes: make(chan struct{}, 1)}
			options := append(parsed.Options(), runtime.WithSource(probe), runtime.WithAcquirer(probe),
				runtime.WithStartupSpread(0), runtime.WithCoalesceWindow(time.Millisecond),
				runtime.WithClientOptions(starmap.WithCatalogStore(store)))
			connected, err := runtime.Open(t.Context(), options...)
			if err != nil {
				t.Fatal(err)
			}
			func() {
				defer func() {
					if err := connected.Close(); err != nil {
						t.Error(err)
					}
				}()
				before := connected.State().GenerationID
				probe.changes <- struct{}{}
				synctest.Wait()
				time.Sleep(50 * time.Millisecond)
				synctest.Wait()
				reads, acquisitions := probe.reads.Load(), probe.acquisitions.Load()
				if mode == "automatic" {
					if reads < 2 || acquisitions < 2 || probe.subscriptions.Load() != 1 {
						t.Fatalf("attempt %d did not exercise source and acquisition timers: reads=%d acquisitions=%d subscriptions=%d", attempt, reads, acquisitions, probe.subscriptions.Load())
					}
					return
				}
				if reads != 0 || probe.subscriptions.Load() != 0 || len(probe.changes) != 1 {
					t.Fatalf("attempt %d bypassed source control: reads=%d subscriptions=%d queued=%d", attempt, reads, probe.subscriptions.Load(), len(probe.changes))
				}
				if mode == "manual" {
					if acquisitions < 2 {
						t.Fatal("manual source mode disabled independent scheduled acquisition")
					}
					if _, err := connected.RefreshSource(t.Context()); err != nil || probe.reads.Load() != 1 {
						t.Fatalf("manual source mode refused explicit refresh: %v", err)
					}
					return
				}
				if acquisitions != 0 {
					t.Fatalf("%s allowed %d automatic acquisition calls", mode, acquisitions)
				}
				_, err := connected.RefreshSource(t.Context())
				assertConfiguredControlRefusal(t, mode, err)
				_, err = connected.Sync(t.Context())
				assertConfiguredControlRefusal(t, mode, err)
				if connected.State().GenerationID != before || (mode == "pinned" && before != selected.Manifest.GenerationID) {
					t.Fatal("scheduled or explicit work replaced the selected generation")
				}
			}()
		}
	})
}

func assertConfiguredControlRefusal(t *testing.T, mode string, err error) {
	t.Helper()
	if mode == "pinned" {
		if !errors.IsConflict(err) {
			t.Fatalf("pin did not return a conflict: %v", err)
		}
		return
	}
	var configuration *errors.ConfigError
	if !stderrors.As(err, &configuration) || configuration.Component != "catalog network policy" {
		t.Fatalf("offline operation did not return its configuration refusal: %v", err)
	}
}
