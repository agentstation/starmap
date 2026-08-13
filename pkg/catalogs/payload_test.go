package catalogs

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/provenance"
)

func TestCatalogPayloadPersistsDisjointAuthorAndProviderModels(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
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
		Models: map[string]*Model{
			"model": {
				ID:       "model",
				ModelRef: "author/model",
				Name:     "Model",
				Pricing: &ModelPricing{
					Currency: ModelPricingCurrencyUSD,
					Tokens: &ModelTokenPricing{
						CacheRead: &ModelTokenCost{Per1M: 0.25},
					},
				},
			},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}

	payload, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(payload, &document); err != nil {
		t.Fatalf("Unmarshal payload: %v", err)
	}

	keys := make([]string, 0, len(document))
	for key := range document {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	want := []string{
		"author_models",
		"authors",
		"provenance",
		"provider_models",
		"providers",
		"schema_version",
	}
	if !slices.Equal(keys, want) {
		t.Fatalf("top-level payload keys = %v, want %v", keys, want)
	}

	var providerModels map[string][]Model
	if err := json.Unmarshal(document["provider_models"], &providerModels); err != nil {
		t.Fatalf("Unmarshal provider models: %v", err)
	}
	if models := providerModels["provider"]; len(models) != 1 || models[0].ID != "model" {
		t.Fatalf("provider models = %#v, want one canonical provider model", providerModels)
	}
	model := providerModels["provider"][0]
	if model.Pricing == nil || model.Pricing.Tokens == nil ||
		model.Pricing.Tokens.CacheRead == nil ||
		model.Pricing.Tokens.CacheRead.Per1M != 0.25 {
		t.Fatalf("flat cache pricing = %#v, want cache_read 0.25", model.Pricing)
	}
	if model.ModelRef != "author/model" {
		t.Fatalf("provider model reference = %q, want author/model", model.ModelRef)
	}

	var authorModels map[string][]Model
	if err := json.Unmarshal(document["author_models"], &authorModels); err != nil {
		t.Fatalf("Unmarshal author models: %v", err)
	}
	if models := authorModels["author"]; len(models) != 1 || models[0].ID != "model" {
		t.Fatalf("author models = %#v, want one authored model", authorModels)
	}
	authored := authorModels["author"][0]
	if authored.ModelRef != "" || authored.Pricing != nil || authored.Limits != nil {
		t.Fatalf("authored model contains provider-serving facts: %#v", authored)
	}
}

func TestCatalogSemanticChecksumSeparatesFactsFromObservationEvidence(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetProvider(Provider{ID: "provider", Name: "Provider"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	firstObservedAt := time.Date(2026, time.July, 29, 20, 0, 0, 0, time.UTC)
	setEvidence := func(observedAt time.Time, observationID string) {
		builder.SetProvenance(provenance.Map{
			"providers.provider.Name": {{
				Source: evidence.ProvidersID, Field: "Name", Value: "Provider",
				Timestamp: observedAt, ObservedAt: observedAt,
				ObservationID:    observationID,
				EvidenceChecksum: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			}},
		})
	}
	setEvidence(firstObservedAt, "observation:first")
	first, err := builder.Build()
	if err != nil {
		t.Fatalf("Build first: %v", err)
	}
	firstPayload, err := EncodeCatalogPayload(first)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload first: %v", err)
	}
	firstSemantic, err := CatalogSemanticChecksum(first)
	if err != nil {
		t.Fatalf("CatalogSemanticChecksum first: %v", err)
	}

	setEvidence(firstObservedAt.Add(time.Hour), "observation:second")
	second, err := builder.Build()
	if err != nil {
		t.Fatalf("Build second: %v", err)
	}
	secondPayload, err := EncodeCatalogPayload(second)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload second: %v", err)
	}
	secondSemantic, err := CatalogSemanticChecksum(second)
	if err != nil {
		t.Fatalf("CatalogSemanticChecksum second: %v", err)
	}
	if bytes.Equal(firstPayload, secondPayload) {
		t.Fatal("exact evidence payload did not change with observation identity/time")
	}
	if firstSemantic != secondSemantic {
		t.Fatalf("semantic checksums differ: %q != %q", firstSemantic, secondSemantic)
	}

	if err := builder.SetProvider(Provider{ID: "provider", Name: "Changed Provider"}); err != nil {
		t.Fatalf("SetProvider changed: %v", err)
	}
	changed, err := builder.Build()
	if err != nil {
		t.Fatalf("Build changed: %v", err)
	}
	changedSemantic, err := CatalogSemanticChecksum(changed)
	if err != nil {
		t.Fatalf("CatalogSemanticChecksum changed: %v", err)
	}
	if changedSemantic == firstSemantic {
		t.Fatal("catalog fact change retained the previous semantic checksum")
	}
}
