package runtime

import (
	"context"
	"reflect"

	"github.com/agentstation/starmap/pkg/catalogs"
)

const removalPolicyName = "removals.json"
const removalPolicyVersion = 1

type removalPolicyRecord struct {
	Version int                           `json:"version"`
	Policy  catalogs.CatalogRemovalPolicy `json:"policy"`
}

func validateRemovalRecord(record removalPolicyRecord) (*catalogs.CatalogRemovalPolicy, error) {
	if record.Version != removalPolicyVersion {
		return nil, invalidInputPublication("unsupported operator removal version")
	}
	builder := catalogs.NewEmpty()
	if err := builder.SetRemovalPolicies([]catalogs.CatalogRemovalPolicy{record.Policy}); err != nil {
		return nil, err
	}
	policy := builder.RemovalPolicies()[0]
	return &policy, nil
}

func (s *layerStore) loadRemovals() (*catalogs.CatalogRemovalPolicy, error) {
	if !s.durable() {
		return nil, nil
	}
	raw, err := readLayerFile(s.directory, removalPolicyName)
	if err != nil || raw == nil {
		return nil, err
	}
	var record removalPolicyRecord
	if err := decodeInputRecord(raw, &record); err != nil {
		return nil, err
	}
	return validateRemovalRecord(record)
}

func (s *layerStore) readRemovalInput(reference string) (*catalogs.CatalogRemovalPolicy, error) {
	if reference == "" {
		return nil, nil
	}
	directory, err := s.directory.ExistingChild(inputPublicationDirectory)
	if err != nil {
		return nil, err
	}
	var record removalPolicyRecord
	if err := s.readInput(directory, reference, &record); err != nil {
		return nil, err
	}
	return validateRemovalRecord(record)
}

func (s *layerStore) saveRemovals(ctx context.Context, policy *catalogs.CatalogRemovalPolicy) error {
	if policy == nil || !s.durable() {
		return ctx.Err()
	}
	return s.writeContext(ctx, s.directory, removalPolicyName, removalPolicyRecord{Version: removalPolicyVersion, Policy: *policy})
}

// checkRetainedRemovals prevents missing or changed private state from restoring accepted removals.
func checkRetainedRemovals(current *catalogs.Catalog, publisher string, retained *catalogs.CatalogRemovalPolicy) error {
	for _, policy := range current.RemovalPolicies() {
		if policy.PublisherID == publisher && (retained == nil || !reflect.DeepEqual(policy, *retained)) {
			return invalidInputPublication("retained operator removal state differs from the accepted catalog")
		}
	}
	return nil
}
