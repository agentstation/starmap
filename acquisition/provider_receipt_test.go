package acquisition

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestProviderSourceObserverPreservesPartialReceipt(t *testing.T) {
	builder := catalogs.NewEmpty()
	provider := catalogs.Provider{ID: "provider", Name: "Provider",
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.test/models"}},
		Credentials: &catalogs.ProviderCredentials{
			Profiles:           []catalogs.ProviderCredentialProfile{{ID: "public", Primitive: catalogs.ProviderAuthenticationNone}},
			CatalogAcquisition: catalogs.ProviderCredentialPlane{Alternatives: []catalogs.ProviderCredentialProfileID{"public"}},
			Inference:          catalogs.ProviderCredentialPlane{Alternatives: []catalogs.ProviderCredentialProfileID{"public"}},
		},
	}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	current, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	observer := &providerSourceObserver{options: []providers.SourceOption{
		providers.WithCredentialResolver(auth.NewResolver()),
		providers.WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) { return receiptProviderClient{}, nil }),
	}}
	observed, err := observer.ObserveProvider(t.Context(), current, provider.ID)
	if err != nil {
		t.Fatal(err)
	}
	if observed.Attempt.Outcome != sources.ProviderOutcomeSucceeded {
		t.Fatalf("attempt = %s", observed.Attempt.Outcome)
	}
	raw, err := json.Marshal(observed.Layer)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Link    catalogs.SourceObservationLink  `json:"link"`
		Records sources.ObservationRecordCounts `json:"records"`
	}
	if err := json.Unmarshal(fields["Receipt"], &receipt); err != nil {
		t.Fatalf("provider observation lost its receipt: %v", err)
	}
	if receipt.Link.Completeness != sources.ObservationCompletenessPartial || receipt.Link.Status != sources.ObservationStatusDegraded || receipt.Records.Rejected != 1 {
		t.Fatalf("partial receipt = %+v", receipt)
	}
}

type receiptProviderClient struct{}

func (receiptProviderClient) ListModels(context.Context, sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
	return []catalogs.Model{{ID: "valid", Name: "Valid"}, {Name: "Invalid"}}, nil
}
