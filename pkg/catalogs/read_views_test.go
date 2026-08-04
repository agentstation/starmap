package catalogs

import (
	"encoding/json"
	stderrors "errors"
	"reflect"
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

func testReadViewModel(id string, price float64, tier string) *Model {
	return &Model{
		ID:       id,
		ModelRef: ModelDefinitionID("author/" + id),
		Name:     "Shared Model",
		Authors:  []Author{{ID: "author", Name: "Author"}},
		Metadata: &ModelMetadata{
			OpenWeights:  true,
			Architecture: &ModelArchitecture{Type: ArchitectureTypeTransformer},
		},
		Features: &ModelFeatures{
			Modalities: ModelModalities{
				Input:  []ModelModality{ModelModalityText},
				Output: []ModelModality{ModelModalityText},
			},
			ToolCalls: true,
		},
		Pricing: testOfferingPricing(price),
		Limits:  &ModelLimits{ContextWindow: 1000},
		Modes: map[string]ModelMode{
			"fast": {
				Provider: &ModelProviderMode{Body: map[string]any{"service_tier": tier}},
			},
		},
	}
}

func setTestReadViewDefinition(t *testing.T, builder *Builder, id, name string) {
	t.Helper()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatalf("SetAuthor(author): %v", err)
	}
	if err := builder.SetAuthorModel("author", Model{
		ID: id, Name: name,
		Authors: []Author{{ID: "author", Name: "Author"}},
	}); err != nil {
		t.Fatalf("SetAuthorModel(author/%s): %v", id, err)
	}
}

func TestReadViewsRejectInvalidModelLifecycle(t *testing.T) {
	builder := NewEmpty()
	setTestReadViewDefinition(t, builder, "model", "Shared Model")
	model := testReadViewModel("model", 1, "standard")
	model.Status = "surprise"
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider",
		Models: map[string]*Model{model.ID: model},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}

	if _, err := builder.Build(); err == nil {
		t.Fatal("Build accepted an invalid model lifecycle")
	}
}

func TestReadViewsPreserveUnknownProviderFacts(t *testing.T) {
	builder := NewEmpty()
	setTestReadViewDefinition(t, builder, "model", "Model")
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider",
		Models: map[string]*Model{
			"model": {ID: "model", ModelRef: "author/model", Name: "Model"},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}

	catalog := mustCatalog(t, builder)
	offering, err := catalog.Offering("provider", "model")
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	if offering.Availability != OfferingAvailabilityUnknown {
		t.Fatalf("availability = %q, want unknown", offering.Availability)
	}
	if offering.Lifecycle != OfferingLifecycleUnknown {
		t.Fatalf("lifecycle = %q, want unknown", offering.Lifecycle)
	}

	definition, err := catalog.Definition("author/model")
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if definition.Weights.Open != nil {
		t.Fatalf("open weights = %v, want unknown", *definition.Weights.Open)
	}
}

func TestReadViewsResolveLineageThroughCanonicalAliases(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	root := "root-provider-id"
	for _, model := range []Model{
		{
			ID: "root", Name: "Root",
			Authors: []Author{{ID: "author", Name: "Author"}},
		},
		{
			ID: "child", Name: "Child",
			Authors: []Author{{ID: "author", Name: "Author"}},
			Lineage: &ModelLineage{Root: &root},
		},
	} {
		if err := builder.SetAuthorModel("author", model); err != nil {
			t.Fatalf("SetAuthorModel(%s): %v", model.ID, err)
		}
	}
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider",
		Models: map[string]*Model{
			"root-provider-id": {
				ID: "root-provider-id", Name: "Root",
				ModelRef: "author/root",
			},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}

	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	child, err := catalog.Definition("author/child")
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if child.Lineage.Root == nil || *child.Lineage.Root != "author/root" {
		t.Fatalf("lineage root = %v, want author/root", child.Lineage.Root)
	}
}

