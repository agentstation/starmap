package runtime

import (
	"context"
	"fmt"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// CapturedCatalogRecovery holds structurally checked original descriptor and baseline inputs.
// It proves no archive completeness, publication, target ownership, semantics, or permission.
type CapturedCatalogRecovery struct {
	record localCatalogRecovery
	inputs fleetRecoveryRecord
}

// CapturedCatalogBinding identifies the retained generation that the host must independently verify.
type CapturedCatalogBinding struct {
	ManifestChecksum string
	GenerationID     string
	PayloadChecksum  string
}

// InspectCapturedCatalogRecovery checks original archive bytes without opening a runtime directory.
// The host supplies a bounded reader for the original baseline and verifies the complete archive census.
// Historical inputs receive structural validation. Target replay and permission checks remain separate.
func InspectCapturedCatalogRecovery(ctx context.Context, name string, descriptor []byte, readBaseline func(context.Context, string, int64) ([]byte, error)) (*CapturedCatalogRecovery, error) {
	if readBaseline == nil {
		return nil, invalidInputPublication("captured recovery requires its original baseline reader")
	}
	record, err := decodeLocalCatalogRecovery(ctx, name, descriptor)
	if err != nil {
		return nil, err
	}
	raw, err := readBaseline(ctx, record.BaselineChecksum+".json.gz", MaxFleetRecoveryBytes)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || len(raw) > MaxFleetRecoveryBytes {
		return nil, invalidInputPublication("captured recovery baseline exceeds its byte limit")
	}
	baseline, err := decodeCatalogRecoveryBaseline(ctx, raw, record.BaselineChecksum)
	if err != nil {
		return nil, err
	}
	inputs := record.Inputs
	inputs.Baseline = baseline
	if _, err := decodeFleetRecoveryRecord(ctx, inputs, layerSet{}); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &CapturedCatalogRecovery{record: record, inputs: inputs}, nil
}

// Binding returns checked identity fields without exposing private source inputs.
func (p *CapturedCatalogRecovery) Binding() CapturedCatalogBinding {
	if p == nil {
		return CapturedCatalogBinding{}
	}
	return CapturedCatalogBinding{p.record.ManifestChecksum, p.record.GenerationID, p.record.PayloadChecksum}
}

// RecoveryFor binds the original inputs to an independently retained complete generation.
// It creates a separate encoded copy. It neither replays target state nor renews permission.
func (p *CapturedCatalogRecovery) RecoveryFor(ctx context.Context, generation catalogs.Generation) (CatalogRecovery, error) {
	if p == nil || ctx == nil {
		return CatalogRecovery{}, invalidInputPublication("captured recovery requires checked original inputs")
	}
	if err := ctx.Err(); err != nil {
		return CatalogRecovery{}, err
	}
	if err := p.record.validate(generation); err != nil {
		return CatalogRecovery{}, err
	}
	data, err := encodeFleetRecoveryRecord(p.inputs)
	if err != nil {
		return CatalogRecovery{}, err
	}
	return CatalogRecovery{ManifestChecksum: p.record.ManifestChecksum, Inputs: FleetRecovery{GenerationID: p.record.GenerationID, PayloadChecksum: p.record.PayloadChecksum, Data: data, Checksum: fleetRecoveryChecksum(data)}}, ctx.Err()
}

// Format excludes private source configuration and baseline data from diagnostics.
func (*CapturedCatalogRecovery) Format(state fmt.State, _ rune) {
	_, _ = fmt.Fprint(state, "captured catalog recovery (private)")
}
