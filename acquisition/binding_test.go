package acquisition

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func acquisitionBindingFixture(t *testing.T) (*catalogs.Catalog, sources.ProviderAcquisitionBinding) {
	t.Helper()
	provider := catalogs.Provider{ID: "provider", Name: "Provider", Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.test/models"}}, Credentials: &catalogs.ProviderCredentials{
		Profiles:           []catalogs.ProviderCredentialProfile{{ID: "public", Primitive: catalogs.ProviderAuthenticationNone}},
		CatalogAcquisition: catalogs.ProviderCredentialPlane{Alternatives: []catalogs.ProviderCredentialProfileID{"public"}},
		Inference:          catalogs.ProviderCredentialPlane{Alternatives: []catalogs.ProviderCredentialProfileID{"public"}},
	}}
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	current, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	return current, sources.ProviderAcquisitionBinding{SchemaVersion: 1, ID: "binding", Revision: "1", ProviderID: provider.ID, Public: true, Region: "global", APISurface: "models", CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "public"}
}

func TestAcquirerObservesBindingWithRetainedReceipt(t *testing.T) {
	current, binding := acquisitionBindingFixture(t)
	observer := newProviderSourceObserver(nil)
	observer.options = append(observer.options, providers.WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) { return receiptProviderClient{}, nil }))
	acquirer, err := NewAcquirer(WithProviderObserver(observer))
	if err != nil {
		t.Fatal(err)
	}
	observed, err := acquirer.ObserveProviderBinding(t.Context(), current, binding)
	if err != nil {
		t.Fatal(err)
	}
	if observed.Layer.Receipt.ProviderBinding == nil || *observed.Layer.Receipt.ProviderBinding != binding {
		t.Fatal("provider layer lost requested binding")
	}
	payload, err := catalogs.DecodeSourceObservationPayload(observed.Layer.Payload)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := observed.Layer.Receipt.Restore(payload)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Status != sources.ObservationStatusDegraded || restored.Records.Rejected != 1 {
		t.Fatal("binding erased partial-source evidence")
	}
}

type unscopedOnlyObserver struct{}

func (unscopedOnlyObserver) ObserveProvider(context.Context, *catalogs.Catalog, catalogs.ProviderID) (ProviderObservation, error) {
	return ProviderObservation{}, nil
}

func TestAcquirerRefusesUnscopedCustomObserverForBinding(t *testing.T) {
	current, binding := acquisitionBindingFixture(t)
	acquirer, err := NewAcquirer(WithProviderObserver(unscopedOnlyObserver{}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acquirer.ObserveProviderBinding(t.Context(), current, binding); err == nil {
		t.Fatal("bound request fell back to an unscoped observer")
	}
}

type wrongBindingObserver struct{ providerSourceObserver }

func (o *wrongBindingObserver) ObserveProviderBinding(ctx context.Context, current *catalogs.Catalog, binding sources.ProviderAcquisitionBinding) (ProviderObservation, error) {
	binding.ID = "another-binding"
	return o.providerSourceObserver.ObserveProviderBinding(ctx, current, binding)
}

func TestAcquirerRefusesDifferentReceiptBinding(t *testing.T) {
	current, binding := acquisitionBindingFixture(t)
	observer := &wrongBindingObserver{*newProviderSourceObserver(nil)}
	observer.options = append(observer.options, providers.WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) { return receiptProviderClient{}, nil }))
	acquirer, err := NewAcquirer(WithProviderObserver(observer))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acquirer.ObserveProviderBinding(t.Context(), current, binding); err == nil {
		t.Fatal("accepted another binding's receipt")
	}
}
