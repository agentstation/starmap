package workspace

import "github.com/agentstation/starmap/pkg/productpaths/policy"

func requireWorkspaceAccess() error {
	for _, role := range []string{"workspace", "workspace-receipt", "workspace-lock", "workspace-journal", "workspace-backup", "workspace-staging"} {
		if err := policy.Require(role, policy.DeploymentControlled); err != nil {
			return err
		}
	}
	return nil
}
