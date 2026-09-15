package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
)

// credentialPolicy initializes selection history before catalog startup creates installation evidence.
func (a *App) credentialPolicy(ctx context.Context) (*auth.FilePolicyStore, error) {
	a.policyMu.Lock()
	defer a.policyMu.Unlock()
	if a.policyStore != nil {
		return a.policyStore, nil
	}
	paths, err := a.ResolvedPaths()
	if err != nil {
		return nil, err
	}
	initial := auth.EnvironmentPolicyCurrent
	for _, marker := range []struct {
		path      string
		directory bool
	}{
		{path: filepath.Join(paths.CatalogStore.Path, "current")},
		{path: filepath.Join(paths.Runtime.Path, "instance-seed")},
		{path: paths.Baselines.Path, directory: true},
	} {
		// Windows can report a path beneath a regular file as absent.
		// Check every marker's route before persistent policy initialization.
		if err := privatefiles.ValidateAncestors(marker.path); err != nil {
			return nil, errors.WrapIO("inspect credential migration state", marker.path, err)
		}
		info, err := os.Lstat(marker.path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, errors.WrapIO("inspect credential migration state", marker.path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || info.IsDir() != marker.directory || !marker.directory && !info.Mode().IsRegular() {
			return nil, &errors.ValidationError{
				Field: "credential_policy.installation", Value: marker.path,
				Message: "installation marker has an unexpected file type",
			}
		}
		initial = auth.EnvironmentPolicyLegacy
	}
	store, err := auth.OpenFilePolicyStore(ctx, paths.CredentialPolicy.Path, auth.PolicyOwner{
		Product: "starmap", Deployment: paths.DeploymentID, Instance: paths.InstanceID,
	}, initial)
	if err != nil {
		return nil, err
	}
	a.policyStore = store
	return store, nil
}
