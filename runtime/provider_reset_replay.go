package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// observationResetSelection retains original observations and selects their permitted records.
// A later batch can accept the same validated observation as new replacement evidence.
type observationResetSelection struct {
	observations map[string]manualObservation
	excluded     map[string]map[catalogs.ProviderID]bool
	latest       map[string]*manualBatch
	aliases      map[catalogs.ProviderID][]catalogs.ProviderID
}

func selectObservationResetHistory(ctx context.Context, history *manualBatch, baseline *catalogs.Catalog) (*observationResetSelection, error) {
	selected := &observationResetSelection{observations: make(map[string]manualObservation), excluded: make(map[string]map[catalogs.ProviderID]bool), latest: make(map[string]*manualBatch), aliases: make(map[catalogs.ProviderID][]catalogs.ProviderID)}
	for _, provider := range baseline.Providers().List() {
		selected.aliases[provider.ID] = provider.Aliases
	}
	for _, batch := range manualBatches(history) {
		for _, reset := range batch.resets {
			for id, observation := range selected.observations {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if !reset.matches(observation.Receipt) {
					continue
				}
				if selected.excluded[id] == nil {
					selected.excluded[id] = make(map[catalogs.ProviderID]bool)
				}
				selected.excluded[id][reset.ProviderID] = true
			}
		}
		for _, observation := range batch.observations {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !resettableSource(observation.Receipt.Link.Source) {
				continue
			}
			id := observation.Receipt.Link.ObservationID
			selected.observations[id] = observation
			selected.latest[id] = batch
			delete(selected.excluded, id)
		}
	}
	return selected, nil
}

func (s *observationResetSelection) permitsProjection(provider catalogs.ProviderID, entry provenance.Entry) bool {
	if !resettableSource(entry.Source) {
		return true
	}
	excluded := s.excluded[entry.ObservationID]
	if excluded[""] || excluded[provider] {
		return false
	}
	if entry.Source != sources.ProvidersID {
		for _, alias := range s.aliases[provider] {
			if excluded[alias] {
				return false
			}
		}
	}
	return true
}

func (s *observationResetSelection) selection(observations []sources.Observation) map[string][]catalogs.ProviderID {
	selected := make(map[string][]catalogs.ProviderID)
	for _, observation := range observations {
		if !resettableSource(observation.SourceID) || len(s.excluded[observation.ID]) == 0 {
			continue
		}
		selected[observation.ID] = []catalogs.ProviderID{}
		for _, provider := range observation.Catalog.Providers().List() {
			if !s.excluded[observation.ID][""] && !s.excluded[observation.ID][provider.ID] {
				selected[observation.ID] = append(selected[observation.ID], provider.ID)
			}
		}
		slices.Sort(selected[observation.ID])
	}
	return selected
}

func (s *observationResetSelection) excludesAll(observation sources.Observation) bool {
	if s.excluded[observation.ID][""] {
		return true
	}
	// A provider-only metadata reset retains independent authored definitions.
	if observation.SourceID != sources.ProvidersID {
		return false
	}
	if len(s.excluded[observation.ID]) == 0 {
		return false
	}
	for _, provider := range observation.Catalog.Providers().List() {
		if !s.excluded[observation.ID][provider.ID] {
			return false
		}
	}
	return true
}

// observationResetChecksum binds accepted reset operations even when payload bytes stay equal.
func observationResetChecksum(checksum string, history *manualBatch) (string, error) {
	type resetIdentity struct {
		Scopes       []ObservationReset
		Replacements []string
	}
	var resets []resetIdentity
	for _, batch := range manualBatches(history) {
		if len(batch.resets) == 0 {
			continue
		}
		entry := resetIdentity{Scopes: batch.resets}
		for _, observation := range batch.observations {
			entry.Replacements = append(entry.Replacements, observation.Receipt.Link.ObservationID)
		}
		slices.Sort(entry.Replacements)
		resets = append(resets, entry)
	}
	if len(resets) == 0 {
		return checksum, nil
	}
	raw, err := json.Marshal(struct {
		Domain, Checksum string
		Resets           []resetIdentity
	}{"starmap-provider-resets:v1", checksum, resets})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
