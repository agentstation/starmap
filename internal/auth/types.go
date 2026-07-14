// Package auth provides authentication checking for AI model providers.
package auth

import "github.com/agentstation/starmap/pkg/catalogs"

// State represents the authentication state of a provider.
type State int

const (
	// StateReady means the provider has credentials configured.
	StateReady State = iota
	// StateUnavailable means required credentials are missing.
	StateUnavailable
	// StateInvalid means credentials are found but malformed or invalid.
	StateInvalid
	// StateUnauthenticated means the provider has optional or no auth requirements.
	StateUnauthenticated
	// StateUnsupported means the provider has no client implementation.
	StateUnsupported
)

// Status represents aggregate local authentication readiness.
type Status struct {
	State   State
	Summary string
	Sources []SourceStatus
}

// SourceStatus is the secret-safe local resolution status of one logical source.
type SourceStatus struct {
	SourceID        string
	State           State
	Summary         string
	AcceptedMethods []catalogs.ProviderCredentialID
	Environment     []string
}

// Checker checks authentication status for providers.
type Checker struct{ resolver *Resolver }

// CheckerOption configures source-aware status resolution.
type CheckerOption func(*Checker)

// WithCheckerResolver supplies an isolated resolver for status checks.
func WithCheckerResolver(resolver *Resolver) CheckerOption {
	return func(checker *Checker) { checker.resolver = resolver }
}

// WithCheckerCloudChainRegistry enables official-SDK cloud-chain status resolution.
func WithCheckerCloudChainRegistry(registry *CloudChainRegistry) CheckerOption {
	return func(checker *Checker) {
		checker.resolver = NewResolver(WithCloudChainRegistry(registry))
	}
}

// NewChecker creates a new authentication checker.
func NewChecker(options ...CheckerOption) *Checker {
	checker := &Checker{resolver: NewResolver()}
	for _, option := range options {
		option(checker)
	}
	return checker
}