func TestReadViewsRejectDanglingLineage(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	missing := "missing"
	if err := builder.SetAuthorModel("author", Model{
		ID: "child", Name: "Child",
		Authors: []Author{{ID: "author", Name: "Author"}},
		Lineage: &ModelLineage{Parent: &missing},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}

	_, err := builder.Build()
	var notFound *errors.NotFoundError
	if !stderrors.As(err, &notFound) {
		t.Fatalf("Build error = %T %v, want NotFoundError", err, err)
	}
}

func TestReadViewsRejectAmbiguousLineage(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	for _, author := range []Author{
		{ID: "first", Name: "First"},
		{ID: "second", Name: "Second"},
		{ID: "child-author", Name: "Child Author"},
	} {
		if err := builder.SetAuthor(author); err != nil {
			t.Fatalf("SetAuthor(%s): %v", author.ID, err)
		}
	}
	for _, record := range []struct {
		author AuthorID
		model  Model
	}{
		{
			author: "first",
			model: Model{
				ID: "shared", Name: "First Shared",
				Authors: []Author{{ID: "first", Name: "First"}},
			},
		},
		{
			author: "second",
			model: Model{
				ID: "shared", Name: "Second Shared",
				Authors: []Author{{ID: "second", Name: "Second"}},
			},
		},
		{
			author: "child-author",
			model: func() Model {
				root := "shared"
				return Model{
					ID: "child", Name: "Child",
					Authors: []Author{{ID: "child-author", Name: "Child Author"}},
					Lineage: &ModelLineage{Root: &root},
				}
			}(),
		},
	} {
		if err := builder.SetAuthorModel(record.author, record.model); err != nil {
			t.Fatalf("SetAuthorModel(%s/%s): %v", record.author, record.model.ID, err)
		}
	}

	_, err := builder.Build()
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("Build error = %T %v, want ConflictError", err, err)
	}
}

func TestReadViewsDiscardResolvableSelfLineage(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	root := "provider-model"
	if err := builder.SetAuthorModel("author", Model{
		ID: "model", Name: "Model",
		Authors: []Author{{ID: "author", Name: "Author"}},
		Lineage: &ModelLineage{Root: &root},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider",
		Models: map[string]*Model{
			"provider-model": {
				ID: "provider-model", Name: "Model",
				ModelRef: "author/model",
			},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}

	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	definition, err := catalog.Definition("author/model")
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if definition.Lineage.Root != nil {
		t.Fatalf("self lineage root = %v, want nil", *definition.Lineage.Root)
	}
}

func TestReadViewsRejectResolvableSelfParent(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	parent := "provider-model"
	if err := builder.SetAuthorModel("author", Model{
		ID: "model", Name: "Model",
		Authors: []Author{{ID: "author", Name: "Author"}},
		Lineage: &ModelLineage{Parent: &parent},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider",
		Models: map[string]*Model{
			"provider-model": {
				ID: "provider-model", ModelRef: "author/model", Name: "Model",
			},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}

	_, err := builder.Build()
	var validationErr *errors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("Build error = %T %v, want ValidationError", err, err)
	}
}

func TestReadViewsRequireExplicitCanonicalModelReference(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider",
		Models: map[string]*Model{"model": {ID: "model", Name: "Model"}},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}

	_, err := builder.Build()
	var validationErr *errors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("Build error = %T %v, want ValidationError", err, err)
	}
	if validationErr.Field != "provider_model.model" {
		t.Fatalf("validation field = %q, want provider_model.model", validationErr.Field)
	}
}

func TestObservationCatalogPreservesUnresolvedRecordsWithoutPublishingReadViews(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider",
		Models: map[string]*Model{"model": {ID: "model", Name: "Model"}},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}

	observation, err := NewObservationCatalog(builder)
	if err != nil {
		t.Fatalf("NewObservationCatalog: %v", err)
	}
	provider, err := observation.Provider("provider")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if provider.Models["model"] == nil {
		t.Fatal("unresolved provider record was not preserved")
	}
	if definitions := observation.Definitions(); len(definitions) != 0 {
		t.Fatalf("observation definitions = %#v, want no consumer read views", definitions)
	}
	if _, err := observation.Offering("provider", "model"); err == nil {
		t.Fatal("unresolved observation exposed a provider offering")
	}
}

