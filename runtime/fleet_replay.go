package runtime

import (
	"context"
	"encoding/json/v2"
	"slices"

	"github.com/agentstation/starmap/pkg/sources"
)

// fleetLayerCompatibility binds replay to the baseline and declared acquisition policy.
// Nil and explicit empty selections remain different. No credential value enters this record.
func fleetLayerCompatibility(layers layerSet) (string, error) {
	var bindings map[string]sources.ProviderAcquisitionBinding
	if layers.providerBindings != nil {
		bindings = layers.providerBindings.bindings
	}
	var acquisition []sources.ID
	if layers.acquisitionSources != nil {
		acquisition = append([]sources.ID{}, layers.acquisitionSources.ids...)
	}
	aliases := slices.Clone(layers.publisherAliases)
	slices.Sort(aliases)
	record := struct {
		BaselineID, BaselineChecksum string
		RequireAuthority             bool
		Bindings                     map[string]sources.ProviderAcquisitionBinding
		Sources                      []sources.ID
		Aliases                      []string
		Capabilities                 []sources.SourceActivity
	}{layers.embedded.GenerationID, layers.embedded.PayloadChecksum, layers.requireAuthority, bindings, acquisition, aliases, layers.sourceConfiguration}
	data, err := json.Marshal(record, json.Deterministic(true), json.FormatNilMapAsNull(true), json.FormatNilSliceAsNull(true))
	if err != nil {
		return "", err
	}
	return fleetRecoveryChecksum(data), nil
}

// recoverFleetState rejects incompatible replay before replacing any local inputs.
func recoverFleetState(ctx context.Context, snapshot FleetSnapshot, local layerSet) (layerSet, *generationPinRecord, error) {
	if err := ctx.Err(); err != nil {
		return layerSet{}, nil, err
	}
	if err := snapshot.Validate(); err != nil {
		return layerSet{}, nil, err
	}
	record, err := readFleetRecovery(ctx, snapshot.Publication.Recovery.Data)
	if err != nil {
		return layerSet{}, nil, err
	}
	if record.Pin != nil && !pinRecordMatches(*record.Pin, snapshot.Publication.Generation) {
		return layerSet{}, nil, pinRecordConflict("the shared pin receipt differs from the selected generation")
	}
	layers, err := recoverFleetLayers(ctx, snapshot, local, record)
	return layers, record.Pin, err
}

// recoverFleetLayers validates acquisition replay independently of a valid pin receipt.
func recoverFleetLayers(ctx context.Context, snapshot FleetSnapshot, local layerSet, record fleetRecoveryRecord) (layerSet, error) {
	recovered, err := decodeFleetRecoveryRecord(ctx, record, local)
	if err != nil {
		return layerSet{}, err
	}
	// The packaged baseline is a candidate for explicit promotion, not replay input.
	local.embedded, local.embeddedManifest, local.fleetBaseline = recovered.embedded, recovered.embeddedManifest, recovered.fleetBaseline
	compatibility, err := fleetLayerCompatibility(local)
	if err != nil {
		return layerSet{}, err
	}
	if record.Compatibility != compatibility {
		return layerSet{}, fleetConflict("recovery requires the same baseline and acquisition policy")
	}
	if err := local.providerBindings.validateRetained(recovered.providers); err != nil {
		return layerSet{}, err
	}
	if err := validateManualHistory(recovered.manual, local.providerBindings); err != nil {
		return layerSet{}, err
	}
	local.publisherID, local.source = recovered.publisherID, recovered.source
	local.providers, local.manual, local.removals = recovered.providers, recovered.manual, recovered.removals
	state, err := local.build(ctx, local.embedded)
	if err != nil {
		return layerSet{}, err
	}
	expectedChecksum := snapshot.Publication.Generation.Manifest.Payload.Checksum
	if record.Pin != nil {
		expectedChecksum = record.ReplayChecksum
	}
	if state.PayloadChecksum != expectedChecksum {
		return layerSet{}, fleetConflict("recovered inputs do not reproduce the accepted catalog")
	}
	return local, nil
}
