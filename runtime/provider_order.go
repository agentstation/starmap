package runtime

import (
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// selectProviderEvidence orders validated, owned observations before retention.
// Older retained evidence can advance. Equal observations require no write.
// A regressive or ambiguous batch changes no retained evidence.
func selectProviderEvidence(prepared []ProviderLayer, retained map[providerEvidenceKey]ProviderLayer) ([]ProviderLayer, error) {
	if err := validateProviderScopes(prepared, retained); err != nil {
		return nil, err
	}
	slices.SortFunc(prepared, func(left, right ProviderLayer) int {
		if order := compareProviderEvidenceKeys(left.evidenceKey(), right.evidenceKey()); order != 0 {
			return order
		}
		return left.ObservedAt.Compare(right.ObservedAt)
	})
	latest := make([]ProviderLayer, 0, len(prepared))
	for _, layer := range prepared {
		if prior, exists := retained[layer.evidenceKey()]; exists && layer.ObservedAt.Equal(prior.ObservedAt) && !sameProviderObservation(layer, prior) {
			return nil, conflictingProviderEvidence(layer.ProviderID, "conflicting observations share the retained observation time")
		}
		if len(latest) == 0 || latest[len(latest)-1].evidenceKey() != layer.evidenceKey() {
			latest = append(latest, layer)
			continue
		}
		previous := latest[len(latest)-1]
		if layer.ObservedAt.Equal(previous.ObservedAt) && !sameProviderObservation(layer, previous) {
			return nil, conflictingProviderEvidence(layer.ProviderID, "conflicting observations share one observation time")
		}
		latest[len(latest)-1] = layer
	}
	selected := latest[:0]
	for _, layer := range latest {
		if prior, exists := retained[layer.evidenceKey()]; exists {
			switch layer.ObservedAt.Compare(prior.ObservedAt) {
			case -1:
				return nil, conflictingProviderEvidence(layer.ProviderID, "observation predates retained evidence")
			case 0:
				continue
			}
		}
		selected = append(selected, layer)
	}
	return selected, nil
}

func conflictingProviderEvidence(id catalogs.ProviderID, message string) error {
	return &errors.ConflictError{Resource: "provider evidence " + string(id), Message: message}
}

func sameProviderObservation(left, right ProviderLayer) bool {
	return left.Digest == right.Digest && left.Receipt.Link.ObservationID == right.Receipt.Link.ObservationID
}
