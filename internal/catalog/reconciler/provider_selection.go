package reconciler

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

type providerObservationSelection map[string]map[catalogs.ProviderID]bool

// WithProviderObservationSelection selects provider records within original observations.
// Map keys are observation identities. An omitted identity keeps every provider.
// An empty list excludes all its provider records. Original receipts stay unchanged.
//
// Provider APIs and models.dev observations support selection. Baselines and local
// operator observations do not. Reviewed authored definitions remain separate inputs.
func WithProviderObservationSelection(input map[string][]catalogs.ProviderID) Option {
	selected := make(providerObservationSelection, len(input))
	for id, providers := range input {
		selected[id] = make(map[catalogs.ProviderID]bool, len(providers))
		for _, provider := range providers {
			if provider == "" || selected[id][provider] {
				return func(*options) error {
					return &errors.ValidationError{Field: "reconciliation.provider_selection", Message: "provider identities must be nonempty and unique"}
				}
			}
			selected[id][provider] = true
		}
	}
	return func(options *options) error {
		if _, exists := selected[""]; exists {
			return &errors.ValidationError{Field: "reconciliation.provider_selection", Message: "observation identities must be nonempty"}
		}
		options.providerSelection = selected
		return nil
	}
}

func (s providerObservationSelection) permits(observation sources.Observation, provider catalogs.ProviderID) bool {
	if observation.SourceID == sources.ProvidersID {
		for _, issue := range observation.Issues {
			if issue.Scope != sources.ObservationIssueScopeProvider || issue.Subject != string(provider) {
				continue
			}
			switch issue.Code {
			case sources.ObservationIssueCodeMissingCredentials, sources.ObservationIssueCodeConfiguration,
				sources.ObservationIssueCodeFetchFailed, sources.ObservationIssueCodeSchemaDrift:
				return false
			}
		}
	}
	selected, exists := s[observation.ID]
	return !exists || selected[provider]
}

func (s providerObservationSelection) excludesObservation(observation sources.Observation) bool {
	selected, exists := s[observation.ID]
	return exists && len(selected) == 0
}

// validate checks original receipts before a selection can hide any record.
func (s providerObservationSelection) validate(ctx context.Context, observations []sources.Observation) error {
	if len(s) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(s))
	for _, observation := range observations {
		if err := ctx.Err(); err != nil {
			return err
		}
		providers, selected := s[observation.ID]
		if !selected {
			continue
		}
		if (observation.SourceID != sources.ProvidersID && !isModelsDevSource(observation.SourceID)) || seen[observation.ID] {
			return &errors.ValidationError{Field: "reconciliation.provider_selection", Message: "each selected identity must name one acquisition observation"}
		}
		if err := observation.Validate(); err != nil {
			return err
		}
		seen[observation.ID] = true
		for provider := range providers {
			if _, exists := observation.Catalog.Providers().Get(provider); !exists {
				return &errors.ValidationError{Field: "reconciliation.provider_selection", Message: "selected provider is absent from its original observation"}
			}
		}
	}
	if len(seen) != len(s) {
		return &errors.ValidationError{Field: "reconciliation.provider_selection", Message: "selected observation is absent from reconciliation inputs"}
	}
	return nil
}
