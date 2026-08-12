package reconciler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestGeneratedYAMLRetainsSourceIdentityAndSemanticEditsBecomeLocal(t *testing.T) {
	t.Parallel()

	providerV1 := sourceIdentityCatalog(t, "https://provider.example/v1", catalogs.Model{
		ID:   "shared",
		Name: "Provider Shared",
		Limits: &catalogs.ModelLimits{
			ContextWindow: 8192,
		},
		Pricing: sourceIdentityPricing(1),
		Features: &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
				Output: []catalogs.ModelModality{catalogs.ModelModalityText},
			},
			Tools: true,
		},
	})
	modelsDev := sourceIdentityCatalog(t, "", catalogs.Model{
		ID:          "shared",
		Name:        "Upstream Shared",
		Description: "Generated upstream description",
		Status:      catalogs.ModelStatusActive,
		Limits: &catalogs.ModelLimits{
			OutputTokens: 4096,
		},
		Features: &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input:  []catalogs.ModelModality{catalogs.ModelModalityImage},
				Output: []catalogs.ModelModality{catalogs.ModelModalityImage},
			},
		},
	})
	providerObservationV1 := sourceIdentityObservation(
		t,
		sources.ProvidersID,
		providerV1,
		time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC),
	)
	modelsDevObservation := sourceIdentityObservation(
		t,
		sources.ModelsDevHTTPID,
		modelsDev,
		time.Date(2026, time.July, 28, 10, 1, 0, 0, time.UTC),
	)

	generated := sourceIdentityReconcile(t, sources.ProvidersID, providerObservationV1, modelsDevObservation)
	workspace := filepath.Join(t.TempDir(), "catalog")
	if err := generated.Catalog.SaveTo(workspace); err != nil {
		t.Fatalf("Save generated workspace: %v", err)
	}

	reloaded := sourceIdentityLoad(t, workspace)
	localObservation := sourceIdentityObservation(
		t,
		sources.LocalCatalogID,
		reloaded,
		time.Date(2026, time.July, 28, 10, 2, 0, 0, time.UTC),
	)
	unchanged := sourceIdentityReconcile(t, sources.LocalCatalogID, localObservation)
	assertModelEvidenceSource(
		t,
		unchanged.Catalog,
		"Name",
		sources.ProvidersID,
		providerObservationV1.ID,
	)
	assertModelEvidenceSource(
		t,
		unchanged.Catalog,
		"Description",
		sources.ModelsDevHTTPID,
		modelsDevObservation.ID,
	)
	assertModelEvidenceSource(
		t,
		unchanged.Catalog,
		"limits.context_window",
		sources.ProvidersID,
		providerObservationV1.ID,
	)
	assertModelEvidenceSource(
		t,
		unchanged.Catalog,
		"limits.output_tokens",
		sources.ModelsDevHTTPID,
		modelsDevObservation.ID,
	)
	assertModelEvidenceSource(
		t,
		unchanged.Catalog,
		"pricing",
		sources.ProvidersID,
		providerObservationV1.ID,
	)
	assertModelEvidenceSource(
		t,
		unchanged.Catalog,
		"Status",
		sources.ModelsDevHTTPID,
		modelsDevObservation.ID,
	)
	assertModelEvidenceSource(
		t,
		unchanged.Catalog,
		"Features",
		sources.ProvidersID,
		providerObservationV1.ID,
	)
	assertProviderEvidenceSource(
		t,
		unchanged.Catalog,
		"Catalog",
		sources.ProvidersID,
		providerObservationV1.ID,
	)

	providerV2 := sourceIdentityCatalog(t, "https://provider.example/v2", catalogs.Model{
		ID:   "shared",
		Name: "Provider Shared",
		Limits: &catalogs.ModelLimits{
			ContextWindow: 16384,
		},
		Pricing: sourceIdentityPricing(2),
		Features: &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
				Output: []catalogs.ModelModality{catalogs.ModelModalityText},
			},
			Tools: true,
		},
	})
	providerObservationV2 := sourceIdentityObservation(
		t,
		sources.ProvidersID,
		providerV2,
		time.Date(2026, time.July, 28, 10, 3, 0, 0, time.UTC),
	)
	refreshed := sourceIdentityReconcile(t, sources.ProvidersID, providerObservationV2, localObservation)
	refreshedProvider, err := refreshed.Catalog.Provider("provider-a")
	if err != nil {
		t.Fatalf("Provider after refresh: %v", err)
	}
	if got := refreshedProvider.Catalog.Endpoint.URL; got != "https://provider.example/v2" {
		t.Fatalf("unchanged generated provider catalog blocked dynamic refresh: %q", got)
	}
	refreshedModel := refreshedProvider.Models["shared"]
	if refreshedModel == nil || refreshedModel.Limits == nil || refreshedModel.Limits.ContextWindow != 16384 {
		t.Fatalf("unchanged generated limit blocked dynamic refresh: %#v", refreshedModel)
	}
	assertProviderEvidenceSource(
		t,
		refreshed.Catalog,
		"Catalog",
		sources.ProvidersID,
		providerObservationV2.ID,
	)

	replaceWorkspaceText(
		t,
		filepath.Join(workspace, "providers.yaml"),
		"https://provider.example/v1",
		"https://operator.example/manual",
	)
	replaceWorkspaceText(
		t,
		filepath.Join(workspace, "providers", "provider-a", "models", "shared.yaml"),
		"Generated upstream description",
		"Operator supplied description",
	)
	editedCatalog := sourceIdentityLoad(t, workspace)
	editedObservation := sourceIdentityObservation(
		t,
		sources.LocalCatalogID,
		editedCatalog,
		time.Date(2026, time.July, 28, 10, 4, 0, 0, time.UTC),
	)
	edited := sourceIdentityReconcile(t, sources.ProvidersID, providerObservationV2, editedObservation)
	editedProvider, err := edited.Catalog.Provider("provider-a")
	if err != nil {
		t.Fatalf("Provider after edit: %v", err)
	}
	if got := editedProvider.Catalog.Endpoint.URL; got != "https://operator.example/manual" {
		t.Fatalf("operator provider configuration did not win: %q", got)
	}
	editedModel := editedProvider.Models["shared"]
	if editedModel == nil || editedModel.Description != "Operator supplied description" {
		t.Fatalf("operator model fallback did not survive: %#v", editedModel)
	}
	assertProviderEvidenceSource(
		t,
		edited.Catalog,
		"Catalog",
		sources.LocalCatalogID,
		editedObservation.ID,
	)
	assertModelEvidenceSource(
		t,
		edited.Catalog,
		"Description",
		sources.LocalCatalogID,
		editedObservation.ID,
	)
}

