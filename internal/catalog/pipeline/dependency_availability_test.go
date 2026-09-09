package pipeline

import (
	"errors"
	"testing"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestMissingDependenciesCannotUseDistributionAsAcquisition(t *testing.T) {
	for _, base := range []sources.ID{sources.EmbeddedCatalogID, sources.ReleaseArtifactID} {
		t.Run(string(base), func(t *testing.T) {
			missing := &lifecycleTestSource{id: sources.ModelsDevGitID, optional: true, deps: []sources.Dependency{missingDependencyForTest()}}
			selected := []sources.Source{&lifecycleTestSource{id: base}, missing}
			resolved, err := resolveDependencies(t.Context(), selected, pkgsync.Defaults())
			var configuration *pkgerrors.ConfigError
			var dependency *pkgerrors.DependencyError
			if !errors.As(err, &configuration) || !errors.As(err, &dependency) || dependency.Source != string(sources.ModelsDevGitID) {
				t.Fatalf("dependency error=%v", err)
			}
			if len(resolved) != 0 {
				t.Fatal("distribution baseline masked unavailable acquisition")
			}
			selected = append(selected, &lifecycleTestSource{id: sources.LocalCatalogID})
			resolved, err = resolveDependencies(t.Context(), selected, pkgsync.Defaults())
			if err != nil || len(resolved) != 2 {
				t.Fatalf("available local source was rejected: sources=%v error=%v", sourceIDs(resolved), err)
			}
		})
	}
}
