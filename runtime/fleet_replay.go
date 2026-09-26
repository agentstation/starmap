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
	}{layers.embedded.GenerationID, layers.embedded.PayloadChecksum, layers.requireAuthority, bindings, acquisition, aliases}
	data, err := json.Marshal(record, json.Deterministic(true), json.FormatNilMapAsNull(true), json.FormatNilSliceAsNull(true))
	if err != nil {
		return "", err
	}
	return fleetRecoveryChecksum(data), nil
}

// recoverFleetLayers rejects incompatible replay before replacing any local inputs.
func recoverFleetLayers(ctx context.Context, snapshot FleetSnapshot, local layerSet) (layerSet, error) {
	if err := ctx.Err(); err != nil {
		return layerSet{}, err
	}
	if err := snapshot.Validate(); err != nil {
		return layerSet{}, err
	}
	record, err := readFleetRecovery(ctx, snapshot.Publication.Recovery.Data)
	if err != nil {
		return layerSet{}, err
	}
	compatibility, err := fleetLayerCompatibility(local)
	if err != nil {
		return layerSet{}, err
	}
	if record.Compatibility != compatibility {
		return layerSet{}, fleetConflict("recovery requires the same baseline and acquisition policy")
	}
	recovered, err := decodeFleetRecoveryRecord(ctx, record)
	if err != nil {
		return layerSet{}, err
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
	if state.PayloadChecksum != snapshot.Publication.Generation.Manifest.Payload.Checksum {
		return layerSet{}, fleetConflict("recovered inputs do not reproduce the accepted catalog")
	}
	return local, nil
}
