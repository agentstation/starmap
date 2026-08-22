package catalogs

import (
	"bytes"
	"testing"
	"time"

	"github.com/agentstation/utc"
)

// identityFixture builds a catalog whose provider and author carry brand
// identity (logo bytes, description, docs URL) and whose provider model
// carries lifecycle dates.
func identityFixture(t *testing.T) (*Builder, Provider, Author, Model) {
	t.Helper()

	builder := NewEmpty()
	author := Author{
		ID:   "qwen",
		Name: "Qwen",
		Logo: []byte(`<svg xmlns="http://www.w3.org/2000/svg"><title>qwen</title></svg>`),
	}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	features := &ModelFeatures{}
	for _, feature := range modelFeatures() {
		features.SetSupport(feature, false)
	}
	if err := builder.SetAuthorModel(author.ID, Model{
		ID: "qwen3-max", Name: "Qwen3-Max", Authors: []Author{{ID: author.ID, Name: author.Name}}, Features: features,
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}

	deprecatedAt := utc.New(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	retiresAt := utc.New(time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC))
	model := Model{
		ID:           "qwen/qwen3-max-legacy",
		ModelRef:     "qwen/qwen3-max",
		Name:         "Qwen3 Max Legacy",
		Status:       ModelStatusDeprecated,
		DeprecatedAt: &deprecatedAt,
		RetiresAt:    &retiresAt,
		Features:     features,
	}
	description := "Serves frontier open-weight models over an OpenAI-compatible API."
	docsURL := "https://example.com/docs"
	provider := Provider{
		ID:          "deepinfra",
		Name:        "DeepInfra",
		Description: &description,
		DocsURL:     &docsURL,
		Logo:        []byte(`<svg xmlns="http://www.w3.org/2000/svg"><title>deepinfra</title></svg>`),
		Models:      map[string]*Model{model.ID: &model},
	}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	return builder, provider, author, model
}

func assertIdentityCatalog(t *testing.T, catalog *Catalog, provider Provider, author Author, model Model) {
	t.Helper()

	gotProvider, err := catalog.Provider(provider.ID)
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if gotProvider.Description == nil || *gotProvider.Description != *provider.Description {
		t.Fatalf("provider description = %v, want %q", gotProvider.Description, *provider.Description)
	}
	if gotProvider.DocsURL == nil || *gotProvider.DocsURL != *provider.DocsURL {
		t.Fatalf("provider docs URL = %v, want %q", gotProvider.DocsURL, *provider.DocsURL)
	}
	if !bytes.Equal(gotProvider.Logo, provider.Logo) {
		t.Fatalf("provider logo = %q, want %q", gotProvider.Logo, provider.Logo)
	}

	gotAuthor, err := catalog.Author(author.ID)
	if err != nil {
		t.Fatalf("Author: %v", err)
	}
	if !bytes.Equal(gotAuthor.Logo, author.Logo) {
		t.Fatalf("author logo = %q, want %q", gotAuthor.Logo, author.Logo)
	}

	gotModel, ok := gotProvider.Models[model.ID]
	if !ok || gotModel == nil {
		t.Fatalf("provider model %q missing", model.ID)
	}
	if gotModel.DeprecatedAt == nil || !gotModel.DeprecatedAt.Equal(*model.DeprecatedAt) {
		t.Fatalf("model deprecated_at = %v, want %v", gotModel.DeprecatedAt, model.DeprecatedAt)
	}
	if gotModel.RetiresAt == nil || !gotModel.RetiresAt.Equal(*model.RetiresAt) {
		t.Fatalf("model retires_at = %v, want %v", gotModel.RetiresAt, model.RetiresAt)
	}

	offerings, err := catalog.ProviderOfferings(provider.ID)
	if err != nil {
		t.Fatalf("ProviderOfferings: %v", err)
	}
	if len(offerings) != 1 {
		t.Fatalf("offerings = %d, want 1", len(offerings))
	}
	offering := offerings[0]
	if offering.Lifecycle != OfferingLifecycleDeprecated {
		t.Fatalf("offering lifecycle = %q, want deprecated", offering.Lifecycle)
	}
	if offering.DeprecatedAt == nil || !offering.DeprecatedAt.Equal(*model.DeprecatedAt) {
		t.Fatalf("offering deprecated_at = %v, want %v", offering.DeprecatedAt, model.DeprecatedAt)
	}
	if offering.RetiresAt == nil || !offering.RetiresAt.Equal(*model.RetiresAt) {
		t.Fatalf("offering retires_at = %v, want %v", offering.RetiresAt, model.RetiresAt)
	}
}

// TestCatalogPayloadCarriesIdentityAndLifecycle proves logo bytes, provider
// description, docs URL, and offering lifecycle dates survive the canonical
// payload encode/decode round trip.
func TestCatalogPayloadCarriesIdentityAndLifecycle(t *testing.T) {
	t.Parallel()

	builder, provider, author, model := identityFixture(t)
	original, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	assertIdentityCatalog(t, original, provider, author, model)

	payload, err := EncodeCatalogPayload(original)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	decoded, err := DecodeCatalogPayload(payload)
	if err != nil {
		t.Fatalf("DecodeCatalogPayload: %v", err)
	}
	assertIdentityCatalog(t, decoded, provider, author, model)
}

// TestCatalogWorkspaceRoundTripPreservesIdentity proves logo sidecar files and
// lifecycle dates survive a SaveTo/NewFromPath workspace round trip.
func TestCatalogWorkspaceRoundTripPreservesIdentity(t *testing.T) {
	t.Parallel()

	builder, provider, author, model := identityFixture(t)
	path := t.TempDir()
	if err := builder.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	reloadedBuilder, err := NewFromPath(path)
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	reloaded, err := reloadedBuilder.Build()
	if err != nil {
		t.Fatalf("Build reloaded: %v", err)
	}
	assertIdentityCatalog(t, reloaded, provider, author, model)
}
