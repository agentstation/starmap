package pipeline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/internal/providers/clients"
	"github.com/agentstation/starmap/internal/testcatalog"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestYAMLOnlyProviderAcquisitionPublishesReviewedOfferingAndQuarantinesUnknown(t *testing.T) {
	const (
		providerID     = catalogs.ProviderID("acme")
		authorID       = catalogs.AuthorID("acme-labs")
		knownModelID   = "tenant/acme-model@2026:01"
		unknownModelID = "tenant/new-model@2026:02"
		apiKey         = "yaml-api-key"
	)

	var requestCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestCalls.Add(1)
		if request.URL.Path != "/models" || request.URL.RawQuery != "" {
			t.Errorf("request URL = %q, want /models", request.URL.String())
		}
		if got := request.Header.Get("Authorization"); got != "Bearer "+apiKey {
			t.Errorf("Authorization = %q, want catalog-declared bearer credential", got)
		}
		response.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(response).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{
					"id": knownModelID, "object": "model", "owned_by": "Acme Research",
					"name": "Acme Known Live", "context_window": 131072,
					"max_completion_tokens": 8192,
				},
				{
					"id": unknownModelID, "object": "model", "owned_by": "Acme Research",
					"name": "Acme Unknown Live", "context_window": 65536,
					"max_completion_tokens": 4096,
				},
			},
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	workspace := catalogs.NewEmpty()
	if err := workspace.SetAuthor(catalogs.Author{ID: authorID, Name: "Acme Labs"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := workspace.SetAuthorModel(authorID, catalogs.Model{
		ID: "acme-model", Name: "Acme Model",
		Authors: []catalogs.Author{{ID: authorID, Name: "Acme Labs"}},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	credentials := testcatalog.APIKeyCredentials(
		"ACME_API_KEY",
		"Authorization",
		catalogs.ProviderCredentialSchemeBearer,
	)
	provider := catalogs.Provider{
		ID: providerID, Name: "Acme Inference", Credentials: credentials,
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
			Type:            catalogs.EndpointTypeOpenAI,
			URL:             server.URL + "/models",
			ProtocolOptions: testcatalog.OpenAIProtocolOptions(),
			FieldMappings: []catalogs.FieldMapping{
				{From: "name", To: "name"},
				{From: "context_window", To: "limits.context_window"},
				{From: "max_completion_tokens", To: "limits.output_tokens"},
			},
			AuthorMapping: &catalogs.AuthorMapping{
				Field:      "owned_by",
				Normalized: map[string]catalogs.AuthorID{"Acme Research": authorID},
			},
		}},
		Models: map[string]*catalogs.Model{
			knownModelID: {
				ID: knownModelID, ModelRef: "acme-labs/acme-model", Name: "Reviewed offering",
			},
		},
	}
	if err := workspace.SetProvider(provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	workspacePath := t.TempDir()
	if err := workspace.SaveTo(workspacePath); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	var resolverCalls atomic.Int32
	credentialResolver := sources.ProviderCredentialResolverFunc(func(
		_ context.Context,
		configured *catalogs.Provider,
	) (sources.ProviderCredentialMaterial, error) {
		resolverCalls.Add(1)
		if configured.ID != providerID || configured.Credentials == nil {
			t.Fatalf("resolver provider = %#v", configured)
		}
		fields := configured.Credentials.Fields
		if len(fields) != 1 || fields[0].ID != "api-key" ||
			len(fields[0].Environment) != 1 || fields[0].Environment[0] != "ACME_API_KEY" {
			t.Fatalf("catalog credential fields = %#v", fields)
		}
		return testcatalog.APIKeyMaterial(configured.Credentials, apiKey), nil
	})

	runner := NewAcquisition(func(configured *catalogs.Provider) (sources.ProviderClient, error) {
		return clients.NewProvider(configured)
	}, credentialResolver)
	runner.loadEmbedded = func() (*catalogs.Builder, error) {
		return catalogs.NewEmpty(), nil
	}
	empty := catalogs.NewEmpty()
	existing, err := empty.Build()
	if err != nil {
		t.Fatalf("Build empty baseline: %v", err)
	}
	prepared, err := runner.Prepare(
		context.Background(),
		existing,
		pkgsync.WithDryRun(true),
		pkgsync.WithCatalogPath(workspacePath),
		pkgsync.WithSources(sources.LocalCatalogID, sources.ProvidersID),
		pkgsync.WithProvider(providerID),
	)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if resolverCalls.Load() != 1 || requestCalls.Load() != 1 {
		t.Fatalf("resolver calls = %d, request calls = %d, want 1 each", resolverCalls.Load(), requestCalls.Load())
	}

	acquired, err := prepared.Catalog.Provider(providerID)
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	known := acquired.Models[knownModelID]
	if known == nil || known.ID != knownModelID || known.ModelRef != "acme-labs/acme-model" ||
		known.Name != "Acme Known Live" {
		t.Fatalf("known offering = %#v", known)
	}
	if known.Limits == nil {
		t.Fatalf("known offering limits = %#v", known)
	}
	if value, state := known.Limits.Value(catalogs.ModelLimitContextWindow); value != 131072 || state != catalogs.ValueKnown {
		t.Fatalf("context window = %d/%v, want 131072/known", value, state)
	}
	if value, state := known.Limits.Value(catalogs.ModelLimitOutputTokens); value != 8192 || state != catalogs.ValueKnown {
		t.Fatalf("output tokens = %d/%v, want 8192/known", value, state)
	}
	if len(known.Authors) != 1 || known.Authors[0].ID != authorID {
		t.Fatalf("known offering authors = %#v", known.Authors)
	}
	if acquired.Models[unknownModelID] != nil {
		t.Fatalf("unknown offering was published: %#v", acquired.Models[unknownModelID])
	}

	if len(prepared.Result.ReviewCandidates) != 1 {
		t.Fatalf("review candidates = %#v, want one", prepared.Result.ReviewCandidates)
	}
	candidate := prepared.Result.ReviewCandidates[0]
	if candidate.Code != catalogmeta.ReviewCandidateUnresolvedModelReference ||
		candidate.ProviderID != string(providerID) ||
		candidate.ProviderModelID != unknownModelID ||
		candidate.SourceID != catalogmeta.ProvidersID ||
		candidate.SourceObservationID == "" || candidate.SourceRevision.Kind == "" ||
		candidate.EvidenceChecksum == "" {
		t.Fatalf("review candidate = %#v", candidate)
	}

	providerObservationFound := false
	for _, observation := range prepared.Observations {
		if observation.SourceID != sources.ProvidersID {
			continue
		}
		providerObservationFound = true
		observedProvider, providerErr := observation.Catalog.Provider(providerID)
		if providerErr != nil || observedProvider.Models[unknownModelID] == nil ||
			observedProvider.Models[unknownModelID].ID != unknownModelID {
			t.Fatalf("provider observation did not retain exact unknown ID: provider=%#v error=%v", observedProvider, providerErr)
		}
	}
	if !providerObservationFound {
		t.Fatal("provider observation is missing")
	}
}
