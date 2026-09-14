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

// ObservationReset names local acquisition evidence to replace.
// Empty SourceID selects provider APIs for compatibility.
type ObservationReset struct {
	// SourceID selects provider APIs or one models.dev source.
	SourceID sources.ID `json:"source,omitempty"`
	// ProviderID selects an original source provider identity. Empty selects an entire metadata source.
	ProviderID catalogs.ProviderID `json:"provider_id"`
	// BindingID selects one declared acquisition binding, or legacy unscoped input when empty.
	BindingID string `json:"binding_id,omitempty"`
	// BindingRevision selects that binding's declared revision.
	BindingRevision string `json:"binding_revision,omitempty"`
}

// ProviderObservationReset retains the provider-oriented spelling of ObservationReset.
type ProviderObservationReset = ObservationReset

const maxObservationResetScopes = 4096

type observationResetKey struct {
	source   sources.ID
	provider providerEvidenceKey
}

func (r ObservationReset) key() observationResetKey {
	return observationResetKey{source: r.SourceID, provider: providerEvidenceKey{providerID: r.ProviderID, bindingID: r.BindingID, revision: r.BindingRevision}}
}

func (r ObservationReset) source() sources.ID {
	if r.SourceID == "" {
		return sources.ProvidersID
	}
	return r.SourceID
}

func resettableSource(source sources.ID) bool {
	return source == sources.ProvidersID || source == sources.ModelsDevHTTPID || source == sources.ModelsDevGitID
}

func prepareObservationResets(input []ObservationReset) ([]ObservationReset, error) {
	if len(input) > maxObservationResetScopes {
		return nil, invalidObservationReset("too many reset scopes")
	}
	resets := slices.Clone(input)
	for i := range resets {
		if resets[i].SourceID == sources.ProvidersID {
			resets[i].SourceID = ""
		}
		if err := resets[i].validate(); err != nil {
			return nil, err
		}
	}
	slices.SortFunc(resets, func(a, b ObservationReset) int {
		if order := strings.Compare(a.SourceID.String(), b.SourceID.String()); order != 0 {
			return order
		}
		return compareProviderEvidenceKeys(a.key().provider, b.key().provider)
	})
	for i := 1; i < len(resets); i++ {
		if resets[i].SourceID == resets[i-1].SourceID && (resets[i] == resets[i-1] || resets[i-1].ProviderID == "") {
			return nil, invalidObservationReset("reset scopes must be unique and must not overlap")
		}
	}
	return resets, nil
}

func (r ObservationReset) validate() error {
	if !resettableSource(r.source()) {
		return invalidObservationReset("only provider APIs and metadata acquisition sources can be reset")
	}
	if r.SourceID != "" && (r.BindingID != "" || r.BindingRevision != "") {
		return invalidObservationReset("metadata reset scopes cannot contain provider bindings")
	}
	if r.ProviderID != "" || r.SourceID == "" {
		if err := validateProviderLayerID(r.ProviderID); err != nil {
			return err
		}
	}
	if (r.BindingID == "") != (r.BindingRevision == "") {
		return invalidObservationReset("binding identity and revision must be supplied together")
	}
	for _, value := range []string{r.BindingID, r.BindingRevision} {
		if len(value) > sources.MaxProviderBindingFieldBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return invalidObservationReset("binding fields must be bounded identifiers without control characters")
		}
	}
	return nil
}

func invalidObservationReset(message string) error {
	return &errors.ValidationError{Field: "observations.reset", Message: message}
}

func (r ObservationReset) matches(receipt sources.ObservationReceipt) bool {
	if receipt.Link.Source != r.source() {
		return false
	}
	if receipt.ProviderBinding == nil {
		return r.BindingID == ""
	}
	binding := receipt.ProviderBinding
	return r.BindingID == binding.ID && r.BindingRevision == binding.Revision && r.ProviderID == binding.ProviderID
}

// validateObservationReplacement requires complete replacement evidence for every reset scope.
func validateObservationReplacement(ctx context.Context, resets []ObservationReset, observations []manualObservation) error {
	if len(resets) == 0 {
		return nil
	}
	restored := make([]sources.Observation, 0, len(observations))
	for _, retained := range observations {
		if err := ctx.Err(); err != nil {
			return err
		}
		observation, err := retained.restore()
		if err != nil {
			return err
		}
		restored = append(restored, observation)
	}
	return validateRestoredObservationReplacement(ctx, resets, restored)
}

func validateRestoredObservationReplacement(ctx context.Context, resets []ObservationReset, observations []sources.Observation) error {
	if len(resets) == 0 {
		return nil
	}
	matched := make(map[observationResetKey]bool, len(resets))
	for _, observation := range observations {
		if err := ctx.Err(); err != nil {
			return err
		}
		if observation.Status != sources.ObservationStatusSucceeded || observation.Completeness != sources.ObservationCompletenessComplete {
			return invalidObservationReset("replacement observations must be complete and successful")
		}
		for _, reset := range resets {
			if !reset.matches(sources.ObservationReceipt{Link: observation.Link(), ProviderBinding: observation.ProviderBinding}) {
				continue
			}
			if _, exists := observation.Catalog.Providers().Get(reset.ProviderID); exists || reset.SourceID != "" {
				matched[reset.key()] = true
			}
		}
	}
	if len(matched) != len(resets) {
		return invalidObservationReset("each reset scope requires matching replacement evidence")
	}
	return nil
}

func observationResetBytes(resets []ObservationReset) (int, error) {
	raw, err := json.Marshal(resets)
	return len(raw), err
}
