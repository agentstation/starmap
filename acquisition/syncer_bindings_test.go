package acquisition

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestManualBindingOptionCopiesAndValidatesWithoutAcquisition(t *testing.T) {
	_, binding := acquisitionBindingFixture(t)
	bindings := []sources.ProviderAcquisitionBinding{binding}
	option := WithProviderBindings(bindings...)
	bindings[0].ID = "changed"
	configured := defaults()
	if err := option(&configured); err != nil {
		t.Fatal(err)
	}
	if configured.providerBindings == nil || (*configured.providerBindings)[0] != binding {
		t.Fatal("option borrowed declaration storage")
	}
	(*configured.providerBindings)[0].Revision = "changed"
	another := defaults()
	if err := option(&another); err != nil {
		t.Fatal(err)
	}
	if (*another.providerBindings)[0] != binding {
		t.Fatal("reused option borrowed earlier settings")
	}
	empty := defaults()
	if err := WithProviderBindings()(&empty); err != nil || empty.providerBindings == nil || len(*empty.providerBindings) != 0 {
		t.Fatal("explicit empty selection was lost")
	}
	for _, invalid := range [][]sources.ProviderAcquisitionBinding{{{}}, {binding, binding}} {
		if err := WithProviderBindings(invalid...)(&empty); err == nil {
			t.Fatal("invalid binding option accepted")
		}
	}
}

func TestManualSyncBindingReceiptsReachDurableGeneration(t *testing.T) {
	current, binding := acquisitionBindingFixture(t)
	builder, err := catalogs.NewBuilderFrom(current)
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := builder.Provider(binding.ProviderID)
	provider.Catalog.Endpoint.ProtocolOptions = testcatalog.OpenAIProtocolOptions()
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	path := t.TempDir()
	if err := builder.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	store := storage.NewMemory()
	client, err := starmap.New(starmap.WithCatalogStore(store), starmap.WithCatalogPath(path))
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	factory := WithProviderClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
		calls.Add(1)
		return receiptProviderClient{}, nil
	})
	resolver := WithCredentialResolver(sources.ProviderCredentialResolverFunc(func(_ context.Context, p *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
		return sources.NewProviderCredentialMaterial(p.Credentials.Profiles[0], nil, sources.ProviderCredentialMetadata{}), nil
	}))
	second := binding
	second.ID = "peer"
	syncer, err := New(client, factory, resolver, WithProviderBindings(binding, second))
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("constructor acquired provider models")
	}
	before := client.Catalog()
	options := []pkgsync.Option{pkgsync.WithCatalogPath(path), pkgsync.WithSources(sources.ProvidersID), pkgsync.WithProvider(binding.ProviderID)}
	preview, err := syncer.Sync(t.Context(), append(options, pkgsync.WithDryRun(true))...)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || !preview.DryRun || preview.GenerationID != "" || before != client.Catalog() {
		t.Fatal("preview did not preserve publication boundary")
	}
	if _, err := store.Current(t.Context()); err == nil {
		t.Fatal("preview wrote the store")
	}
	result, err := syncer.Sync(t.Context(), options...)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 4 || result.GenerationID == "" {
		t.Fatal("manual publication was not completed")
	}
	generation, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	observed := map[string]bool{}
	for _, link := range generation.Manifest.SourceObservations {
		if link.Source == sources.ProvidersID {
			observed[link.ObservationID] = true
		}
	}
	if len(observed) != 2 {
		t.Fatal("durable generation collapsed binding receipts")
	}
	if result.GenerationID != generation.Manifest.GenerationID {
		t.Fatal("published generation does not match result")
	}
	disabled, err := New(client, factory, resolver, WithProviderBindings())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := disabled.Sync(t.Context(), pkgsync.WithDryRun(true), pkgsync.WithSources(sources.ProvidersID)); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 4 {
		t.Fatal("empty binding selection used ambient provider discovery")
	}
}
