package runtime

import (
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestUpstreamScopeReceiptsSurviveDerivativePublicationAndRestart(t *testing.T) {
	upstream, observation, expected, at := upstreamScopeFixture(t)
	for name, mutate := range map[string]func(*sourceLayer){
		"missing original manifest":      func(layer *sourceLayer) { layer.Manifest = nil },
		"wrong retained identity":        func(layer *sourceLayer) { layer.GenerationID = "other" },
		"wrong retained checksum":        func(layer *sourceLayer) { layer.Checksum = "other" },
		"missing original scope receipt": func(layer *sourceLayer) { layer.Manifest.SourceObservations = nil },
	} {
		t.Run(name, func(t *testing.T) {
			manifest := upstream.Manifest.Copy()
			layer := sourceLayer{Manifest: &manifest, Identity: "trusted-upstream", GenerationID: manifest.GenerationID, Checksum: manifest.Payload.Checksum, Payload: upstream.Payload}
			mutate(&layer)
			if catalog, err := layer.decodeCatalog(); err == nil || catalog != nil {
				t.Fatal("retained scope accepted invalid original evidence")
			}
		})
	}
	source := newStubSource("trusted-upstream")
	source.replies = []SourceRead{{Changed: true, Generation: upstream, PublishedAt: upstream.Manifest.GeneratedAt, ChannelUpdatedAt: upstream.Manifest.GeneratedAt, Health: HealthOK}}
	store, err := storage.NewFilesystem(privateRuntimeDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	options := []Option{WithSource(source), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(store))}
	subscriber := openTestRuntime(t, options...)
	if _, err := subscriber.RefreshSource(t.Context()); err != nil {
		t.Fatalf("import upstream scope: %v", err)
	}
	check := func(runtime *Runtime) {
		t.Helper()
		if !reflect.DeepEqual(runtime.Catalog().MembershipScopes(), expected) {
			t.Fatal("import changed scope identity or original evidence")
		}
		generation, err := store.Current(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := catalogs.DecodeCatalogGeneration(generation); err != nil {
			t.Fatalf("derived generation lost scope evidence: %v", err)
		}
		found := false
		for _, link := range generation.Manifest.SourceObservations {
			found = found || link == observation.Link()
		}
		if !found {
			t.Fatal("derived generation omitted original scoped observation")
		}
	}
	check(subscriber)
	update := manualProviderObservation(t, 300, at.Add(time.Hour))
	if _, err := subscriber.PublishObservations(t.Context(), update); err != nil {
		t.Fatal(err)
	}
	check(subscriber)
	before := subscriber.State()
	if err := subscriber.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := openTestRuntime(t, options...)
	check(restarted)
	if restarted.State().GenerationID != before.GenerationID {
		t.Fatal("restart changed derived generation identity")
	}
	upstreamCatalog, err := catalogs.DecodeCatalogGeneration(upstream)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := catalogs.NewBuilderFrom(upstreamCatalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := replacement.SetMembershipScopes(nil); err != nil {
		t.Fatal(err)
	}
	withdrawn := upstream.Copy()
	withdrawn.Payload, err = catalogs.EncodeCatalogPayload(replacement)
	if err != nil {
		t.Fatal(err)
	}
	withdrawn.Manifest.Payload = catalogs.DescribeCatalogPayload(withdrawn.Payload)
	withdrawn.Manifest.GenerationID += "-scope-withdrawn"
	source.mu.Lock()
	source.replies = []SourceRead{{Changed: true, Generation: withdrawn, PublishedAt: withdrawn.Manifest.GeneratedAt, Health: HealthOK}}
	source.mu.Unlock()
	if _, err := restarted.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(restarted.Catalog().MembershipScopes()) != 0 {
		t.Fatal("source replacement retained a withdrawn scope")
	}
	if err := restarted.Close(); err != nil {
		t.Fatal(err)
	}
	afterWithdrawal := openTestRuntime(t, options...)
	if len(afterWithdrawal.Catalog().MembershipScopes()) != 0 {
		t.Fatal("restart restored a withdrawn upstream scope")
	}
}

func upstreamScopeFixture(t *testing.T) (catalogs.Generation, sources.Observation, []catalogs.ProviderMembershipScope, time.Time) {
	t.Helper()
	binding := sources.ProviderAcquisitionBinding{
		SchemaVersion: sources.ProviderAcquisitionBindingSchemaVersion,
		ID:            "account-a", Revision: "1", ProviderID: "provider", AccountID: "account-a", Region: "global", APISurface: "models.list",
		MembershipAuthority: sources.ProviderMembershipScope, CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "catalog",
	}
	producerStore := storage.NewMemory()
	producer, _, at := providerResetRuntime(t, producerStore, WithProviderBindings(binding))
	observation := providerResetObservation(t, sources.ProvidersID, producer.Catalog(), at, &binding)
	if _, err := producer.PublishObservations(t.Context(), observation); err != nil {
		t.Fatal(err)
	}
	upstream, err := producerStore.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	expected := producer.Catalog().MembershipScopes()
	if len(expected) != 1 {
		t.Fatal("producer did not publish a scope")
	}
	if err := producer.Close(); err != nil {
		t.Fatal(err)
	}
	return upstream, observation, expected, at
}
