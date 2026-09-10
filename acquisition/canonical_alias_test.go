package acquisition

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/agentstation/starmap/runtime"
)

type aliasAcquisitionSource struct{ generation catalogs.Generation }

func (aliasAcquisitionSource) Identity() string { return "alias-acquisition-baseline" }
func (s aliasAcquisitionSource) Read(context.Context) (runtime.SourceRead, error) {
	return runtime.SourceRead{Changed: true, Generation: s.generation, Health: runtime.HealthOK, PublishedAt: s.generation.Manifest.GeneratedAt}, nil
}

func TestCanonicalAliasSurvivesProviderPreviewFreshSyncAndRestart(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/models" || r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Error("wrong provider request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"deployment","name":"Live offering","context_window":12345}]}`))
	}))
	defer server.Close()
	builder := catalogs.NewEmpty()
	author := catalogs.Author{ID: "author", Name: "Author"}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetAuthorModel(author.ID, catalogs.Model{ID: "current", Name: "Current", Authors: []catalogs.Author{author}}); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetCanonicalAliasRecords([]catalogs.CanonicalAlias{{ID: "author/old", TargetID: "author/current", PublisherID: "baseline", State: catalogs.CanonicalAliasActive}}); err != nil {
		t.Fatal(err)
	}
	credentials := testcatalog.APIKeyCredentials("ACME_API_KEY", "Authorization", catalogs.ProviderCredentialSchemeBearer)
	if err := builder.SetProvider(catalogs.Provider{ID: "acme", Name: "Acme", Credentials: credentials, Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
		Type: catalogs.EndpointTypeOpenAI, URL: server.URL + "/models", ProtocolOptions: testcatalog.OpenAIProtocolOptions(),
		FieldMappings: []catalogs.FieldMapping{{From: "name", To: "name"}, {From: "context_window", To: "limits.context_window"}},
	}}, Models: map[string]*catalogs.Model{"deployment": {ID: "deployment", ModelRef: "author/current", Name: "Baseline offering"}}}); err != nil {
		t.Fatal(err)
	}
	workspacePath := filepath.Join(t.TempDir(), "workspace")
	if err := builder.SaveTo(workspacePath); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := starmap.New(starmap.WithCatalogStore(storage.NewMemory()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.Update(t.Context(), func(context.Context, *catalogs.Catalog) (*starmap.Candidate, error) {
		return starmap.NewCandidate(catalog, starmap.CandidateEvidence{})
	}); err != nil {
		t.Fatal(err)
	}
	generation, err := publisher.CurrentGeneration(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	options := []runtime.Option{runtime.WithSource(aliasAcquisitionSource{generation}), runtime.WithStateDirectory(filepath.Join(t.TempDir(), "runtime")), runtime.WithSourcePollInterval(0), runtime.WithAcquisitionEnabled(false), runtime.WithClientOptions(starmap.WithCatalogPath(workspacePath))}
	connected, err := runtime.Open(t.Context(), options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connected.Close() })
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	before := connected.State()
	syncer, err := NewForRuntime(connected, WithCredentialResolver(sources.ProviderCredentialResolverFunc(func(_ context.Context, p *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
		return testcatalog.APIKeyMaterial(p.Credentials, "fixture-key"), nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	selected := []pkgsync.Option{pkgsync.WithSources(sources.LocalCatalogID, sources.ProvidersID), pkgsync.WithProvider("acme"), pkgsync.WithFresh(true)}
	preview, err := syncer.Sync(t.Context(), append(selected, pkgsync.WithDryRun(true))...)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || connected.State().GenerationID != before.GenerationID {
		t.Fatal("preview published a generation")
	}
	result, err := syncer.Sync(t.Context(), selected...)
	if err != nil {
		t.Fatal(err)
	}
	if result.GenerationID == "" || result.GenerationID == before.GenerationID || requests.Load() != 2 {
		t.Fatal("provider sync did not publish observed facts")
	}
	assert := func(c *catalogs.Catalog) {
		t.Helper()
		definition, err := c.FindModel("author/old")
		if err != nil || definition.ID != "author/current" {
			t.Fatalf("alias lookup: %v, %v", definition, err)
		}
		provider, err := c.Provider("acme")
		if err != nil {
			t.Fatal(err)
		}
		if provider.Models["deployment"].Name != "Live offering" || provider.Models["deployment"].Limits.ContextWindow != 12345 {
			t.Fatal("live facts were lost")
		}
		if len(c.CanonicalAliasRecords()) != 1 {
			t.Fatal("acquisition changed rename inventory")
		}
	}
	assert(connected.Catalog())
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := runtime.Open(t.Context(), options...)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	assert(restarted.Catalog())
	if restarted.State().GenerationID != result.GenerationID || requests.Load() != 2 {
		t.Fatal("restart acquired facts or changed the accepted generation")
	}
}