func TestBuildRejectsAmbiguousAliases(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Builder) error
	}{
		{
			name: "provider alias collides with canonical ID",
			setup: func(builder *Builder) error {
				if err := builder.SetProvider(Provider{
					ID: "provider-a", Name: "Provider A", Aliases: []ProviderID{"provider-b"},
				}); err != nil {
					return err
				}
				return builder.SetProvider(Provider{ID: "provider-b", Name: "Provider B"})
			},
		},
		{
			name: "provider alias has multiple owners",
			setup: func(builder *Builder) error {
				if err := builder.SetProvider(Provider{
					ID: "provider-a", Name: "Provider A", Aliases: []ProviderID{"shared"},
				}); err != nil {
					return err
				}
				return builder.SetProvider(Provider{
					ID: "provider-b", Name: "Provider B", Aliases: []ProviderID{"shared"},
				})
			},
		},
		{
			name: "author alias collides with canonical ID",
			setup: func(builder *Builder) error {
				if err := builder.SetAuthor(Author{
					ID: "author-a", Name: "Author A", Aliases: []AuthorID{"author-b"},
				}); err != nil {
					return err
				}
				return builder.SetAuthor(Author{ID: "author-b", Name: "Author B"})
			},
		},
		{
			name: "author alias has multiple owners",
			setup: func(builder *Builder) error {
				if err := builder.SetAuthor(Author{
					ID: "author-a", Name: "Author A", Aliases: []AuthorID{"shared"},
				}); err != nil {
					return err
				}
				return builder.SetAuthor(Author{
					ID: "author-b", Name: "Author B", Aliases: []AuthorID{"shared"},
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := NewEmpty()
			if err := test.setup(builder); err != nil {
				t.Fatalf("setup: %v", err)
			}
			_, err := builder.Build()
			var conflict *errors.ConflictError
			if !stderrors.As(err, &conflict) {
				t.Fatalf("Build error = %T %v, want *errors.ConflictError", err, err)
			}
		})
	}
}

func TestProviderOfferingsPreserveProviderSpecificFacts(t *testing.T) {
	builder := NewEmpty()
	setTestReadViewDefinition(t, builder, "shared", "Shared Model")
	url := "https://api.provider-a.example/v1/chat/completions"
	first := testReadViewModel("shared", 1, "priority")
	first.Status = ModelStatusPreview
	first.Limits = &ModelLimits{ContextWindow: 128_000}
	second := testReadViewModel("shared", 2, "standard")
	second.Status = ModelStatusDeprecated
	second.Limits = &ModelLimits{ContextWindow: 200_000}
	for _, provider := range []Provider{
		{
			ID: "provider-a", Name: "Provider A",
			Inference: &ProviderInference{
				BaseURL: "https://api.provider-a.example",
				Endpoints: []ProviderInferenceEndpoint{{
					Operation: ProviderOperationChatCompletions,
					Type:      EndpointTypeOpenAI,
					Path:      "/v1/chat/completions",
				}},
			},
			Models: map[string]*Model{first.ID: first},
		},
		{
			ID: "provider-b", Name: "Provider B",
			Models: map[string]*Model{second.ID: second},
		},
	} {
		if err := builder.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider(%s): %v", provider.ID, err)
		}
	}
	catalog := mustCatalog(t, builder)

	a, err := catalog.Offering("provider-a", "shared")
	if err != nil {
		t.Fatalf("Offering(provider-a): %v", err)
	}
	b, err := catalog.Offering("provider-b", "shared")
	if err != nil {
		t.Fatalf("Offering(provider-b): %v", err)
	}
	if a.Pricing.Tokens.Input.Per1M != 1 || b.Pricing.Tokens.Input.Per1M != 2 {
		t.Fatalf("prices = (%v, %v), want (1, 2)", a.Pricing, b.Pricing)
	}
	if a.Limits.ContextWindow != 128_000 || b.Limits.ContextWindow != 200_000 {
		t.Fatalf("limits = (%v, %v)", a.Limits, b.Limits)
	}
	if a.Availability != OfferingAvailabilityUnknown ||
		b.Availability != OfferingAvailabilityUnknown {
		t.Fatalf("availability = (%q, %q), want unknown", a.Availability, b.Availability)
	}
	if a.Lifecycle != OfferingLifecyclePreview ||
		b.Lifecycle != OfferingLifecycleDeprecated {
		t.Fatalf("lifecycle = (%q, %q)", a.Lifecycle, b.Lifecycle)
	}
	chatEndpoint, found := a.Endpoint(ProviderOperationChatCompletions)
	if !found || chatEndpoint.URL != url {
		t.Fatalf("chat endpoint = %#v, found = %t, want URL %q", chatEndpoint, found, url)
	}
	if string(a.Modes["fast"].Request.Body["service_tier"]) != `"priority"` ||
		string(b.Modes["fast"].Request.Body["service_tier"]) != `"standard"` {
		t.Fatalf("mode bodies = (%s, %s)", a.Modes["fast"].Request.Body, b.Modes["fast"].Request.Body)
	}

	a.Pricing.Tokens.Input.Per1M = 99
	a.Modes["fast"].Request.Body["service_tier"][0] = 'x'
	again, err := catalog.Offering("provider-a", "shared")
	if err != nil {
		t.Fatalf("Offering(provider-a) again: %v", err)
	}
	if again.Pricing.Tokens.Input.Per1M != 1 ||
		string(again.Modes["fast"].Request.Body["service_tier"]) != `"priority"` {
		t.Fatal("Offering exposed mutable catalog state")
	}
}

