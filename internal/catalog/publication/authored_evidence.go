package publication

import (
	"encoding/json"
	"maps"
	"slices"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	catalogevidence "github.com/agentstation/starmap/pkg/catalogs/evidence"
)

// authoredBaselineEvidence preserves baseline evidence and explicit review edits.
// Unchanged acquired evidence remains in the source history, outside the baseline.
func authoredBaselineEvidence(catalog *catalogs.Catalog, original, published, edited catalogs.GenerationManifest) (starmap.CandidateEvidence, error) {
	views := make([]map[string]json.RawMessage, 3)
	for index, manifest := range []catalogs.GenerationManifest{original, published, edited} {
		raw, err := json.Marshal(manifest.ReviewCandidates)
		if err != nil {
			return starmap.CandidateEvidence{}, err
		}
		views[index], err = indexAuthoredRecords(raw,
			[]string{"provider_id", "provider_model_id", "code", "source", "source_observation_id"})
		if err != nil {
			return starmap.CandidateEvidence{}, err
		}
	}
	records, err := applyAuthoredMap(views[0], views[1], views[2], false)
	if err != nil {
		return starmap.CandidateEvidence{}, err
	}
	result := starmap.CandidateEvidence{}
	referenced := make(map[string]bool)
	for _, key := range slices.Sorted(maps.Keys(records)) {
		var review catalogevidence.ReviewCandidate
		if err := json.Unmarshal(records[key], &review); err != nil {
			return starmap.CandidateEvidence{}, admissionError("baseline.review", "cannot decode the authored review")
		}
		result.ReviewCandidates = append(result.ReviewCandidates, review)
		referenced[review.SourceObservationID] = true
	}
	for _, entries := range catalog.Provenance().Map() {
		for _, entry := range entries {
			if entry.ObservationID != "" {
				referenced[entry.ObservationID] = true
			}
		}
	}
	for _, scope := range catalog.MembershipScopes() {
		if scope.Inventory != nil {
			referenced[scope.Inventory.ObservationID] = true
		}
		for _, addition := range scope.Additions {
			referenced[addition.ObservationID] = true
		}
	}
	links := make(map[string]catalogs.SourceObservationLink)
	for _, link := range original.SourceObservations {
		links[authoredRecordKey(link.Source.String(), link.ObservationID)] = link
	}
	for _, link := range edited.SourceObservations {
		key := authoredRecordKey(link.Source.String(), link.ObservationID)
		if previous, found := links[key]; found && previous != link {
			return starmap.CandidateEvidence{}, admissionError("baseline.source_observations", "cannot change a retained observation identity")
		}
		if referenced[link.ObservationID] {
			links[key] = link
		}
	}
	for _, key := range slices.Sorted(maps.Keys(links)) {
		result.SourceObservations = append(result.SourceObservations, links[key])
	}
	return result, nil
}
