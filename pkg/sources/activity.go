package sources

import (
	"errors"
	"slices"
)

// Eligibility states whether acquisition preflight confirms source availability.
type Eligibility string

const (
	// EligibilityUnknown means preflight has not established availability.
	EligibilityUnknown Eligibility = "unknown"
	// EligibilityEligible means the required preflight checks passed.
	EligibilityEligible Eligibility = "eligible"
	// EligibilityIneligible means a required preflight condition failed.
	EligibilityIneligible Eligibility = "ineligible"
)

// SourceActivity reports capability, selection, and execution for one acquisition source.
// Accepted input belongs to the active generation and remains separate from this run.
type SourceActivity struct {
	// Source identifies the registered acquisition source.
	Source ID `json:"source"`
	// Supported reports whether this composition implements the source.
	Supported bool `json:"supported"`
	// Enabled reports configured selection, independent of automatic refresh scheduling.
	Enabled bool `json:"enabled"`
	// Eligibility remains unknown until the required source preflight checks finish.
	Eligibility Eligibility `json:"eligibility"`
	// Attempted reports whether the source collector ran. Provider attempts separately report network requests.
	Attempted bool `json:"attempted"`
}

// Valid reports whether activity uses a known acquisition source and consistent states.
func (a SourceActivity) Valid() bool {
	switch a.Source {
	case LocalCatalogID, ModelsDevHTTPID, ModelsDevGitID, ProvidersID:
	default:
		return false
	}
	switch a.Eligibility {
	case EligibilityUnknown, EligibilityEligible, EligibilityIneligible:
	default:
		return false
	}
	if a.Attempted && (!a.Supported || !a.Enabled) {
		return false
	}
	return a.Eligibility == EligibilityUnknown || (a.Supported && a.Enabled)
}

// ActivityError preserves per-run source activity when acquisition fails before a result exists.
// Call ActivityFromError to get an owned report while retaining the original error identity.
type ActivityError struct {
	// Activities contains the source report for this failed run.
	Activities []SourceActivity
	// ProviderAttempts preserves per-profile outcomes from the failed run.
	ProviderAttempts []ProviderAttempt
	// Err preserves the original acquisition failure.
	Err error
}

// Error preserves the underlying acquisition error text.
func (e *ActivityError) Error() string {
	if e.Err == nil {
		return "source acquisition failed"
	}
	return e.Err.Error()
}

// Unwrap preserves typed acquisition errors and cancellation identity.
func (e *ActivityError) Unwrap() error { return e.Err }

// ActivityFromError returns an owned source report from an acquisition error.
func ActivityFromError(err error) []SourceActivity {
	var report *ActivityError
	if !errors.As(err, &report) {
		return nil
	}
	return slices.Clone(report.Activities)
}

// ProviderAttemptsFromError returns owned provider outcomes from a failed acquisition run.
func ProviderAttemptsFromError(err error) []ProviderAttempt {
	var report *ActivityError
	if !errors.As(err, &report) {
		return nil
	}
	return slices.Clone(report.ProviderAttempts)
}