func TestProviderOfferingOmitsChatURLForOperationPricedModel(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	setTestReadViewDefinition(t, builder, "image-model", "Image Model")
	perImage := 0.01
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider",
		Inference: &ProviderInference{
			BaseURL: "https://api.example",
			Endpoints: []ProviderInferenceEndpoint{{
				Operation: ProviderOperationChatCompletions,
				Type:      EndpointTypeOpenAI,
				Path:      "/v1/chat/completions",
			}},
		},
		Models: map[string]*Model{
			"image-model": {
				ID: "image-model", ModelRef: "author/image-model", Name: "Image Model",
				Features: &ModelFeatures{Modalities: ModelModalities{
					Input:  []ModelModality{ModelModalityText},
					Output: []ModelModality{ModelModalityImage},
				}},
				Pricing: &ModelPricing{
					Currency: ModelPricingCurrencyUSD,
					Operations: &ModelOperationPricing{
						ImageGen: &perImage,
					},
				},
			},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}

	offering, err := mustCatalog(t, builder).Offering("provider", "image-model")
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	if len(offering.Endpoints) != 0 {
		t.Fatalf("image offering endpoints = %#v, want omitted chat route", offering.Endpoints)
	}
}

func TestProviderLabelsCannotOverrideAuthoredIdentity(t *testing.T) {
	builder := NewEmpty()
	setTestReadViewDefinition(t, builder, "shared", "Canonical Model")
	for _, provider := range []Provider{
		{
			ID: "provider-a", Name: "Provider A",
			Models: map[string]*Model{"deployment-a": {
				ID: "deployment-a", ModelRef: "author/shared", Name: "Provider Label A",
			}},
		},
		{
			ID: "provider-b", Name: "Provider B",
			Models: map[string]*Model{"deployment-b": {
				ID: "deployment-b", ModelRef: "author/shared", Name: "Provider Label B",
			}},
		},
	} {
		if err := builder.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider(%s): %v", provider.ID, err)
		}
	}

	definition, err := mustCatalog(t, builder).Definition("author/shared")
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if definition.Name != "Canonical Model" {
		t.Fatalf("definition name = %q, want authored identity", definition.Name)
	}
}

