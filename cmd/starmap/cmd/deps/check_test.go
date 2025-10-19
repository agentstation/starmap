package deps

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestCollectDependencyStatuses(t *testing.T) {
	tests := []struct {
		name                  string
		sources               []sources.Source
		wantSourcesCount      int
		wantSourcesWithNoDeps int
	}{
		{
			name: "git source with bun dependency",
			sources: []sources.Source{
				modelsdev.NewGitSource(),
			},
			wantSourcesCount:      1,
			wantSourcesWithNoDeps: 0,
		},
		{
			name: "http source with no dependencies",
			sources: []sources.Source{
				modelsdev.NewHTTPSource(),
			},
			wantSourcesCount:      1,
			wantSourcesWithNoDeps: 1,
		},
		{
			name: "providers source with no dependencies",
			sources: []sources.Source{
				providers.New(catalogs.NewProviders()),
			},
			wantSourcesCount:      1,
			wantSourcesWithNoDeps: 1,
		},
		{
			name: "multiple sources",
			sources: []sources.Source{
				providers.New(catalogs.NewProviders()),
				modelsdev.NewGitSource(),
				modelsdev.NewHTTPSource(),
			},
			wantSourcesCount:      3,
			wantSourcesWithNoDeps: 2, // providers and http have no deps
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			results := collectDependencyStatuses(ctx, tt.sources)

			if len(results.Sources) != tt.wantSourcesCount {
				t.Errorf("collectDependencyStatuses() sources count = %v, want %v",
					len(results.Sources), tt.wantSourcesCount)
			}

			if results.SourcesWithNoDeps != tt.wantSourcesWithNoDeps {
				t.Errorf("collectDependencyStatuses() sourcesWithNoDeps = %v, want %v",
					results.SourcesWithNoDeps, tt.wantSourcesWithNoDeps)
			}

			// Verify total dependencies count
			expectedTotal := results.AvailableDeps + results.MissingDeps
			if results.TotalDeps != expectedTotal {
				t.Errorf("collectDependencyStatuses() totalDeps = %v, want %v",
					results.TotalDeps, expectedTotal)
			}
		})
	}
}

func TestGetAllSources(t *testing.T) {
	allSources := getAllSources()

	// We expect 4 sources: local, providers, git, http
	if len(allSources) != 4 {
		t.Errorf("getAllSources() returned %d sources, want 4", len(allSources))
	}

	// Verify we have all expected source IDs
	expectedIDs := map[sources.ID]bool{
		sources.LocalCatalogID:  false,
		sources.ProvidersID:     false,
		sources.ModelsDevGitID:  false,
		sources.ModelsDevHTTPID: false,
	}

	for _, src := range allSources {
		if _, ok := expectedIDs[src.ID()]; ok {
			expectedIDs[src.ID()] = true
		}
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("getAllSources() missing source %s", id)
		}
	}
}

func TestSourceDepStatus(t *testing.T) {
	// Test that SourceDepStatus can be marshaled for JSON output
	status := SourceDepStatus{
		SourceID:   sources.ProvidersID,
		SourceName: "providers",
		IsOptional: false,
		Dependencies: []DependencyDetail{
			{
				Dependency: sources.Dependency{
					Name:        "test-dep",
					DisplayName: "Test Dependency",
					Required:    true,
				},
				Status: sources.DependencyStatus{
					Available: true,
					Version:   "1.0.0",
					Path:      "/usr/bin/test",
				},
			},
		},
	}

	// Basic validation
	if status.SourceID != sources.ProvidersID {
		t.Errorf("SourceID = %v, want %v", status.SourceID, sources.ProvidersID)
	}
	if len(status.Dependencies) != 1 {
		t.Errorf("Dependencies length = %v, want 1", len(status.Dependencies))
	}
}

func TestCheckResults(t *testing.T) {
	// Test that CheckResults properly accumulates counts
	results := &CheckResults{
		Sources:           []SourceDepStatus{},
		TotalDeps:         5,
		AvailableDeps:     3,
		MissingDeps:       2,
		SourcesWithNoDeps: 1,
	}

	if results.TotalDeps != (results.AvailableDeps + results.MissingDeps) {
		t.Errorf("TotalDeps should equal AvailableDeps + MissingDeps")
	}
}
