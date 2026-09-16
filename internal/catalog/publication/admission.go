// Package publication selects scoped evidence under an explicit publication policy.
package publication

import (
	"slices"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/sources"
)

// Scope names one source and, for providers, its exact acquisition binding.
type Scope struct {
	Source  sources.ID
	Binding *sources.ProviderAcquisitionBinding
}

// DisabledAction declares what a disabled scope permits the publisher to remove.
type DisabledAction string

const (
	// Preserve keeps catalog entries when their acquisition scope becomes disabled.
	Preserve DisabledAction = "preserve"
	// Remove permits removal of only the disabled scope's entries.
	Remove DisabledAction = "remove"
)

// ScopePolicy declares evidence requirements without inferring them from credentials.
type ScopePolicy struct {
	Scope                 Scope
	Required              bool
	Enabled               bool
	AllowMissing          bool
	AllowRecordQuarantine bool
	AllowStaleRetained    bool
	MaxRetainedAge        time.Duration
	DisabledAction        DisabledAction
}

// Profile records the policy revision and every selected source scope.
type Profile struct {
	Version string
	Scopes  []ScopePolicy
}

// AttemptOutcome reports the source operation independently of retained evidence.
type AttemptOutcome string

const (
	// Succeeded identifies complete, valid evidence observed during this run.
	Succeeded AttemptOutcome = "succeeded"
	// Partial identifies valid evidence with known omissions or degraded data.
	Partial AttemptOutcome = "partial"
	// Failed identifies an operation that produced no valid evidence.
	Failed AttemptOutcome = "failed"
	// MissingCredentials identifies a scope whose acquisition credentials were absent.
	MissingCredentials AttemptOutcome = "missing_credentials"
	// NotAttempted identifies an enabled scope whose collector did not run.
	NotAttempted AttemptOutcome = "not_attempted"
	// Disabled identifies a scope excluded by the declared profile.
	Disabled AttemptOutcome = "disabled"
)

// Attempt holds one explicit source outcome and its direct observation, when present.
type Attempt struct {
	Scope       Scope
	Outcome     AttemptOutcome
	Observation *sources.Observation
}

// Run contains current attempts and previously accepted evidence.
// Callers must authenticate retained evidence before they supply it.
type Run struct {
	StartedAt   time.Time
	CompletedAt time.Time
	Attempts    []Attempt
	Retained    []sources.Observation
}

// EvidenceKind distinguishes new observations from unchanged retained receipts.
type EvidenceKind string

const (
	// FreshEvidence identifies eligible evidence observed during this run.
	FreshEvidence EvidenceKind = "fresh"
	// RetainedEvidence identifies eligible evidence from an earlier accepted run.
	RetainedEvidence EvidenceKind = "retained"
	// StaleRetainedEvidence identifies retained evidence beyond the configured freshness limit.
	StaleRetainedEvidence EvidenceKind = "stale_retained"
	// NoEvidence reports that the scope supplies no input for this publication.
	NoEvidence EvidenceKind = "none"
)

// ScopeResult reports admission without raw upstream errors or credential values.
type ScopeResult struct {
	Scope        Scope
	Required     bool
	Attempt      AttemptOutcome
	EvidenceKind EvidenceKind
	Evidence     *catalogs.SourceObservationLink
	Allowed      bool
	Remove       bool
	Quarantine   *evidence.RecordQuarantine
}

// Decision contains the complete admission result in profile order.
// Rejected runs expose diagnostics but supply no inputs or removal operations.
type Decision struct {
	PolicyVersion    string
	Allowed          bool
	FreshAcquisition bool
	Scopes           []ScopeResult
	Inputs           []sources.Observation
	Removals         []Scope
}

// Admit selects complete evidence or explicitly permitted record quarantine within each scope.
// It makes no network requests and leaves catalog facts and publication heads unchanged.
func Admit(profile Profile, run Run) (Decision, error) {
	input, err := validateInputs(profile, run)
	if err != nil {
		return Decision{}, err
	}
	decision := Decision{PolicyVersion: profile.Version, Allowed: true}
	for _, policy := range profile.Scopes {
		key := policy.Scope.key()
		attempt := input.attempts[key]
		row := ScopeResult{
			Scope: policy.Scope.clone(), Required: policy.Required, Attempt: attempt.Outcome,
			EvidenceKind: NoEvidence, Allowed: true,
		}
		if !policy.Enabled {
			row.Remove = policy.DisabledAction == Remove
			if row.Remove {
				decision.Removals = append(decision.Removals, policy.Scope.clone())
			}
		} else {
			observation, kind := selectEvidence(policy, attempt, input.retained[key], run.CompletedAt)
			row.EvidenceKind = kind
			if observation == nil {
				row.Allowed = !policy.Required && policy.AllowMissing
			} else {
				link := observation.Link()
				row.Evidence = &link
				row.Quarantine = quarantinedRecords(*observation)
				decision.Inputs = append(decision.Inputs, cloneObservation(*observation))
				if kind == FreshEvidence && policy.Scope.Source != sources.EmbeddedCatalogID && policy.Scope.Source != sources.ReleaseArtifactID {
					decision.FreshAcquisition = true
				}
			}
		}
		decision.Allowed = decision.Allowed && row.Allowed
		decision.Scopes = append(decision.Scopes, row)
	}
	if !decision.Allowed {
		decision.Inputs = nil
		decision.Removals = nil
	}
	return decision, nil
}

func selectEvidence(policy ScopePolicy, attempt Attempt, retained *sources.Observation, now time.Time) (*sources.Observation, EvidenceKind) {
	if attempt.Outcome == Succeeded || (attempt.Outcome == Partial && attempt.Observation != nil && usableEvidence(policy, *attempt.Observation)) {
		return attempt.Observation, FreshEvidence
	}
	if retained != nil && usableEvidence(policy, *retained) && policy.MaxRetainedAge > 0 {
		if now.Sub(retained.ObservedAt) <= policy.MaxRetainedAge {
			return retained, RetainedEvidence
		}
		if policy.AllowStaleRetained {
			return retained, StaleRetainedEvidence
		}
	}
	return nil, NoEvidence
}

func complete(observation sources.Observation) bool {
	return observation.Completeness == sources.ObservationCompletenessComplete && observation.Status == sources.ObservationStatusSucceeded
}

func cloneObservation(observation sources.Observation) sources.Observation {
	observation.ProviderBinding = (Scope{Binding: observation.ProviderBinding}).clone().Binding
	observation.Issues = slices.Clone(observation.Issues)
	return observation
}