func TestDefinitionUsesProviderIndependentAuthoredFacts(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	features := &ModelFeatures{
		Modalities: ModelModalities{
			Input:  []ModelModality{ModelModalityImage, ModelModalityText},
			Output: []ModelModality{ModelModalityText},
		},
	}
	features.SetSupport(ModelFeatureToolCalls, true)
	if err := builder.SetAuthorModel("author", Model{
		ID: "shared", Name: "Shared",
		Authors: []Author{{ID: "author", Name: "Author"}},
		Metadata: &ModelMetadata{
			Architecture: &ModelArchitecture{
				Type:           ArchitectureTypeTransformer,
				ParameterCount: "42",
			},
			Tags: []ModelTag{ModelTagCoding, ModelTagMath},
		},
		Features: features,
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	for _, provider := range []Provider{
		{
			ID: "provider-a", Name: "Provider A",
			Models: map[string]*Model{"deployment-a": {
				ID: "deployment-a", ModelRef: "author/shared", Name: "Provider A label",
			}},
		},
		{
			ID: "provider-b", Name: "Provider B",
			Models: map[string]*Model{"deployment-b": {
				ID: "deployment-b", ModelRef: "author/shared", Name: "Provider B label",
			}},
		},
	} {
		if err := builder.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider(%s): %v", provider.ID, err)
		}
	}
	definition, err := mustCatalog(t, builder).Definition("author/shared")
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if definition.Weights.Architecture == nil ||
		definition.Weights.Architecture.Type != ArchitectureTypeTransformer ||
		definition.Weights.Architecture.ParameterCount != "42" {
		t.Fatalf("architecture = %#v, want filled leaves", definition.Weights.Architecture)
	}
	if !slices.Equal(definition.Metadata.Tags, []ModelTag{ModelTagCoding, ModelTagMath}) {
		t.Fatalf("tags = %#v, want set union", definition.Metadata.Tags)
	}
	toolCalls, state := definition.Capabilities.Features.Support(ModelFeatureToolCalls)
	if !toolCalls || state != ValueKnown {
		t.Fatalf("tool calls = (%t, %v), want known true union", toolCalls, state)
	}
	if !reflect.DeepEqual(
		definition.Capabilities.Features.Modalities,
		ModelModalities{
			Input:  []ModelModality{ModelModalityImage, ModelModalityText},
			Output: []ModelModality{ModelModalityText},
		},
	) {
		t.Fatalf("modalities = %#v, want union", definition.Capabilities.Features.Modalities)
	}
}

func TestAuthorMembershipUsesExplicitAuthoredCredits(t *testing.T) {
	builder := NewEmpty()
	authors := []Author{
		{ID: "inline", Aliases: []AuthorID{"inline-alias"}, Name: "Inline"},
		{
			ID: "provider-owner", Name: "Provider Owner",
			Catalog: &AuthorCatalog{Attribution: &AuthorAttribution{ProviderID: "provider"}},
		},
		{
			ID: "provider-pattern", Name: "Provider Pattern",
			Catalog: &AuthorCatalog{Attribution: &AuthorAttribution{
				ProviderID: "provider",
				Patterns:   []string{"shared-*"},
			}},
		},
		{
			ID: "global-pattern", Name: "Global Pattern",
			Catalog: &AuthorCatalog{Attribution: &AuthorAttribution{Patterns: []string{"*-model"}}},
		},
	}
	for _, author := range authors {
		if err := builder.SetAuthor(author); err != nil {
			t.Fatalf("SetAuthor(%s): %v", author.ID, err)
		}
	}
	model := &Model{
		ID: "shared-model", ModelRef: "inline/shared-model", Name: "Shared",
		Authors: []Author{{ID: "inline-alias", Name: "Inline Alias"}},
	}
	if err := builder.SetAuthorModel("inline", Model{
		ID: "shared-model", Name: "Shared",
		Authors: []Author{{ID: "inline-alias", Name: "Inline Alias"}},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider",
		Models: map[string]*Model{model.ID: model},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog := mustCatalog(t, builder)
	definition, err := catalog.Definition("inline/shared-model")
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	want := []AuthorID{"inline"}
	if !slices.Equal(definition.AuthorIDs, want) {
		t.Fatalf("author IDs = %#v, want %#v", definition.AuthorIDs, want)
	}
	for _, authorID := range want {
		models, err := catalog.AuthorModels(authorID)
		if err != nil {
			t.Fatalf("AuthorModels(%s): %v", authorID, err)
		}
		if len(models) != 1 || models[0].ID != "inline/shared-model" {
			t.Fatalf("AuthorModels(%s) = %#v", authorID, models)
		}
	}
	viaAlias, err := catalog.AuthorModels("inline-alias")
	if err != nil || len(viaAlias) != 1 {
		t.Fatalf("AuthorModels(inline-alias) = (%#v, %v)", viaAlias, err)
	}
}

func TestProviderAuthorFetchScopeDoesNotInventAuthorship(t *testing.T) {
	for _, authors := range [][]AuthorID{
		{"author-a"},
		{"author-a", "author-b"},
	} {
		builder := NewEmpty()
		setTestReadViewDefinition(t, builder, "model", "Model")
		if err := builder.SetProvider(Provider{
			ID: "marketplace", Name: "Marketplace",
			Catalog: &ProviderCatalog{Authors: authors},
			Models: map[string]*Model{"model": {
				ID: "model", ModelRef: "author/model", Name: "Model",
			}},
		}); err != nil {
			t.Fatalf("SetProvider: %v", err)
		}
		definition, err := mustCatalog(t, builder).Definition("author/model")
		if err != nil {
			t.Fatalf("Definition: %v", err)
		}
		if !slices.Equal(definition.AuthorIDs, []AuthorID{"author"}) {
			t.Fatalf("fetch-scope authors %v produced definition authors %#v", authors, definition.AuthorIDs)
		}
	}
}

func TestLegacyAuthorAttributionDoesNotOverrideExplicitAuthorship(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetAuthor(Author{
		ID: "author", Name: "Author",
		Catalog: &AuthorCatalog{Attribution: &AuthorAttribution{Patterns: []string{"["}}},
	}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetAuthorModel("author", Model{
		ID: "model", Name: "Model",
		Authors: []Author{{ID: "author", Name: "Author"}},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider",
		Models: map[string]*Model{"model": {
			ID: "model", ModelRef: "author/model", Name: "Model",
		}},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	definition, err := catalog.Definition("author/model")
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if !slices.Equal(definition.AuthorIDs, []AuthorID{"author"}) {
		t.Fatalf("definition authors = %#v, want explicit author only", definition.AuthorIDs)
	}
}

func TestZeroOfferingProviderAndAliasReturnEmptySlices(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider", Aliases: []ProviderID{"alias"},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog := mustCatalog(t, builder)
	for _, providerID := range []ProviderID{"provider", "alias"} {
		offerings, err := catalog.ProviderOfferings(providerID)
		if err != nil {
			t.Fatalf("ProviderOfferings(%s): %v", providerID, err)
		}
		if offerings == nil || len(offerings) != 0 {
			t.Fatalf("ProviderOfferings(%s) = %#v, want non-nil empty", providerID, offerings)
		}
	}
}

func TestProviderModeBodyRejectsNonJSONValue(t *testing.T) {
	builder := NewEmpty()
	model := testReadViewModel("model", 1, "standard")
	model.Modes["fast"].Provider.Body["channel"] = make(chan int)
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider", Models: map[string]*Model{model.ID: model},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	_, err := builder.Build()
	if err == nil {
		t.Fatal("Build accepted a provider mode body that JSON cannot encode")
	}

	if _, marshalErr := json.Marshal(make(chan int)); marshalErr == nil {
		t.Fatal("test fixture unexpectedly became JSON encodable")
	}
}
