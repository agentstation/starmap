package pipeline

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestFilterSourcesHonorsExplicitSourceSelection(t *testing.T) {
	localCatalog := catalogs.NewEmpty()

	filtered := filterSources(&pkgsync.Options{
		Sources: []sources.ID{sources.LocalCatalogID, sources.ModelsDevHTTPID},
	}, asSnapshot(localCatalog))

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
	filtered := filterSources(&pkgsync.Options{Fresh: true}, asSnapshot(catalogs.NewEmpty()))

	for _, id := range sourceIDs(filtered) {
		if id == sources.LocalCatalogID {
			t.Fatal("Fresh source selection retained the existing local catalog")
		}
	}
}

func TestCreateSourcesWithConfigUsesModelsDevSourcesDir(t *testing.T) {
	localCatalog := catalogs.NewEmpty()

	srcs := createSourcesWithConfig(&pkgsync.Options{
		SourcesDir: t.TempDir(),
	}, asSnapshot(localCatalog))

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
	localCatalog := asSnapshot(catalogs.NewEmpty())
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
			got := sourceIDs(filterSources(&pkgsync.Options{Sources: test.sources}, localCatalog))
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
