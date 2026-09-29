package runtime

import (
	"context"
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// ValidateFleetRecovery checks retained input structure without opening a runtime.
// It grants no permission and does not check compatibility with deployment settings.
func ValidateFleetRecovery(ctx context.Context, snapshot FleetSnapshot) error {
	if ctx == nil {
		return &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := snapshot.Validate(); err != nil {
		return err
	}
	record, err := readFleetRecovery(ctx, snapshot.Publication.Recovery.Data)
	if err != nil {
		return err
	}
	if record.Pin != nil && !pinRecordMatches(*record.Pin, snapshot.Publication.Generation) {
		return pinRecordConflict("the shared pin receipt differs from the selected generation")
	}
	_, err = decodeFleetRecoveryRecord(ctx, record, layerSet{})
	return err
}

// ValidateFleetReplay reproduces a retained catalog under the supplied runtime settings.
// It starts no runtime, acquisition, clock observation, filesystem access, or lease.
// Successful validation does not renew a receipt or authorize an inference attempt.
func ValidateFleetReplay(ctx context.Context, snapshot FleetSnapshot, opts ...Option) error {
	if ctx == nil {
		return &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := snapshot.Validate(); err != nil {
		return err
	}
	return validateCatalogReplay(ctx, snapshot.Publication.Generation, snapshot.Publication.Recovery, opts...)
}

// ValidateCatalogReplay checks exact retained inputs against the supplied deployment settings.
// It starts no acquisition, writes, clock observation, or lease operation.
// Success grants no serving permission and does not prove independent recovery history.
func ValidateCatalogReplay(ctx context.Context, generation catalogs.Generation, recovery CatalogRecovery, opts ...Option) error {
	if ctx == nil {
		return invalidInputPublication("replay requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := recovery.Validate(generation); err != nil {
		return err
	}
	return validateCatalogReplay(ctx, generation, recovery.Inputs, opts...)
}

func validateCatalogReplay(ctx context.Context, generation catalogs.Generation, recovery FleetRecovery, opts ...Option) error {
	config := defaults()
	if _, err := config.apply(opts...); err != nil {
		return err
	}
	config.resolve()
	if err := config.validate(); err != nil {
		return err
	}
	if err := config.validateStoredAuthoritySelection(generation.Manifest.AuthorityHead); err != nil {
		return err
	}
	description, err := describeSources(config)
	if err != nil {
		return err
	}
	probe := &Runtime{config: *config, source: config.customSource}
	// File sources use their protocol identity, without reading the selected path.
	if probe.source == nil && config.source.Kind == SourceFile {
		probe.source = &fileSource{}
	}
	local := layerSet{requireAuthority: probe.requiresAuthority(), providerBindings: config.providerBindings,
		acquisitionSources: config.acquisitionSources, publisherAliases: slices.Clone(config.source.Aliases), sourceConfiguration: description}
	recovered, pin, err := recoverCatalogState(ctx, generation, recovery, local)
	if err != nil {
		return err
	}
	if pin != nil && pin.Binding != probe.pinBinding() {
		return pinRecordConflict("the shared pin belongs to a different source or authority")
	}
	identity := config.source.SafeIdentity()
	if probe.source != nil {
		identity = probe.source.Identity()
	}
	if recovered.source != nil && recovered.source.Identity != identity {
		return fleetConflict("retained inputs belong to a different configured source")
	}
	return ctx.Err()
}
