package auth

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

type referenceResource struct {
	backend  ReferenceBackend
	resource string
}

type referenceSnapshot struct {
	reference Reference
	material  sourceMaterial
}

// referenceResolution holds one source snapshot per resource for a complete profile.
type referenceResolution struct {
	resolver  *Resolver
	snapshots map[referenceResource]referenceSnapshot
}

func (r *Resolver) newReferenceResolution(
	providerID catalogs.ProviderID,
	profile catalogs.ProviderCredentialProfile,
) (*referenceResolution, error) {
	versions := make(map[referenceResource]string)
	for _, fieldID := range profile.Fields {
		policy, exists := r.references[CredentialFieldKey{ProviderID: providerID, FieldID: fieldID}]
		if !exists {
			continue
		}
		reference := policy.Reference
		resource := referenceResource{backend: reference.backend, resource: reference.resource}
		if version, exists := versions[resource]; exists && version != reference.version {
			return nil, &errors.ValidationError{
				Field: "credential_sources.version", Value: fieldID,
				Message: "one profile cannot select different versions of the same secret",
			}
		}
		versions[resource] = reference.version
	}
	return &referenceResolution{resolver: r, snapshots: make(map[referenceResource]referenceSnapshot)}, nil
}

func (r *referenceResolution) resolve(
	ctx context.Context,
	key CredentialFieldKey,
	reference Reference,
) (sourceMaterial, error) {
	resource := referenceResource{backend: reference.backend, resource: reference.resource}
	previous, exists := r.snapshots[resource]
	if exists && (previous.material.snapshot != nil || previous.reference.field == reference.field) {
		return previous.material.copy(), nil
	}
	material, err := r.resolver.resolveReference(ctx, key, reference)
	if err != nil {
		return sourceMaterial{}, err
	}
	if exists && (material.version == "" || material.version != previous.material.version) {
		return sourceMaterial{}, newSourceError(SourceErrorUnavailable, reference.backend)
	}
	r.snapshots[resource] = referenceSnapshot{reference: reference, material: material.copy()}
	return material, nil
}
