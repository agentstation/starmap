package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime"
)

// refuseLegacyPath prevents an implicit root change from abandoning known state.
// Explicit path selections retain their exact locations.
func refuseLegacyPath(selected string, legacy string) error {
	if filepath.Clean(selected) == filepath.Clean(legacy) {
		return nil
	}
	if _, err := os.Lstat(legacy); err == nil {
		return &errors.ConflictError{Resource: "product directory migration", Expected: selected, Actual: legacy, Message: "legacy state requires an explicit verified migration"}
	} else if !os.IsNotExist(err) {
		return errors.WrapIO("inspect", legacy, err)
	}
	return nil
}

func checkLegacyCatalogRoots(paths ProductPaths) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	legacy := filepath.Join(home, ".starmap")
	checks := []struct {
		path   string
		origin string
		legacy []string
	}{
		{paths.Workspace.Path, paths.Workspace.Origin, []string{"catalog/providers.yaml", "catalog/authors.yaml", "catalog/current"}},
		{paths.CatalogStore.Path, paths.CatalogStore.Origin, []string{"state/catalog/current"}},
	}
	for _, check := range checks {
		if check.origin != "platform-default" {
			continue
		}
		for _, name := range check.legacy {
			if err := refuseLegacyPath(check.path, filepath.Join(legacy, filepath.FromSlash(name))); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *App) checkLegacyRoots(ctx context.Context, paths ProductPaths) (*runtime.DirectoryMigrationCompletion, error) {
	if err := checkLegacyCatalogRoots(paths); err != nil {
		return nil, err
	}
	if paths.Runtime.Origin != "platform-default" {
		return nil, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil
	}
	var sources []string
	for _, parts := range [][]string{{"state", "runtime"}, {"state"}} {
		source := filepath.Join(append([]string{home, ".starmap"}, parts...)...)
		for _, name := range []string{"instance-seed", "catalog-runtime", "github-catalog-source"} {
			if _, err := os.Lstat(filepath.Join(source, name)); err == nil {
				sources = append(sources, source)
				break
			} else if !os.IsNotExist(err) {
				return nil, errors.WrapIO("inspect", source, err)
			}
		}
	}
	if len(sources) == 0 {
		return nil, nil
	}
	completion, err := runtime.ReadDirectoryMigrationCompletion(ctx, paths.Runtime.Path)
	if err != nil {
		return nil, errors.WrapResource("verify", "legacy runtime migration", paths.Runtime.Path, err)
	}
	owner := runtime.DirectoryOwner{Product: "starmap", Deployment: paths.DeploymentID, Instance: paths.InstanceID}
	if completion.Owner != owner || completion.SchedulerIdentity != paths.SchedulerIdentity {
		return nil, migrationSelectionConflict("legacy migration does not match the configured owner and identity")
	}
	for _, source := range sources {
		if source != completion.SourceDirectory {
			return nil, &errors.ConflictError{Resource: "product directory migration", Expected: completion.SourceDirectory, Actual: source, Message: "another legacy runtime requires an explicit verified migration"}
		}
	}
	return &completion, nil
}
