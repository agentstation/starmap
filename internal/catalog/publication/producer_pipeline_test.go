package publication

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestProducerPipelineRetainsOneAccountAndExcludesDisabledScopes(t *testing.T) {
	baseline, profile := producerPipelineFixture(t)
	var phase, resolutions, requests, clients atomic.Int32
	factory := func(*catalogs.Provider) (sources.ProviderClient, error) {
		clients.Add(1)
		return producerPipelineClient{phase: &phase, requests: &requests}, nil
	}
	resolver := sources.ProviderCredentialResolverFunc(func(_ context.Context, provider *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
		resolutions.Add(1)
		alternatives := provider.Credentials.CatalogAcquisition.Alternatives
		if len(alternatives) != 1 || alternatives[0] == "disabled" {
			return sources.ProviderCredentialMaterial{}, &pkgerrors.ConfigError{Message: "unexpected acquisition profile"}
		}
		for _, declared := range provider.Credentials.Profiles {
			if declared.ID == alternatives[0] {
				values := map[catalogs.ProviderCredentialFieldID]string{"api-key": "fixture"}
				if phase.Load() == 2 && declared.ID == "two" {
					return sources.ProviderCredentialMaterial{}, &pkgerrors.AuthenticationError{Provider: "provider", Method: "api_key", Message: "fixture credential absent"}
				}
				return sources.NewProviderCredentialMaterial(declared, values, sources.ProviderCredentialMetadata{}), nil
			}
		}
		return sources.ProviderCredentialMaterial{}, &pkgerrors.ConfigError{Message: "missing profile"}
	})
	producer, err := NewProducer(profile, factory, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if resolutions.Load() != 0 || clients.Load() != 0 || requests.Load() != 0 {
		t.Fatal("constructor started acquisition")
	}
	profile.Scopes[0].Scope.Binding.ID = "caller-change"
	profile.Scopes[1].Enabled = false
	options := []pkgsync.Option{pkgsync.WithCatalogPath(t.TempDir())}
	first, err := producer.Collect(t.Context(), baseline, nil, options...)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Decision.Allowed || !first.Decision.FreshAcquisition || len(first.Decision.Inputs) != 2 {
		t.Fatalf("first admission=%+v", first.Decision)
	}
	retained := first.Decision.Inputs
	if resolutions.Load() != 2 || clients.Load() != 2 || requests.Load() != 2 {
		t.Fatal("initial acquisition did not use exactly two scopes")
	}
	for _, state := range []int32{1, 2} {
		phase.Store(state)
		resolutions.Store(0)
		clients.Store(0)
		requests.Store(0)
		result, err := producer.Collect(t.Context(), baseline, retained, options...)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Decision.Allowed || !result.Decision.FreshAcquisition || len(result.Decision.Inputs) != 2 {
			t.Fatalf("mixed admission=%+v", result.Decision)
		}
		wantOutcome, wantRequests := Failed, int32(2)
		if state == 2 {
			wantOutcome, wantRequests = MissingCredentials, 1
		}
		if resolutions.Load() != 2 || clients.Load() != wantRequests || requests.Load() != wantRequests {
			t.Fatalf("resolutions=%d clients=%d requests=%d", resolutions.Load(), clients.Load(), requests.Load())
		}
		if result.Run.Attempts[0].Outcome != Succeeded || result.Run.Attempts[1].Outcome != wantOutcome || result.Run.Attempts[2].Outcome != Disabled {
			t.Fatalf("independent attempts=%+v", result.Run.Attempts)
		}
		if result.Decision.Scopes[0].EvidenceKind != FreshEvidence || result.Decision.Scopes[1].EvidenceKind != RetainedEvidence || !result.Decision.Scopes[1].Required {
			t.Fatalf("scope evidence=%+v", result.Decision.Scopes)
		}
		for _, observation := range result.Decision.Inputs {
			if observation.ProviderBinding.ID == "two" && observation.ID != retained[1].ID {
				t.Fatal("failed account lost original retained identity")
			}
		}
	}
	missing, err := producer.Collect(t.Context(), baseline, nil, options...)
	if err != nil {
		t.Fatal(err)
	}
	if missing.Decision.Allowed || len(missing.Decision.Inputs) != 0 {
		t.Fatal("required account became optional without retained evidence")
	}
}

type producerPipelineClient struct{ phase, requests *atomic.Int32 }

func (c producerPipelineClient) ListModels(_ context.Context, material sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
	c.requests.Add(1)
	id := string(material.Profile().ID)
	if id == "two" && c.phase.Load() == 1 {
		return nil, &pkgerrors.APIError{Provider: "provider", Endpoint: "/models", StatusCode: 503}
	}
	return []catalogs.Model{{ID: id, ModelRef: catalogs.ModelDefinitionID("author/" + id), Name: id}}, nil
}

func producerPipelineFixture(t *testing.T) (*catalogs.Catalog, Profile) {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetAuthor(catalogs.Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatal(err)
	}
	credentials := testcatalog.APIKeyCredentials("FIXTURE_PUBLICATION_KEY", "Authorization", catalogs.ProviderCredentialSchemeBearer)
	template := credentials.Profiles[0]
	credentials.Profiles = nil
	credentials.CatalogAcquisition.Alternatives = nil
	credentials.Inference.Alternatives = nil
	profile := Profile{Version: "fixture-v1"}
	models := make(map[string]*catalogs.Model)
	for _, id := range []string{"one", "two", "disabled"} {
		if err := builder.SetAuthorModel("author", catalogs.Model{ID: id, Name: id, Authors: []catalogs.Author{{ID: "author", Name: "Author"}}}); err != nil {
			t.Fatal(err)
		}
		models[id] = &catalogs.Model{ID: id, ModelRef: catalogs.ModelDefinitionID("author/" + id), Name: id}
		declared := template
		declared.ID = catalogs.ProviderCredentialProfileID(id)
		credentials.Profiles = append(credentials.Profiles, declared)
		credentials.CatalogAcquisition.Alternatives = append(credentials.CatalogAcquisition.Alternatives, declared.ID)
		credentials.Inference.Alternatives = append(credentials.Inference.Alternatives, declared.ID)
		binding := sources.ProviderAcquisitionBinding{SchemaVersion: sources.ProviderAcquisitionBindingSchemaVersion, ID: id, Revision: "1", ProviderID: "provider", AccountID: "account-" + id, Region: "global", APISurface: "models", CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: declared.ID, MembershipAuthority: sources.ProviderMembershipScope}
		profile.Scopes = append(profile.Scopes, ScopePolicy{Scope: Scope{Source: sources.ProvidersID, Binding: &binding}, Enabled: id != "disabled", Required: true, MaxRetainedAge: time.Hour, DisabledAction: Preserve})
	}
	if err := builder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider", Models: models, Credentials: credentials, Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.test/models", ProtocolOptions: testcatalog.OpenAIProtocolOptions()}}}); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	return catalog, profile
}
