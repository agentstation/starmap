package config_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

const acquisitionSourcesSetting = "STARMAP_CATALOG_ACQUISITION_SOURCES"

type configuredSourceAcquirer struct{ calls atomic.Int32 }

func (a *configuredSourceAcquirer) AcquireSources(context.Context, runtime.SourceAcquisitionRequest) ([]sources.Observation, error) {
	a.calls.Add(1)
	return nil, nil
}

func TestExplicitAcquisitionSourceSelectionPreventsDisabledReads(t *testing.T) {
	settings, err := config.Parse(map[string]string{acquisitionSourcesSetting: "providers"})
	if err != nil {
		t.Fatal(err)
	}
	acquirer := &configuredSourceAcquirer{}
	options := []runtime.Option{runtime.WithCatalogSource("embedded"), runtime.WithAcquisitionEnabled(false), runtime.WithSourceAcquirer(acquirer), runtime.WithClientOptions(starmap.WithCatalogStore(storage.NewMemory()))}
	options = append(options, settings.Options()...)
	connected, err := runtime.Open(t.Context(), options...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := connected.Close(); err != nil {
			t.Error(err)
		}
	}()
	// No provider role exists, so explicit sync can report that no input is eligible.
	_, _ = connected.Sync(t.Context())
	if acquirer.calls.Load() != 0 {
		t.Fatal("explicit source selection read disabled metadata")
	}
}

func TestAcquisitionSourceSettingPresenceAndValidation(t *testing.T) {
	for _, test := range []struct {
		name, value    string
		present, valid bool
	}{
		{name: "omitted", valid: true},
		{name: "empty", present: true, valid: true},
		{name: "providers", value: string(sources.ProvidersID), present: true, valid: true},
		{name: "unknown", value: "unknown", present: true},
		{name: "distribution", value: string(sources.EmbeddedCatalogID), present: true},
		{name: "duplicate", value: string(sources.LocalCatalogID) + "," + string(sources.LocalCatalogID), present: true},
		{name: "both_forms", value: string(sources.ModelsDevHTTPID) + "," + string(sources.ModelsDevGitID), present: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := map[string]string{}
			if test.present {
				values[acquisitionSourcesSetting] = test.value
			}
			settings, err := config.Parse(values)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%t, error=%v", test.valid, err)
			}
			if err != nil {
				return
			}
			selected, present := settings.AcquisitionSourceSelection()
			if present != test.present {
				t.Fatal("lost setting presence")
			}
			if len(selected) != 0 {
				selected[0] = sources.LocalCatalogID
				again, _ := settings.AcquisitionSourceSelection()
				if again[0] != sources.ProvidersID {
					t.Fatal("setting returned shared source storage")
				}
			}
		})
	}
}
