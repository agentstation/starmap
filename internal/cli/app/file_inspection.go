package app

import (
	"context"
	"path/filepath"

	"github.com/agentstation/starmap/pkg/productpaths"
	"github.com/agentstation/starmap/runtime"
)

// InspectFiles adds bounded filesystem observations without opening application state.
func (a *App) InspectFiles(ctx context.Context, limit int) (productpaths.FileManifest, error) {
	manifest, err := a.FileManifest()
	if err != nil {
		return productpaths.FileManifest{}, err
	}
	inspection, err := productpaths.InspectManifest(ctx, manifest, limit)
	if err != nil {
		return productpaths.FileManifest{}, err
	}
	for index := range inspection.Observations {
		item := &inspection.Observations[index]
		if item.ID != "runtime-owner" {
			continue
		}
		paths, err := a.ResolvedPaths()
		if err != nil {
			return productpaths.FileManifest{}, err
		}
		owner := runtime.DirectoryOwner{Product: string(manifest.Product), Deployment: manifest.DeploymentID, Instance: manifest.InstanceID}
		status, err := runtime.InspectDirectoryOwnerRecord(ctx, filepath.Dir(item.Path), owner, paths.SchedulerIdentity)
		if err != nil {
			return productpaths.FileManifest{}, err
		}
		item.OwnerBinding = string(status)
	}
	inspection.Limitations = []string{"Metadata observations are not an atomic snapshot.", "Mode bits and DACL policy checks do not establish effective access or ancestor safety.", "Owner bindings compare only the bounded owner record. They do not verify seeds, live locks, or fleet fencing.", "The scan skips observed symbolic-link targets and does not read catalog payloads or credential files."}
	manifest.Inspection = &inspection
	return manifest, nil
}
