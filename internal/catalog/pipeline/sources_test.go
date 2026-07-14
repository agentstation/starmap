package pipeline

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestFilterSourcesHonorsExplicitSourceSelection(t *testing.T) {
	localCatalog := catalogs.NewEmpty()

	filtered, err := filterSources(&pkgsync.Options{
		Sources: []sources.ID{sources.LocalCatalogID, sources.ModelsDevHTTPID},
	}, asSnapshot(localCatalog))
	if err != nil {
		t.Fatalf("filterSources: %v", err)
	}

	got := sourceIDs(filtered)
	want := []sources.ID{sources.LocalCatalogID, sources.ModelsDevHTTPID}
	if len(got) != len(want) {
		t.Fatalf("Expected %d filtered sources, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Filtered source %d mismatch: expected %s, got %s", i, want[i], got[i])
		}
	}
}

func TestFilterSourcesFreshExcludesLocalCatalog(t *testing.T) {
	filtered, err := filterSources(&pkgsync.Options{Fresh: true}, asSnapshot(catalogs.NewEmpty()))
	if err != nil {
		t.Fatalf("filterSources: %v", err)
	}

	for _, id := range sourceIDs(filtered) {
		if id == sources.LocalCatalogID {
			t.Fatal("Fresh source selection retained the existing local catalog")
		}
	}
}

func TestCreateSourcesWithConfigUsesModelsDevSourcesDir(t *testing.T) {
	localCatalog := configuredNativeCatalog(t)

	srcs, err := createSourcesWithConfig(&pkgsync.Options{
		SourcesDir: t.TempDir(),
	}, asSnapshot(localCatalog))
	if err != nil {
		t.Fatalf("createSourcesWithConfig: %v", err)
	}

	got := sourceIDs(srcs)
	want := []sources.ID{
		sources.LocalCatalogID,
		sources.ProvidersID,
		sources.AmazonBedrockID,
		sources.MicrosoftFoundryID,
		sources.OCIGenerativeAIID,
		sources.ModelsDevHTTPID,
	}
	if len(got) != len(want) {
		t.Fatalf("Expected %d configured sources, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Configured source %d mismatch: expected %s, got %s", i, want[i], got[i])
		}
	}
}

func TestModelsDevSourceSelectionFallbackMatrix(t *testing.T) {
	localCatalog := asSnapshot(configuredNativeCatalog(t))
	tests := []struct {
		name    string
		sources []sources.ID
		want    []sources.ID
	}{
		{
			name: "default uses HTTP transport only",
			want: []sources.ID{sources.LocalCatalogID, sources.ProvidersID, sources.AmazonBedrockID, sources.MicrosoftFoundryID, sources.OCIGenerativeAIID, sources.ModelsDevHTTPID},
		},
		{
			name:    "explicit HTTP verification",
			sources: []sources.ID{sources.ModelsDevHTTPID},
			want:    []sources.ID{sources.ModelsDevHTTPID},
		},
		{
			name:    "explicit Git verification",
			sources: []sources.ID{sources.ModelsDevGitID},
			want:    []sources.ID{sources.ModelsDevGitID},
		},
		{
			name:    "provider-only does not add a models.dev fallback transport",
			sources: []sources.ID{sources.ProvidersID},
			want:    []sources.ID{sources.ProvidersID},
		},
		{
			name:    "explicit Bedrock regional sweep",
			sources: []sources.ID{sources.AmazonBedrockID},
			want:    []sources.ID{sources.AmazonBedrockID},
		},
		{
			name:    "explicit Microsoft Foundry account sweep",
			sources: []sources.ID{sources.MicrosoftFoundryID},
			want:    []sources.ID{sources.MicrosoftFoundryID},
		},
		{
			name:    "explicit OCI regional sweep",
			sources: []sources.ID{sources.OCIGenerativeAIID},
			want:    []sources.ID{sources.OCIGenerativeAIID},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filtered, err := filterSources(&pkgsync.Options{Sources: test.sources}, localCatalog)
			if err != nil {
				t.Fatalf("filterSources: %v", err)
			}
			got := sourceIDs(filtered)
			if len(got) != len(test.want) {
				t.Fatalf("source IDs = %v, want %v", got, test.want)
			}
			for index := range test.want {
				if got[index] != test.want[index] {
					t.Fatalf("source IDs = %v, want %v", got, test.want)
				}
			}
		})
	}
}

func configuredNativeCatalog(t *testing.T) *catalogs.Builder {
	t.Helper()
	builder := catalogs.NewEmpty()
	for _, config := range []struct {
		id       catalogs.ProviderID
		endpoint catalogs.EndpointType
	}{
		{catalogs.ProviderIDAmazonBedrock, catalogs.EndpointTypeBedrock},
		{catalogs.ProviderIDMicrosoftFoundry, catalogs.EndpointTypeAzureOpenAI},
		{catalogs.ProviderIDOCI, catalogs.EndpointTypeOCI},
	} {
		provider := catalogs.Provider{ID: config.id, Name: string(config.id), Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "inventory", Optional: true,
			ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeCredentialScoped},
			Auth:             catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"cloud_chain"}},
			Endpoint:         catalogs.ProviderSourceEndpoint{Type: config.endpoint, URL: "https://example.com"},
		}}}}
		if err := builder.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider(%s): %v", config.id, err)
		}
	}
	connectorProvider := catalogs.Provider{
		ID: catalogs.ProviderIDOpenAI, Name: "OpenAI",
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "models", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic},
			Auth:     catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone},
			Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.com/models"},
		}}},
	}
	if err := builder.SetProvider(connectorProvider); err != nil {
		t.Fatalf("SetProvider(%s): %v", connectorProvider.ID, err)
	}
	return builder
}
