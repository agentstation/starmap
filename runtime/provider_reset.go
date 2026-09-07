package runtime

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// ProviderObservationReset names one provider acquisition scope to replace.
// Empty binding fields select legacy unscoped observations only.
type ProviderObservationReset struct {
	// ProviderID names the canonical provider whose prior local observations are reset.
	ProviderID catalogs.ProviderID `json:"provider_id"`
	// BindingID selects one declared acquisition binding, or legacy unscoped input when empty.
	BindingID string `json:"binding_id,omitempty"`
	// BindingRevision selects that binding's declared revision.
	BindingRevision string `json:"binding_revision,omitempty"`
}

const maxProviderResetScopes = 4096

func (r ProviderObservationReset) key() providerEvidenceKey {
	return providerEvidenceKey{providerID: r.ProviderID, bindingID: r.BindingID, revision: r.BindingRevision}
}

func prepareProviderResets(input []ProviderObservationReset) ([]ProviderObservationReset, error) {
	if len(input) > maxProviderResetScopes {
		return nil, invalidProviderReset("too many reset scopes")
	}
	resets := slices.Clone(input)
	for _, reset := range resets {
		if err := validateProviderLayerID(reset.ProviderID); err != nil {
			return nil, err
		}
		if (reset.BindingID == "") != (reset.BindingRevision == "") {
			return nil, invalidProviderReset("binding identity and revision must be supplied together")
		}
		for _, value := range []string{reset.BindingID, reset.BindingRevision} {
			if len(value) > sources.MaxProviderBindingFieldBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value || strings.IndexFunc(value, unicode.IsControl) >= 0 {
				return nil, invalidProviderReset("binding fields must be bounded identifiers without control characters")
			}
		}
	}
	slices.SortFunc(resets, func(a, b ProviderObservationReset) int { return compareProviderEvidenceKeys(a.key(), b.key()) })
	for i := 1; i < len(resets); i++ {
		if resets[i] == resets[i-1] {
			return nil, invalidProviderReset("reset scopes must be unique")
		}
	}
	return resets, nil
}

func invalidProviderReset(message string) error {
	return &errors.ValidationError{Field: "observations.reset", Message: message}
}

func (r ProviderObservationReset) matches(receipt sources.ObservationReceipt) bool {
	if receipt.Link.Source != sources.ProvidersID {
		return false
	}
	if receipt.ProviderBinding == nil {
		return r.BindingID == ""
	}
	binding := receipt.ProviderBinding
	return r.BindingID == binding.ID && r.BindingRevision == binding.Revision && r.ProviderID == binding.ProviderID
}

// validateProviderReplacement requires complete replacement evidence for every reset scope.
func validateProviderReplacement(ctx context.Context, resets []ProviderObservationReset, observations []manualObservation) error {
	if len(resets) == 0 {
		return nil
	}
	matched := make(map[providerEvidenceKey]bool, len(resets))
	for _, retained := range observations {
		if err := ctx.Err(); err != nil {
			return err
		}
		observation, err := retained.restore()
		if err != nil {
			return err
		}
		if observation.Status != sources.ObservationStatusSucceeded || observation.Completeness != sources.ObservationCompletenessComplete {
			return invalidProviderReset("replacement observations must be complete and successful")
		}
		for _, reset := range resets {
			if !reset.matches(retained.Receipt) {
				continue
			}
			if _, exists := observation.Catalog.Providers().Get(reset.ProviderID); exists {
				matched[reset.key()] = true
			}
		}
	}
	if len(matched) != len(resets) {
		return invalidProviderReset("each reset scope requires matching replacement evidence")
	}
	return nil
}

func providerResetBytes(resets []ProviderObservationReset) (int, error) {
	raw, err := json.Marshal(resets)
	return len(raw), err
}