func sourceIdentityCatalog(t testing.TB, catalogURL string, model catalogs.Model) *catalogs.Catalog {
	t.Helper()

	var providerCatalog *catalogs.ProviderCatalog
	if catalogURL != "" {
		providerCatalog = &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type:            catalogs.EndpointTypeOpenAI,
				URL:             catalogURL,
				ProtocolOptions: testcatalog.OpenAIProtocolOptions(),
			},
		}
	}
	builder := catalogs.NewEmpty()
	author := catalogs.Author{ID: "test-author", Name: "Test Author"}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	model.ModelRef = catalogs.AuthoredModelID(author.ID, model.ID)
	if err := builder.SetAuthorModel(author.ID, catalogs.Model{
		ID:          model.ID,
		Name:        model.Name,
		Description: model.Description,
		Authors:     []catalogs.Author{author},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	if err := builder.SetProvider(catalogs.Provider{
		ID:          "provider-a",
		Name:        "Provider A",
		Credentials: testcatalog.UnauthenticatedCredentials(),
		Catalog:     providerCatalog,
		Models:      map[string]*catalogs.Model{model.ID: &model},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return catalog
}

func sourceIdentityPricing(input float64) *catalogs.ModelPricing {
	return &catalogs.ModelPricing{
		Currency: catalogs.ModelPricingCurrencyUSD,
		Tokens: &catalogs.ModelTokenPricing{
			Input: &catalogs.ModelTokenCost{Per1M: input},
		},
	}
}

func sourceIdentityObservation(
	t testing.TB,
	sourceID sources.ID,
	catalog *catalogs.Catalog,
	observedAt time.Time,
) sources.Observation {
	t.Helper()

	observation, err := sources.NewObservation(sourceID, catalog, sources.ObservationMetadata{
		ObservedAt:   observedAt,
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
		Records:      sources.ObservationRecordCounts{Accepted: 1},
	})
	if err != nil {
		t.Fatalf("NewObservation(%s): %v", sourceID, err)
	}
	return observation
}

func sourceIdentityReconcile(t testing.TB, primary sources.ID, observations ...sources.Observation) *Result {
	t.Helper()

	var options []Option
	if len(observations) > 0 {
		options = append(options, WithBaseline(observations[0].Catalog))
	}
	reconcile, err := New(options...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := reconcile.Sources(context.Background(), primary, observations)
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	return result
}

func sourceIdentityLoad(t testing.TB, workspace string) *catalogs.Catalog {
	t.Helper()

	builder, err := catalogs.NewFromPath(workspace)
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build loaded workspace: %v", err)
	}
	return catalog
}

func replaceWorkspaceText(t testing.TB, path, old, replacement string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	updated := strings.ReplaceAll(string(content), old, replacement)
	if updated == string(content) {
		t.Fatalf("%s does not contain %q", path, old)
	}
	if err := os.WriteFile(path, []byte(updated), constants.FilePermissions); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func assertModelEvidenceSource(
	t testing.TB,
	catalog catalogs.Reader,
	field string,
	source sources.ID,
	observationID string,
) {
	t.Helper()

	entries := catalog.Provenance().FindModelField("provider-a", "shared", field)
	if len(entries) != 1 || entries[0].Source != source || entries[0].ObservationID != observationID {
		t.Fatalf("model %s evidence = %#v, want %s/%s", field, entries, source, observationID)
	}
}

func assertProviderEvidenceSource(
	t testing.TB,
	catalog catalogs.Reader,
	field string,
	source sources.ID,
	observationID string,
) {
	t.Helper()

	entries := catalog.Provenance().FindByField(sources.ResourceTypeProvider, "provider-a", field)
	if len(entries) != 1 || entries[0].Source != source || entries[0].ObservationID != observationID {
		t.Fatalf("provider %s evidence = %#v, want %s/%s", field, entries, source, observationID)
	}
}
