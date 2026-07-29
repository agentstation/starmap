package catalogs

import (
	"errors"
	"reflect"
	"testing"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestAuthoredModelJoinsIndependentProviderOfferings(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	for _, author := range []Author{
		{ID: "moonshot-ai", Name: "Moonshot AI"},
		{ID: "alibaba", Name: "Alibaba"},
	} {
		if err := builder.SetAuthor(author); err != nil {
			t.Fatalf("SetAuthor(%s): %v", author.ID, err)
		}
	}
	authored := Model{
		ID: "kimi-k2.5", Name: "Kimi K2.5",
		Authors: []Author{{ID: "moonshot-ai", Name: "Moonshot AI"}},
	}
	if err := builder.SetAuthorModel("moonshot-ai", authored); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	for _, provider := range []Provider{
		{
			ID: "alibaba", Name: "Alibaba Cloud",
			Models: map[string]*Model{
				"kimi-k2.5": {
					ID: "kimi-k2.5", ModelRef: "moonshot-ai/kimi-k2.5",
					Name: "Kimi K2.5", Pricing: testTokenPricing(1),
				},
			},
		},
		{
			ID: "deepinfra", Name: "DeepInfra",
			Models: map[string]*Model{
				"moonshotai/Kimi-K2.5": {
					ID: "moonshotai/Kimi-K2.5", ModelRef: "moonshot-ai/kimi-k2.5",
					Name: "Kimi K2.5", Pricing: testTokenPricing(2),
				},
			},
		},
	} {
		if err := builder.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider(%s): %v", provider.ID, err)
		}
	}

	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	definitions := catalog.Definitions()
	if len(definitions) != 1 || definitions[0].ID != "moonshot-ai/kimi-k2.5" {
		t.Fatalf("definitions = %#v, want one Moonshot definition", definitions)
	}
	if !reflect.DeepEqual(definitions[0].AuthorIDs, []AuthorID{"moonshot-ai"}) {
		t.Fatalf("definition authors = %v, want Moonshot only", definitions[0].AuthorIDs)
	}
	alibaba, err := catalog.Offering("alibaba", "kimi-k2.5")
	if err != nil {
		t.Fatalf("Alibaba Offering: %v", err)
	}
	deepinfra, err := catalog.Offering("deepinfra", "moonshotai/Kimi-K2.5")
	if err != nil {
		t.Fatalf("DeepInfra Offering: %v", err)
	}
	if alibaba.DefinitionID != definitions[0].ID || deepinfra.DefinitionID != definitions[0].ID {
		t.Fatalf("offering definitions = %q and %q, want %q",
			alibaba.DefinitionID, deepinfra.DefinitionID, definitions[0].ID)
	}
	if alibaba.Pricing.Tokens.Input.Per1M != 1 || deepinfra.Pricing.Tokens.Input.Per1M != 2 {
		t.Fatalf("offering prices = %#v and %#v, want exact 1 and 2", alibaba.Pricing, deepinfra.Pricing)
	}
	bySlug, err := catalog.FindModel("kimi-k2.5")
	if err != nil || bySlug.ID != definitions[0].ID {
		t.Fatalf("FindModel(kimi-k2.5) = %#v, %v", bySlug, err)
	}
	byProviderID, err := catalog.FindModel("moonshotai/Kimi-K2.5")
	if err != nil || byProviderID.ID != definitions[0].ID {
		t.Fatalf("FindModel(provider ID) = %#v, %v", byProviderID, err)
	}
	byAuthorSlug, err := catalog.AuthorModel("moonshot-ai", "kimi-k2.5")
	if err != nil || byAuthorSlug.ID != definitions[0].ID {
		t.Fatalf("AuthorModel = %#v, %v", byAuthorSlug, err)
	}
	joined, err := catalog.DefinitionOfferings(definitions[0].ID)
	if err != nil || len(joined) != 2 ||
		joined[0].ProviderID != "alibaba" || joined[1].ProviderID != "deepinfra" {
		t.Fatalf("DefinitionOfferings = %#v, %v", joined, err)
	}
}

