package pipeline

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// NewBoundAcquisition creates a prepare-only pipeline with explicit provider scopes.
// It copies the declarations without contacting providers or resolving credentials.
func NewBoundAcquisition(factory sources.ProviderClientFactory, resolver sources.ProviderCredentialResolver, bindings []sources.ProviderAcquisitionBinding) *Pipeline {
	pipeline := newPipeline(factory, resolver)
	owned := slices.Clone(bindings)
	pipeline.providerBindings = &owned
	return pipeline
}

func (p *Pipeline) bindProviderSources(configured []sources.Source, options *pkgsync.Options, inputs catalogInputs) ([]sources.Source, error) {
	if p.providerBindings == nil {
		return configured, nil
	}
	selected := make([]sources.ProviderAcquisitionBinding, 0, len(*p.providerBindings))
	seen := make(map[string]bool, len(*p.providerBindings))
	usesProviders := slices.ContainsFunc(configured, func(source sources.Source) bool { return source.ID() == sources.ProvidersID })
	for _, binding := range *p.providerBindings {
		if err := binding.Validate(); err != nil {
			return nil, err
		}
		if seen[binding.ID] {
			return nil, bindingPipelineError("each binding identity must have exactly one active declaration")
		}
		seen[binding.ID] = true
		if !usesProviders || (options.ProviderID != nil && binding.ProviderID != *options.ProviderID) {
			continue
		}
		provider, _ := inputs.providerConfig.Providers().Get(binding.ProviderID)
		if err := providers.ValidateBinding(provider, binding); err != nil {
			return nil, err
		}
		selected = append(selected, binding)
	}
	if usesProviders && options.ProviderID != nil && len(selected) == 0 {
		return nil, bindingPipelineError("requested provider has no active binding")
	}
	slices.SortFunc(selected, func(left, right sources.ProviderAcquisitionBinding) int {
		if order := strings.Compare(string(left.ProviderID), string(right.ProviderID)); order != 0 {
			return order
		}
		return strings.Compare(left.ID, right.ID)
	})
	gate := make(chan struct{}, constants.MaxConcurrentProviders)
	result := make([]sources.Source, 0, len(configured)+len(selected))
	for _, source := range configured {
		if source.ID() != sources.ProvidersID {
			result = append(result, source)
			continue
		}
		providerSource, ok := source.(*providers.Source)
		if !ok {
			return nil, bindingPipelineError("configured provider source must support declared bindings")
		}
		for _, binding := range selected {
			provider, _ := inputs.providerConfig.Providers().Get(binding.ProviderID)
			result = append(result, &boundProviderSource{Source: providerSource, binding: binding, provider: *provider, gate: gate})
		}
	}
	return result, nil
}

type boundProviderSource struct {
	*providers.Source
	binding  sources.ProviderAcquisitionBinding
	provider catalogs.Provider
	gate     chan struct{}
}

func (s *boundProviderSource) providerBinding() sources.ProviderAcquisitionBinding { return s.binding }

func (s *boundProviderSource) Observe(ctx context.Context, opts ...sources.Option) (sources.Observation, error) {
	options := (&sources.Options{}).Apply(opts...)
	if options.ProviderID != nil && *options.ProviderID != s.binding.ProviderID {
		return sources.Observation{}, bindingPipelineError("provider filter does not match the selected binding")
	}
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	case <-ctx.Done():
		return sources.Observation{}, ctx.Err()
	}
	observation, _, err := s.ObserveBinding(ctx, s.binding)
	if err == nil || ctx.Err() != nil {
		return observation, err
	}
	// Failed calls retain a scoped receipt, never an unscoped replacement.
	provider := catalogs.DeepCopyProvider(s.provider)
	provider.Models = nil
	builder := catalogs.NewEmpty()
	if buildErr := builder.SetProvider(provider); buildErr != nil {
		return sources.Observation{}, buildErr
	}
	candidate, buildErr := catalogs.NewObservationCatalog(builder)
	if buildErr != nil {
		return sources.Observation{}, buildErr
	}
	failed, receiptErr := sources.NewObservation(sources.ProvidersID, candidate, sources.ObservationMetadata{
		ProviderBinding: &s.binding, ObservedAt: time.Now().UTC(),
		Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Status:   sources.ObservationStatusDegraded, Completeness: sources.ObservationCompletenessPartial,
		Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeProvider, Code: sources.ObservationIssueCodeFetchFailed, Subject: string(s.binding.ProviderID), Message: "selected provider binding returned no valid observation"}},
	})
	if receiptErr != nil {
		return sources.Observation{}, receiptErr
	}
	return failed, err
}

type sourceObservationKey struct {
	source  sources.ID
	binding sources.ProviderAcquisitionBinding
}

func configuredObservationKey(source sources.Source) sourceObservationKey {
	key := sourceObservationKey{source: source.ID()}
	if scoped, ok := source.(interface {
		providerBinding() sources.ProviderAcquisitionBinding
	}); ok {
		key.binding = scoped.providerBinding()
	}
	return key
}

func observedSourceKey(observation sources.Observation) sourceObservationKey {
	key := sourceObservationKey{source: observation.SourceID}
	if observation.ProviderBinding != nil {
		key.binding = *observation.ProviderBinding
	}
	return key
}

func bindingPipelineError(message string) error {
	return &errors.ValidationError{Field: "acquisition.provider_bindings", Message: message}
}
