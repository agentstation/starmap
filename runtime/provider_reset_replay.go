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

// providerResetSelection retains original observations and selects their permitted records.
// A later batch can accept the same validated observation as new replacement evidence.
type providerResetSelection struct {
	observations map[string]manualObservation
	excluded     map[string]map[catalogs.ProviderID]bool
}

func selectProviderResetHistory(ctx context.Context, history *manualBatch) (*providerResetSelection, error) {
	selected := &providerResetSelection{observations: make(map[string]manualObservation), excluded: make(map[string]map[catalogs.ProviderID]bool)}
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
			if observation.Receipt.Link.Source != sources.ProvidersID {
				continue
			}
			id := observation.Receipt.Link.ObservationID
			selected.observations[id] = observation
			delete(selected.excluded, id)
		}
	}
	return selected, nil
}

func (s *providerResetSelection) permitsProjection(provider catalogs.ProviderID, entry provenance.Entry) bool {
	return entry.Source != sources.ProvidersID || !s.excluded[entry.ObservationID][provider]
}

func (s *providerResetSelection) selection(observations []sources.Observation) map[string][]catalogs.ProviderID {
	selected := make(map[string][]catalogs.ProviderID)
	for _, observation := range observations {
		if observation.SourceID != sources.ProvidersID || len(s.excluded[observation.ID]) == 0 {
			continue
		}
		selected[observation.ID] = []catalogs.ProviderID{}
		for _, provider := range observation.Catalog.Providers().List() {
			if !s.excluded[observation.ID][provider.ID] {
				selected[observation.ID] = append(selected[observation.ID], provider.ID)
			}
		}
		slices.Sort(selected[observation.ID])
	}
	return selected
}

func (s *providerResetSelection) excludesAll(observation sources.Observation) bool {
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

// providerResetChecksum binds accepted reset operations even when payload bytes stay equal.
func providerResetChecksum(checksum string, history *manualBatch) (string, error) {
	type resetIdentity struct {
		Scopes       []ProviderObservationReset
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
