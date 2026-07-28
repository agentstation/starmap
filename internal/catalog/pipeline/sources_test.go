package pipeline

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestFilterSourcesHonorsExplicitSourceSelection(t *testing.T) {
	localCatalog := catalogs.NewEmpty()

	filtered := filterSources(&pkgsync.Options{
		Sources: []sources.ID{sources.LocalCatalogID, sources.ModelsDevHTTPID},
	}, asSnapshot(localCatalog), catalogs.LoadReport{}, existingWorkspaceInput())

	got := sourceIDs(filtered)
	want := []sources.ID{
		sources.LocalCatalogID,
		sources.ModelsDevHTTPID,
	}
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
	filtered := filterSources(
		&pkgsync.Options{Fresh: true},
		asSnapshot(catalogs.NewEmpty()),
		catalogs.LoadReport{},
		existingWorkspaceInput(),
	)

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
	}, asSnapshot(localCatalog), catalogs.LoadReport{}, existingWorkspaceInput())

	got := sourceIDs(srcs)
	want := []sources.ID{
		sources.LocalCatalogID,
		sources.ProvidersID,
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
			want: []sources.ID{
				sources.LocalCatalogID,
				sources.ProvidersID,
				sources.ModelsDevHTTPID,
			},
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := sourceIDs(filterSources(
				&pkgsync.Options{Sources: test.sources},
				localCatalog,
				catalogs.LoadReport{},
				existingWorkspaceInput(),
			))
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

func TestLocalLoadReportSurvivesPrebuiltPipelineCatalog(t *testing.T) {
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{
		ID: "local", Name: "Local",
		Models: map[string]*catalogs.Model{
			"valid": {ID: "valid", Name: "Valid"},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	report := catalogs.LoadReport{
		Accepted: 1,
		Rejected: 1,
		Issues: []catalogs.LoadIssue{{
			Path: "providers/local/models/invalid.yaml",
			Err:  pkgerrors.NewParseError("yaml", "invalid.yaml", "schema drift", nil),
		}},
	}
	srcs := filterSources(
		&pkgsync.Options{Sources: []sources.ID{sources.LocalCatalogID}},
		asSnapshot(builder),
		report,
		existingWorkspaceInput(),
	)
	if len(srcs) != 1 || srcs[0].ID() != sources.LocalCatalogID {
		t.Fatalf("sources = %v, want local", sourceIDs(srcs))
	}
	observation, err := srcs[0].Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if observation.Status != sources.ObservationStatusDegraded ||
		observation.Records.Accepted != 1 ||
		observation.Records.Rejected != 1 ||
		len(observation.Issues) != 1 {
		t.Fatalf("observation = %#v, want propagated load degradation", observation)
	}
}

func TestAbsentWorkspaceIsNotObservedAsLocalConfiguration(t *testing.T) {
	srcs := filterSources(
		&pkgsync.Options{},
		asSnapshot(catalogs.NewEmpty()),
		catalogs.LoadReport{},
		workspace.InputExpectation{Path: "/absent/catalog"},
	)
	for _, source := range srcs {
		if source.ID() == sources.LocalCatalogID {
			t.Fatal("absent workspace was configured as a local observation")
		}
	}
	if got := sourceIDs(srcs); len(got) == 0 || got[0] != sources.EmbeddedCatalogID {
		t.Fatalf("absent workspace sources = %v, want embedded baseline", got)
	}
}

func existingWorkspaceInput() workspace.InputExpectation {
	return workspace.InputExpectation{Path: "/catalog", Exists: true}
}
