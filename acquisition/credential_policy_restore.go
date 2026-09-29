package acquisition

import (
	"context"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productfiles"
)

// InspectCredentialPolicyState validates existing policy without writes or credential access.
// Product selects the policy family. The retained default takes precedence over LegacyInstallation.
// The caller must fence writers and verify the complete inventory around this inspection.
// A valid policy does not approve replica reuse, acquisition, or inference.
func InspectCredentialPolicyState(ctx context.Context, product CredentialProduct, state CredentialPolicyState) error {
	current, err := credentialPolicyFamily(product)
	if err != nil {
		return err
	}
	return auth.InspectFilePolicyStore(ctx, state.Directory, auth.PolicyOwner{
		Product: state.Product, Deployment: state.DeploymentID, Instance: state.InstanceID,
	}, current)
}

// InspectCredentialPolicyPublications selects verified publication evidence to keep inactive.
// The caller preserves selected files and validates the remaining policy before publication.
// This check does not accept a migration or authorize credential use.
func InspectCredentialPolicyPublications(ctx context.Context, product CredentialProduct, state CredentialPolicyState, files map[string]productfiles.RetainedFile, read productfiles.RetainedRecordReader) ([]string, error) {
	current, err := credentialPolicyFamily(product)
	if err != nil {
		return nil, err
	}
	return auth.InspectPolicyPublications(ctx, auth.PolicyOwner{Product: state.Product, Deployment: state.DeploymentID, Instance: state.InstanceID}, current, files, read)
}

func credentialPolicyFamily(product CredentialProduct) (auth.EnvironmentPolicy, error) {
	current := auth.EnvironmentPolicyCurrent
	switch product {
	case "", CredentialProductStarmap:
	case CredentialProductStarport:
		current = auth.EnvironmentPolicyStarportCurrent
	default:
		return "", &errors.ValidationError{Field: "acquisition.credentials.product", Message: "is not supported"}
	}
	return current, nil
}
