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
	}, localCatalog)

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

func TestCreateSourcesWithConfigUsesModelsDevSourcesDir(t *testing.T) {
	localCatalog := catalogs.NewEmpty()

	srcs := createSourcesWithConfig(&pkgsync.Options{
		SourcesDir: t.TempDir(),
	}, localCatalog)

	got := sourceIDs(srcs)
	want := []sources.ID{
		sources.LocalCatalogID,
		sources.ProvidersID,
		sources.ModelsDevGitID,
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
