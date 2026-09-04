package providers

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// credentialState is the result of one pre-flight credential check.
type credentialState int

const (
	// credentialReady means the deployment holds usable catalog-acquisition
	// material, or the provider declares a public catalog endpoint.
	credentialReady credentialState = iota

	// credentialAbsent means the provider requires catalog-acquisition
	// material that the deployment does not hold.
	credentialAbsent

	// credentialInvalid means the credential reference itself is unusable.
	credentialInvalid
)

// credentialMemo resolves each provider credential once per observation run.
// The pre-flight check and the fetch share one resolution, so a run reads the
// deployment credential store one time for each provider.
type credentialMemo struct {
	resolver sources.ProviderCredentialResolver

	mu      sync.Mutex
	entries map[catalogs.ProviderID]credentialEntry
}

// credentialEntry holds the remembered result of one resolution.
type credentialEntry struct {
	material sources.ProviderCredentialMaterial
	err      error
}

// newCredentialMemo wraps one deployment resolver. A nil resolver stays nil, so
// the caller still reports the missing configuration.
func newCredentialMemo(resolver sources.ProviderCredentialResolver) *credentialMemo {
	if resolver == nil {
		return nil
	}
	return &credentialMemo{
		resolver: resolver,
		entries:  make(map[catalogs.ProviderID]credentialEntry),
	}
}

// ResolveCatalog returns the material that this run already resolved. It calls
// the deployment resolver one time for each provider.
func (m *credentialMemo) ResolveCatalog(
	ctx context.Context,
	provider *catalogs.Provider,
) (sources.ProviderCredentialMaterial, error) {
	if provider == nil {
		return sources.ProviderCredentialMaterial{}, &pkgerrors.ValidationError{
			Field: "provider", Message: "is required",
		}
	}
	m.mu.Lock()
	entry, found := m.entries[provider.ID]
	m.mu.Unlock()
	if found {
		return entry.material, entry.err
	}

	material, err := m.resolver.ResolveCatalog(ctx, provider)
	m.mu.Lock()
	m.entries[provider.ID] = credentialEntry{material: material, err: err}
	m.mu.Unlock()
	return material, err
}

// forget drops every remembered resolution. Each observation run starts with an
// empty memo, so a rotated credential reaches the next run.
func (m *credentialMemo) forget() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.entries = make(map[catalogs.ProviderID]credentialEntry)
	m.mu.Unlock()
}

// preflight checks catalog-acquisition credentials before any provider
// request. The source skips a provider that holds no required material, so it
// never opens a connection that authentication would refuse.
//
// The check and the fetch share one memo, so the run resolves the material of
// one provider one time.
func (s *Source) preflight(ctx context.Context, provider *catalogs.Provider) (credentialState, error) {
	if s.credentials == nil {
		return credentialInvalid, &pkgerrors.ConfigError{
			Component: string(provider.ID),
			Message:   "provider credential resolver is not configured",
		}
	}
	if _, err := s.credentials.ResolveCatalog(ctx, provider); err != nil {
		var authenticationErr *pkgerrors.AuthenticationError
		if errors.As(err, &authenticationErr) {
			return credentialAbsent, err
		}
		return credentialInvalid, err
	}
	return credentialReady, nil
}

// skippedAttempt records one provider that acquisition never requested.
func skippedAttempt(
	providerID catalogs.ProviderID,
	reason sources.ProviderReason,
	started, completed time.Time,
) sources.ProviderAttempt {
	return sources.ProviderAttempt{
		ProviderID:  providerID,
		Outcome:     sources.ProviderOutcomeSkippedNotConfigured,
		Reason:      reason,
		Requested:   false,
		StartedAt:   started,
		CompletedAt: completed,
	}
}

// failedAttempt records one provider request that produced no usable records.
func failedAttempt(
	providerID catalogs.ProviderID,
	err error,
	started, completed time.Time,
) sources.ProviderAttempt {
	return sources.ProviderAttempt{
		ProviderID:  providerID,
		Outcome:     sources.ProviderOutcomeFailed,
		Reason:      sources.ClassifyProviderReason(err),
		Requested:   true,
		StartedAt:   started,
		CompletedAt: completed,
	}
}

// succeededAttempt records one provider request that returned records.
func succeededAttempt(
	providerID catalogs.ProviderID,
	records int,
	started, completed time.Time,
) sources.ProviderAttempt {
	return sources.ProviderAttempt{
		ProviderID:  providerID,
		Outcome:     sources.ProviderOutcomeSucceeded,
		Requested:   true,
		StartedAt:   started,
		CompletedAt: completed,
		Records:     records,
	}
}

// skipIssue reports one skipped provider. The issue degrades the observation.
func skipIssue(providerID catalogs.ProviderID, reason sources.ProviderReason) sources.ObservationIssue {
	return sources.ObservationIssue{
		Scope:   sources.ObservationIssueScopeProvider,
		Code:    sources.ObservationIssueCodeMissingCredentials,
		Subject: string(providerID),
		Message: "provider skipped without a request: " + reason.String(),
	}
}
