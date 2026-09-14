package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/auth"
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
	for _, path := range []string{
		filepath.Join(paths.CatalogStore.Path, "current"),
		filepath.Join(paths.Runtime.Path, "instance-seed"),
		paths.Baselines.Path,
	} {
		if _, err := os.Lstat(path); err == nil {
			initial = auth.EnvironmentPolicyLegacy
			break
		} else if !os.IsNotExist(err) {
			return nil, errors.WrapIO("inspect credential migration state", path, err)
		}
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
