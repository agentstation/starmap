package acquisition

import (
	"context"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/pkg/errors"
)

// InspectCredentialPolicyState validates existing policy without writes or credential access.
// Product selects the policy family. The retained default takes precedence over LegacyInstallation.
// The caller must fence writers and verify the complete inventory around this inspection.
// A valid policy does not approve replica reuse, acquisition, or inference.
func InspectCredentialPolicyState(ctx context.Context, product CredentialProduct, state CredentialPolicyState) error {
	current := auth.EnvironmentPolicyCurrent
	switch product {
	case "", CredentialProductStarmap:
	case CredentialProductStarport:
		current = auth.EnvironmentPolicyStarportCurrent
	default:
		return &errors.ValidationError{Field: "acquisition.credentials.product", Message: "is not supported"}
	}
	return auth.InspectFilePolicyStore(ctx, state.Directory, auth.PolicyOwner{
		Product: state.Product, Deployment: state.DeploymentID, Instance: state.InstanceID,
	}, current)
}
