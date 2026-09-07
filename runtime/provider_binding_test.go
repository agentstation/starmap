package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestProviderBindingReceiptSurvivesRetentionAndRestart(t *testing.T) {
	r := providerOrderRuntime(t, true)
	base := testProviderLayer(t, "provider", "model", "Model", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	catalog, err := catalogs.DecodeSourceObservationPayload(base.Payload)
	if err != nil {
		t.Fatal(err)
	}
	binding := sources.ProviderAcquisitionBinding{
		SchemaVersion: sources.ProviderAcquisitionBindingSchemaVersion, ID: "account-a", Revision: "1", ProviderID: "provider",
		AccountID: "account-a", Region: "global", APISurface: "models.list", CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "catalog-key",
	}
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
		ProviderBinding: &binding, ObservedAt: base.ObservedAt, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	layer, err := NewProviderLayer("provider", observation)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.retainProviders(t.Context(), []ProviderLayer{layer}); err != nil {
		t.Fatal(err)
	}
	layer.Receipt.ProviderBinding.AccountID = "caller-change"
	if *r.layers.providers[layer.evidenceKey()].Receipt.ProviderBinding != binding {
		t.Fatal("retained binding shares caller-owned memory")
	}
	loaded, err := r.store.loadProviders()
	if err != nil {
		t.Fatal(err)
	}
	retained := loaded[layer.evidenceKey()]
	if retained.Receipt.ProviderBinding == nil || *retained.Receipt.ProviderBinding != binding || retained.Receipt.Link.ObservationID != observation.ID {
		t.Fatal("restart lost binding identity")
	}
	retained.Receipt.ProviderBinding.AccountID = "forged"
	if err := r.store.writeContext(t.Context(), r.store.bindings, retained.evidenceKey().filename(), retained); err != nil {
		t.Fatal(err)
	}
	if _, err := r.store.loadProviders(); err == nil {
		t.Fatal("restart accepted changed binding with original identity")
	}
}
