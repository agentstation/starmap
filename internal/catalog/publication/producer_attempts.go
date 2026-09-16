package publication

import (
	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/pkg/sources"
)

func (p *Producer) collectionAttempts(collected *pipeline.Collected) ([]Attempt, error) {
	policies := make(map[scopeKey]ScopePolicy, len(p.profile.Scopes))
	for _, policy := range p.profile.Scopes {
		policies[policy.Scope.key()] = policy
	}
	failedSources, err := p.collectionFailures(collected)
	if err != nil {
		return nil, err
	}
	observations, err := collectionObservations(policies, collected.Observations)
	if err != nil {
		return nil, err
	}
	providerAttempts, err := collectionProviderAttempts(policies, collected.ProviderAttempts)
	if err != nil {
		return nil, err
	}
	attempts := make([]Attempt, 0, len(p.profile.Scopes))
	for _, policy := range p.profile.Scopes {
		attempt := Attempt{Scope: policy.Scope.clone(), Outcome: NotAttempted}
		if !policy.Enabled {
			attempt.Outcome = Disabled
		} else {
			observation := observations[policy.Scope.key()]
			if observation != nil {
				attempt.Outcome, attempt.Observation = Partial, observation
				if complete(*observation) {
					attempt.Outcome = Succeeded
				}
			} else {
				for _, activity := range collected.SourceActivities {
					if activity.Source == policy.Scope.Source && activity.Attempted {
						attempt.Outcome = Failed
					}
				}
			}
			if failedSources[policy.Scope.Source] && (observation == nil || complete(*observation)) {
				attempt.Outcome, attempt.Observation = Failed, nil
			}
			if policy.Scope.Source == sources.ProvidersID {
				provider, exists := providerAttempts[policy.Scope.key()]
				switch {
				case !exists:
					if observation != nil {
						attempt.Outcome, attempt.Observation = Failed, nil
					}
				case provider.Outcome == sources.ProviderOutcomeSkippedNotConfigured && provider.Reason == sources.ProviderReasonCredentialUnavailable:
					attempt.Outcome, attempt.Observation = MissingCredentials, nil
				case provider.Outcome != sources.ProviderOutcomeSucceeded || observation == nil:
					attempt.Outcome, attempt.Observation = Failed, nil
				default:
					attempt.Outcome, attempt.Observation = Partial, observation
					if complete(*observation) {
						attempt.Outcome = Succeeded
					}
				}
			}
		}
		attempts = append(attempts, attempt)
	}
	return attempts, nil
}

func (p *Producer) collectionFailures(collected *pipeline.Collected) (map[sources.ID]bool, error) {
	failures := collected.FailureSummaries()
	if len(collected.SourceFailures) != 0 && len(failures) == 0 {
		return nil, admissionError("collection.failure", "requires a classified source failure")
	}
	failedSources := make(map[sources.ID]bool, len(failures))
	for _, failure := range failures {
		enabled := false
		for _, policy := range p.profile.Scopes {
			enabled = enabled || (policy.Enabled && policy.Scope.Source == failure.Source)
		}
		if !failure.Valid() || !enabled {
			return nil, admissionError("collection.failure", "must match an enabled declared source")
		}
		failedSources[failure.Source] = true
	}
	return failedSources, nil
}

func collectionObservations(policies map[scopeKey]ScopePolicy, input []sources.Observation) (map[scopeKey]*sources.Observation, error) {
	observations := make(map[scopeKey]*sources.Observation, len(input))
	for _, observation := range input {
		scope := Scope{Source: observation.SourceID, Binding: observation.ProviderBinding}
		policy, exists := policies[scope.key()]
		// The shared pipeline observes its embedded input independently of acquisition selection.
		if scope.Source == sources.EmbeddedCatalogID && (!exists || !policy.Enabled) {
			continue
		}
		if !exists || !policy.Enabled || !policy.Scope.equal(scope) {
			return nil, admissionError("collection.scope", "observation must match an enabled declared scope")
		}
		if observations[scope.key()] != nil {
			return nil, admissionError("collection.scope", "cannot repeat an observation scope")
		}
		if err := observation.Validate(); err != nil {
			return nil, admissionError("collection.observation", "must contain valid original evidence")
		}
		owned := cloneObservation(observation)
		observations[scope.key()] = &owned
	}
	return observations, nil
}

func collectionProviderAttempts(policies map[scopeKey]ScopePolicy, input []sources.ProviderAttempt) (map[scopeKey]sources.ProviderAttempt, error) {
	providerAttempts := make(map[scopeKey]sources.ProviderAttempt, len(input))
	for _, attempt := range input {
		key := scopeKey{source: sources.ProvidersID, binding: attempt.BindingID}
		policy, exists := policies[key]
		if !exists || !policy.Enabled || policy.Scope.Binding == nil || attempt.Validate() != nil {
			return nil, admissionError("collection.provider_attempt", "must match an enabled declared provider binding")
		}
		binding := policy.Scope.Binding
		if attempt.ProviderID != binding.ProviderID || attempt.BindingRevision != binding.Revision {
			return nil, admissionError("collection.provider_attempt", "must match the declared provider and binding revision")
		}
		if _, exists := providerAttempts[key]; exists {
			return nil, admissionError("collection.provider_attempt", "cannot repeat a provider attempt")
		}
		providerAttempts[key] = attempt
	}
	return providerAttempts, nil
}
