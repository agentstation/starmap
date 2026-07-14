package catalogs

import "testing"

func sourceProjectionTestModel(id string, price float64, tier string) *Model {
	return &Model{
		ID:      id,
		Name:    "Shared Model",
		Authors: []Author{{ID: "author", Name: "Author"}},
		Metadata: &ModelMetadata{
			OpenWeights:  true,
			Architecture: &ModelArchitecture{Type: ArchitectureTypeTransformer},
		},
		Features: &ModelFeatures{ToolCalls: true},
		Pricing:  testOfferingPricing(price),
		Limits:   &ModelLimits{ContextWindow: 1000},
		Modes: map[string]ModelMode{
			"fast": {Provider: &ModelProviderMode{Body: map[string]any{"service_tier": tier}}},
		},
	}
}

func TestSourceProjectionPreservesCursorApplicationBoundaryAndModes(t *testing.T) {
	model := sourceProjectionTestModel("composer-2.5", 3, "fast")
	model.Authors = []Author{{ID: AuthorIDCursor, Name: "Cursor"}}
	model.OfferingAccess = &OfferingAccess{Channel: OfferingAccessChannelApplication, Routability: OfferingRoutabilityDiscoverable}
	model.OfferingEndpoint = ProviderOfferingEndpoint{Type: EndpointTypeApplication}
	model.Modes = map[string]ModelMode{
		"standard": {Pricing: testOfferingPricing(0.5)},
		"fast":     {Pricing: testOfferingPricing(3)},
	}
	provider := Provider{ID: ProviderIDCursor, Name: "Cursor", Catalog: &ProviderCatalog{Sources: []ProviderSource{{
		ID: "application", Endpoint: ProviderSourceEndpoint{Type: EndpointTypeApplication},
	}}}, Models: map[string]*Model{model.ID: model}}
	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: AuthorIDCursor, Name: "Cursor"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	offering, err := catalog.Offering(ProviderIDCursor, ProviderModelID(model.ID))
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	if offering.IsRoutable() || offering.Access.Channel != OfferingAccessChannelApplication ||
		offering.Access.Routability != OfferingRoutabilityDiscoverable || len(offering.Access.APIs) != 0 ||
		offering.Deployment.Type != "application" || offering.Endpoint.Type != EndpointTypeApplication {
		t.Fatalf("application offering = %#v", offering)
	}
	if len(offering.Modes) != 2 || offering.Modes["standard"].Pricing.Tokens.Input.Per1M != 0.5 ||
		offering.Modes["fast"].Pricing.Tokens.Input.Per1M != 3 {
		t.Fatalf("Composer modes = %#v", offering.Modes)
	}
}
