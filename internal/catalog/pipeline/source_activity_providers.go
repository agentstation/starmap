package pipeline

import (
	"cmp"
	"slices"

	"github.com/agentstation/starmap/pkg/sources"
)

func (r *sourceActivityRun) recordProviderEligibility(attempts []sources.ProviderAttempt, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providerCompleted++
	if len(attempts) == 0 && err != nil {
		r.providerUnknown = true
	}
	for _, attempt := range attempts {
		if attempt.Validate() != nil {
			r.providerUnknown = true
			continue
		}
		r.providerAttempts = append(r.providerAttempts, attempt)
		if attempt.Requested || attempt.Outcome == sources.ProviderOutcomeSucceeded {
			r.providerEligible = true
		} else if !providerPreflightRefusal(attempt.Reason) {
			r.providerUnknown = true
		}
	}
	eligibility := sources.EligibilityUnknown
	if r.providerEligible {
		eligibility = sources.EligibilityEligible
	} else if r.providerCompleted == r.providerExpected && !r.providerUnknown {
		eligibility = sources.EligibilityIneligible
	}
	for i := range r.states {
		if r.states[i].Source == sources.ProvidersID {
			r.states[i].Eligibility = eligibility
		}
	}
}

func providerPreflightRefusal(reason sources.ProviderReason) bool {
	switch reason {
	case sources.ProviderReasonCredentialUnavailable, sources.ProviderReasonCredentialReferenceInvalid, sources.ProviderReasonCredentialExpired, sources.ProviderReasonInsufficientScope, sources.ProviderReasonDependencyUnavailable:
		return true
	default:
		return false
	}
}

func (r *sourceActivityRun) providerReport() []sources.ProviderAttempt {
	r.mu.Lock()
	defer r.mu.Unlock()
	report := slices.Clone(r.providerAttempts)
	slices.SortFunc(report, func(a, b sources.ProviderAttempt) int {
		if order := cmp.Compare(a.ProviderID, b.ProviderID); order != 0 {
			return order
		}
		if order := cmp.Compare(a.BindingID, b.BindingID); order != 0 {
			return order
		}
		return cmp.Compare(a.BindingRevision, b.BindingRevision)
	})
	return report
}
