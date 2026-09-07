package acquisition

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/agentstation/starmap/runtime"
)

func TestRuntimeAcquisitionFreshPreservesBaselineAndRestart(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "runtime")
	options := []runtime.Option{runtime.WithStateDirectory(directory), runtime.WithSourcePollInterval(0), runtime.WithAcquisitionEnabled(false), runtime.WithClientOptions(starmap.WithCatalogStore(storage.NewMemory()))}
	connected, err := runtime.Open(t.Context(), options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connected.Close() })
	baseline := connected.State()
	baselineProvider, err := baseline.Catalog.Provider("openai")
	if err != nil {
		t.Fatal(err)
	}
	baselineModel, ok := baselineProvider.Models["gpt-4o-mini"]
	if !ok || baselineModel.Limits == nil {
		t.Fatal("embedded model has no limits")
	}
	observed := catalogs.Model{ID: "gpt-4o-mini", Name: "GPT-4o mini", Limits: &catalogs.ModelLimits{ContextWindow: 9999}}
	syncer, err := NewForRuntime(connected,
		WithProviderClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
			return batchClientFunc(func(context.Context, sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
				return []catalogs.Model{observed}, nil
			}), nil
		}),
		WithCredentialResolver(sources.ProviderCredentialResolverFunc(func(_ context.Context, p *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
			return sources.NewProviderCredentialMaterial(p.Credentials.Profiles[0], nil, sources.ProviderCredentialMetadata{}), nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}
	selected := []pkgsync.Option{pkgsync.WithSources(sources.ProvidersID), pkgsync.WithProvider("openai")}
	if _, err := syncer.Sync(t.Context(), selected...); err != nil {
		t.Fatal(err)
	}
	acquired := connected.State()
	provider, _ := acquired.Catalog.Provider("openai")
	if provider.Models[observed.ID].Limits.ContextWindow != 9999 {
		t.Fatal("acquisition did not replace embedded limit")
	}
	observed.Limits = nil
	fresh := append(selected, pkgsync.WithFresh(true))
	preview, err := syncer.Sync(t.Context(), append(fresh, pkgsync.WithDryRun(true))...)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.HasChanges() || preview.ResetCount != 1 || connected.State().GenerationID != acquired.GenerationID {
		t.Fatal("preview did not expose reset without activation")
	}
	result, err := syncer.Sync(t.Context(), fresh...)
	if err != nil {
		t.Fatal(err)
	}
	reset := connected.State()
	provider, _ = reset.Catalog.Provider("openai")
	if provider.Models[observed.ID].Limits.ContextWindow != baselineModel.Limits.ContextWindow {
		t.Fatalf("reset limit = %d, baseline = %d", provider.Models[observed.ID].Limits.ContextWindow, baselineModel.Limits.ContextWindow)
	}
	if result.GenerationID != reset.GenerationID || reset.GenerationID == acquired.GenerationID {
		t.Fatal("reset did not activate a new generation")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := runtime.Open(t.Context(), options...)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.State().GenerationID != reset.GenerationID || reopened.State().PayloadChecksum != reset.PayloadChecksum {
		t.Fatal("restart changed reset result")
	}
}
