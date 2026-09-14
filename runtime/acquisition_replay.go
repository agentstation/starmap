package runtime

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// ReplayAcquisition rebuilds an ordered observation history above an explicit baseline.
// The caller authenticates the baseline and observations, and selects the active bindings.
// The baseline can contain this publisher's prior scopes but cannot carry enterprise authority.
// This function reads no sources or storage and starts no runtime workers.
func ReplayAcquisition(ctx context.Context, baseline catalogs.Generation, publisherID string, bindings []sources.ProviderAcquisitionBinding, observations []sources.Observation) (*starmap.Candidate, error) {
	if ctx == nil {
		return nil, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if publisherID == "" || len(publisherID) > sources.MaxProviderBindingFieldBytes || !utf8.ValidString(publisherID) || strings.TrimSpace(publisherID) != publisherID || strings.ContainsFunc(publisherID, unicode.IsControl) {
		return nil, &errors.ValidationError{Field: "replay.publisher_id", Message: "requires a bounded publisher identity"}
	}
	if baseline.Manifest.AuthorityHead != (catalogs.CatalogAuthorityHead{}) {
		return nil, &errors.ValidationError{Field: "replay.baseline", Message: "cannot carry enterprise authority"}
	}
	if len(observations) > maxManualHistoryBatches {
		return nil, &errors.ValidationError{Field: "replay.observations", Message: "exceeds the retained history bound"}
	}
	catalog, err := catalogs.DecodeCatalogGeneration(baseline)
	if err != nil {
		return nil, err
	}
	config := &options{}
	if err := WithProviderBindings(bindings...)(config); err != nil {
		return nil, err
	}
	layers := layerSet{publisherID: publisherID, providerBindings: config.providerBindings,
		embedded: starmap.CatalogState{Catalog: catalog, GenerationID: baseline.Manifest.GenerationID, PayloadChecksum: baseline.Manifest.Payload.Checksum, GeneratedAt: baseline.Manifest.GeneratedAt},
		source:   &sourceLayer{Manifest: &baseline.Manifest, GenerationID: baseline.Manifest.GenerationID, Checksum: baseline.Manifest.Payload.Checksum, Payload: baseline.Payload, PublishedAt: baseline.Manifest.GeneratedAt},
	}
	bytes := 0
	seen := make(map[string]bool, len(observations))
	for _, input := range observations {
		if seen[input.ID] {
			return nil, &errors.ValidationError{Field: "replay.observations", Message: "contains a duplicate observation identity"}
		}
		seen[input.ID] = true
		original, err := prepareManualObservations(ctx, []sources.Observation{input})
		if err != nil {
			return nil, err
		}
		if err := config.providerBindings.validateManual(original, true); err != nil {
			return nil, err
		}
		observation := original[0]
		if len(observation.Payload) > maxLayerBytes-bytes {
			return nil, &errors.ValidationError{Field: "replay.observations", Message: "exceeds the retained payload bound"}
		}
		bytes += len(observation.Payload)
		layers.manual = &manualBatch{parent: layers.manual, observations: original}
	}
	state, err := layers.buildOnBaseline(ctx, layers.embedded, 0)
	if err != nil {
		return nil, err
	}
	if len(layers.buildEvidence.SourceObservations) == 0 {
		layers.buildEvidence.SourceObservations = baseline.Manifest.Copy().SourceObservations
	}
	if err := preserveReplayReviews(state.Catalog, baseline.Manifest, &layers.buildEvidence); err != nil {
		return nil, err
	}
	identity, err := effectiveEvidenceChecksum(state.PayloadChecksum, layers.buildEvidence)
	if err != nil {
		return nil, err
	}
	state.GenerationID = deriveEffectiveGenerationID(state.GenerationID, identity)
	return starmap.NewCandidate(state.Catalog, layers.buildEvidence, starmap.WithCandidateGenerationID(state.GenerationID))
}

func preserveReplayReviews(catalog *catalogs.Catalog, baseline catalogs.GenerationManifest, collected *starmap.CandidateEvidence) error {
	referenced := make(map[string]bool)
	for _, review := range baseline.ReviewCandidates {
		if _, err := catalog.Offering(catalogs.ProviderID(review.ProviderID), catalogs.ProviderModelID(review.ProviderModelID)); err == nil {
			continue
		}
		collected.ReviewCandidates = append(collected.ReviewCandidates, review)
		referenced[review.SourceObservationID] = true
	}
	existing := make(map[string]catalogs.SourceObservationLink)
	for _, link := range collected.SourceObservations {
		existing[link.ObservationID] = link
	}
	for _, link := range baseline.SourceObservations {
		if !referenced[link.ObservationID] {
			continue
		}
		if prior, found := existing[link.ObservationID]; found {
			if prior != link {
				return &errors.ConflictError{Resource: "review observation", Message: "one identity has conflicting source evidence"}
			}
			continue
		}
		collected.SourceObservations = append(collected.SourceObservations, link)
	}
	compactManualEvidence(collected)
	return nil
}