func TestAuthoredModelPublicationIsolation(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	model := Model{
		ID: "model", Name: "Original",
		Authors: []Author{{ID: "author", Name: "Author"}},
	}
	if err := builder.SetAuthorModel("author", model); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	model.Name = "Caller mutation"
	if err := builder.SetAuthorModel("author", Model{
		ID: "model", Name: "Builder mutation",
		Authors: []Author{{ID: "author", Name: "Author"}},
	}); err != nil {
		t.Fatalf("mutate builder: %v", err)
	}
	definition, err := catalog.Definition("author/model")
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if definition.Name != "Original" {
		t.Fatalf("published definition name = %q, want Original", definition.Name)
	}
}

func TestSetAuthorModelCanonicalizesAuthorAliasAndDisplayName(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	if err := builder.SetAuthor(Author{
		ID: "canonical", Aliases: []AuthorID{"alias"}, Name: "Canonical Author",
	}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetAuthorModel("alias", Model{
		ID: "model", Name: "Model",
		Authors: []Author{{ID: "alias", Name: "stale name"}},
	}); err != nil {
		t.Fatalf("SetAuthorModel(alias): %v", err)
	}

	records := builder.AuthoredModels()
	if len(records) != 1 || records[0].AuthorID != "canonical" ||
		records[0].Model.Authors[0].ID != "canonical" ||
		records[0].Model.Authors[0].Name != "Canonical Author" {
		t.Fatalf("canonical authored record = %#v", records)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	model, err := catalog.AuthorModel("alias", "model")
	if err != nil || model.ID != "canonical/model" {
		t.Fatalf("AuthorModel(alias, model) = %#v, %v", model, err)
	}
}

func TestDeleteAuthorRejectsOwnedAuthoredModels(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	if err := builder.SetAuthor(Author{
		ID: "canonical", Aliases: []AuthorID{"alias"}, Name: "Canonical Author",
	}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetAuthorModel("canonical", Model{
		ID: "model", Name: "Model",
		Authors: []Author{{ID: "canonical", Name: "Canonical Author"}},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}

	err := builder.DeleteAuthor("alias")
	var conflict *pkgerrors.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("DeleteAuthor(alias) error = %T %v, want ConflictError", err, err)
	}
	if _, err := builder.Author("canonical"); err != nil {
		t.Fatalf("author removed after rejected delete: %v", err)
	}
	if _, err := builder.Build(); err != nil {
		t.Fatalf("Build after rejected delete: %v", err)
	}
}

func TestDeleteAuthorModelResolvesAuthorAlias(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	if err := builder.SetAuthor(Author{
		ID: "canonical", Aliases: []AuthorID{"alias"}, Name: "Canonical Author",
	}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetAuthorModel("alias", Model{
		ID: "model", Name: "Model",
		Authors: []Author{{ID: "alias", Name: "Stale Author"}},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}

	if err := builder.DeleteAuthorModel("alias", "model"); err != nil {
		t.Fatalf("DeleteAuthorModel(alias): %v", err)
	}
	if records := builder.AuthoredModels(); len(records) != 0 {
		t.Fatalf("authored models = %#v, want empty", records)
	}
}

func TestSetAuthorModelRejectsGenerationDefaultOutsideRange(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	err := builder.SetAuthorModel("author", Model{
		ID: "model", Name: "Model",
		Authors: []Author{{ID: "author", Name: "Author"}},
		Generation: &ModelGeneration{
			TopP: &FloatRange{Min: 0, Max: 0, Default: 0.95},
		},
	})
	var validationErr *pkgerrors.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("SetAuthorModel error = %T %v, want ValidationError", err, err)
	}
	if validationErr.Field != "model.generation.top_p" {
		t.Fatalf("validation field = %q, want model.generation.top_p", validationErr.Field)
	}
}

func TestAuthoredAndProviderRecordsRoundTripIndependently(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	builder, err := New(WithPath(path))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := builder.SetAuthor(Author{ID: "moonshot-ai", Name: "Moonshot AI"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetAuthorModel("moonshot-ai", Model{
		ID: "kimi-k2.5", Name: "Kimi K2.5", Description: "authored description",
		Authors: []Author{{ID: "moonshot-ai", Name: "Moonshot AI"}},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	if err := builder.SetProvider(Provider{ID: "alibaba", Name: "Alibaba Cloud"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	if err := builder.SetProviderModel("alibaba", Model{
		ID: "kimi-k2.5", ModelRef: "moonshot-ai/kimi-k2.5",
		Name: "provider display name", Pricing: testTokenPricing(1),
	}); err != nil {
		t.Fatalf("SetProviderModel: %v", err)
	}
	if err := builder.Save(); err != nil {
		t.Fatalf("Save initial: %v", err)
	}

	reloaded, err := NewFromPath(path)
	if err != nil {
		t.Fatalf("NewFromPath initial: %v", err)
	}
	if err := reloaded.SetAuthorModel("moonshot-ai", Model{
		ID: "kimi-k2.5", Name: "Kimi K2.5", Description: "human authored edit",
		Authors: []Author{{ID: "moonshot-ai", Name: "Moonshot AI"}},
	}); err != nil {
		t.Fatalf("edit authored model: %v", err)
	}
	if err := reloaded.Save(); err != nil {
		t.Fatalf("Save authored edit: %v", err)
	}

	reloaded, err = NewFromPath(path)
	if err != nil {
		t.Fatalf("NewFromPath authored edit: %v", err)
	}
	providerModel, err := reloaded.ProviderModel("alibaba", "kimi-k2.5")
	if err != nil {
		t.Fatalf("ProviderModel: %v", err)
	}
	if providerModel.Pricing.Tokens.Input.Per1M != 1 {
		t.Fatalf("provider price after authored edit = %v, want 1", providerModel.Pricing)
	}
	providerModel.Pricing = testTokenPricing(2)
	if err := reloaded.SetProviderModel("alibaba", providerModel); err != nil {
		t.Fatalf("edit provider model: %v", err)
	}
	if err := reloaded.Save(); err != nil {
		t.Fatalf("Save provider edit: %v", err)
	}

	finalBuilder, err := NewFromPath(path)
	if err != nil {
		t.Fatalf("NewFromPath provider edit: %v", err)
	}
	finalCatalog, err := finalBuilder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	definition, err := finalCatalog.Definition("moonshot-ai/kimi-k2.5")
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if definition.Description != "human authored edit" {
		t.Fatalf("authored description after provider edit = %q", definition.Description)
	}
	offering, err := finalCatalog.Offering("alibaba", "kimi-k2.5")
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	if offering.Pricing.Tokens.Input.Per1M != 2 {
		t.Fatalf("provider price after provider edit = %v, want 2", offering.Pricing)
	}
}

func TestCanonicalModelIdentityRejectsUnsafeOrAmbiguousPaths(t *testing.T) {
	t.Parallel()

	for _, id := range []ModelDefinitionID{
		"", "author", "author/model/variant", "/model", "author/",
		"../model", "author/..", `author\model`,
	} {
		id := id
		t.Run(string(id), func(t *testing.T) {
			t.Parallel()
			if _, _, err := ParseModelDefinitionID(id); err == nil {
				t.Fatalf("ParseModelDefinitionID(%q) succeeded, want error", id)
			}
		})
	}
}

func TestProviderModelDanglingReferenceFailsClosed(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider",
		Models: map[string]*Model{
			"model": {ID: "model", ModelRef: "missing/model", Name: "Model"},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	_, err := builder.Build()
	var notFound *pkgerrors.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("Build error = %v, want typed NotFoundError", err)
	}
}

func TestFindModelRejectsAmbiguousBareSlug(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	for _, authorID := range []AuthorID{"author-a", "author-b"} {
		if err := builder.SetAuthor(Author{ID: authorID, Name: string(authorID)}); err != nil {
			t.Fatalf("SetAuthor: %v", err)
		}
		if err := builder.SetAuthorModel(authorID, Model{
			ID: "shared", Name: "Shared",
			Authors: []Author{{ID: authorID, Name: string(authorID)}},
		}); err != nil {
			t.Fatalf("SetAuthorModel: %v", err)
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	_, err = catalog.FindModel("shared")
	var conflict *pkgerrors.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("FindModel(shared) error = %T %v, want ConflictError", err, err)
	}
}

func testTokenPricing(per1M float64) *ModelPricing {
	return &ModelPricing{
		Currency: ModelPricingCurrencyUSD,
		Tokens: &ModelTokenPricing{
			Input: &ModelTokenCost{Per1M: per1M},
		},
	}
}
