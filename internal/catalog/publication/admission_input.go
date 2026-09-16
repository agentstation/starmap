package publication

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

const maxScopes = 4096

type scopeKey struct {
	source  sources.ID
	binding string
}

type admissionInputs struct {
	attempts map[scopeKey]Attempt
	retained map[scopeKey]*sources.Observation
}

func (s Scope) key() scopeKey {
	key := scopeKey{source: s.Source}
	if s.Binding != nil {
		key.binding = s.Binding.ID
	}
	return key
}

func (s Scope) clone() Scope {
	if s.Binding != nil {
		binding := *s.Binding
		s.Binding = &binding
	}
	return s
}

func (s Scope) equal(other Scope) bool {
	if s.Source != other.Source || (s.Binding == nil) != (other.Binding == nil) {
		return false
	}
	return s.Binding == nil || *s.Binding == *other.Binding
}

func (s Scope) validate() error {
	if !s.Source.IsValid() {
		return admissionError("scope.source", "must name a supported source")
	}
	if s.Source == sources.ProvidersID {
		if s.Binding == nil {
			return admissionError("scope.binding", "provider scope requires an explicit acquisition binding")
		}
		return s.Binding.Validate()
	}
	if s.Binding != nil {
		return admissionError("scope.binding", "only provider sources can declare an acquisition binding")
	}
	return nil
}

func validateProfile(profile Profile) (map[scopeKey]ScopePolicy, error) {
	version := profile.Version
	if version == "" || len(version) > 256 || !utf8.ValidString(version) || strings.TrimSpace(version) != version || strings.ContainsFunc(version, unicode.IsControl) {
		return nil, admissionError("policy_version", "requires a bounded revision without control characters or surrounding whitespace")
	}
	if len(profile.Scopes) == 0 || len(profile.Scopes) > maxScopes {
		return nil, admissionError("scopes", "requires between one and 4096 scopes")
	}
	policies := make(map[scopeKey]ScopePolicy, len(profile.Scopes))
	for _, policy := range profile.Scopes {
		if err := policy.Scope.validate(); err != nil {
			return nil, err
		}
		if policy.Required && policy.AllowMissing {
			return nil, admissionError("scope.allow_missing", "cannot permit missing evidence for a required source")
		}
		if policy.MaxRetainedAge < 0 || (policy.AllowStaleRetained && policy.MaxRetainedAge == 0) {
			return nil, admissionError("scope.max_retained_age", "must be nonnegative and positive when stale retention is enabled")
		}
		if policy.DisabledAction != Preserve && policy.DisabledAction != Remove {
			return nil, admissionError("scope.disabled_action", "requires an explicit preserve or remove policy")
		}
		key := policy.Scope.key()
		if _, exists := policies[key]; exists {
			return nil, admissionError("scopes", "cannot repeat a source and binding identity")
		}
		policies[key] = policy
	}
	return policies, nil
}

func validateInputs(profile Profile, run Run) (admissionInputs, error) {
	policies, err := validateProfile(profile)
	if err != nil {
		return admissionInputs{}, err
	}
	if !utcTime(run.StartedAt) || !utcTime(run.CompletedAt) || run.CompletedAt.Before(run.StartedAt) {
		return admissionInputs{}, admissionError("run.time", "requires ordered, nonzero UTC times")
	}
	if len(run.Attempts) != len(policies) || len(run.Retained) > maxScopes {
		return admissionInputs{}, admissionError("run.scopes", "requires one explicit attempt per scope and bounded retained evidence")
	}
	input := admissionInputs{attempts: make(map[scopeKey]Attempt), retained: make(map[scopeKey]*sources.Observation)}
	for _, attempt := range run.Attempts {
		key := attempt.Scope.key()
		policy, found := policies[key]
		if !found || !policy.Scope.equal(attempt.Scope) {
			return admissionInputs{}, admissionError("attempt.scope", "must match the exact declared scope")
		}
		if _, exists := input.attempts[key]; exists {
			return admissionInputs{}, admissionError("attempt.scope", "cannot repeat an attempt scope")
		}
		if err := validateAttempt(policy, attempt, run); err != nil {
			return admissionInputs{}, err
		}
		input.attempts[key] = attempt
	}
	for _, observation := range run.Retained {
		if err := observation.Validate(); err != nil {
			return admissionInputs{}, admissionError("retained.observation", "must contain valid evidence and its original receipt")
		}
		if observation.ObservedAt.After(run.StartedAt) {
			return admissionInputs{}, admissionError("retained.observed_at", "cannot follow the run start")
		}
		scope := Scope{Source: observation.SourceID, Binding: observation.ProviderBinding}
		key := scope.key()
		policy, found := policies[key]
		if !found || !policy.Scope.equal(scope) {
			continue
		}
		if input.retained[key] != nil {
			return admissionInputs{}, admissionError("retained.scope", "cannot repeat retained evidence for one scope")
		}
		input.retained[key] = &observation
	}
	return input, nil
}

func validateAttempt(policy ScopePolicy, attempt Attempt, run Run) error {
	if policy.Enabled == (attempt.Outcome == Disabled) {
		return admissionError("attempt.outcome", "must agree with the scope's configured enabled state")
	}
	switch attempt.Outcome {
	case Succeeded, Partial:
		if attempt.Observation == nil {
			return admissionError("attempt.observation", "requires a direct observation for a successful or partial attempt")
		}
		observation := *attempt.Observation
		if err := observation.Validate(); err != nil {
			return admissionError("attempt.observation", "must contain valid evidence and its original receipt")
		}
		if !attempt.Scope.equal(Scope{Source: observation.SourceID, Binding: observation.ProviderBinding}) {
			return admissionError("attempt.observation", "must match the exact attempted scope")
		}
		if observation.ObservedAt.Before(run.StartedAt) || observation.ObservedAt.After(run.CompletedAt) {
			return admissionError("attempt.observed_at", "must fall within the run interval")
		}
		if (attempt.Outcome == Succeeded) != complete(observation) {
			return admissionError("attempt.outcome", "must agree with observation completeness and status")
		}
	case Failed, MissingCredentials, NotAttempted, Disabled:
		if attempt.Observation != nil {
			return admissionError("attempt.observation", "cannot attach new evidence to an unsuccessful or disabled attempt")
		}
	default:
		return admissionError("attempt.outcome", "must name a supported outcome")
	}
	return nil
}

func utcTime(value time.Time) bool {
	_, offset := value.Zone()
	return !value.IsZero() && offset == 0
}

func admissionError(field, message string) error {
	return &errors.ValidationError{Field: "publication_admission." + field, Message: message}
}
