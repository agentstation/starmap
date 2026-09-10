package runtime

import "github.com/agentstation/starmap/pkg/sources"

func providerSourceActivity(result AcquisitionResult, attempted bool, err error) sources.SourceActivity {
	row := sources.SourceActivity{Source: sources.ProvidersID, Supported: true, Enabled: true, Attempted: attempted, Eligibility: sources.EligibilityUnknown}
	unknown := len(result.Attempts) < result.Eligible || (len(result.Attempts) == 0 && (err != nil || len(result.Layers) > 0))
	for _, attempt := range result.Attempts {
		if attempt.Validate() != nil {
			unknown = true
			continue
		}
		if attempt.Requested || attempt.Outcome == sources.ProviderOutcomeSucceeded {
			row.Eligibility = sources.EligibilityEligible
			return row
		}
		switch attempt.Reason {
		case sources.ProviderReasonCredentialUnavailable, sources.ProviderReasonCredentialReferenceInvalid, sources.ProviderReasonCredentialExpired, sources.ProviderReasonInsufficientScope, sources.ProviderReasonDependencyUnavailable:
		default:
			unknown = true
		}
	}
	if !unknown {
		row.Eligibility = sources.EligibilityIneligible
	}
	return row
}
