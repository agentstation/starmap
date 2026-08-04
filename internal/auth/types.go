// Package auth provides authentication checking for AI model providers.
package auth

import "github.com/agentstation/starmap/pkg/catalogs"

// State represents the authentication state of a provider.
type State int

const (
	// StateConfigured means the provider has credentials configured.
	StateConfigured State = iota
	// StateMissing means required credentials are missing.
	StateMissing
	// StateInvalid means credentials are found but malformed or invalid.
	StateInvalid
	// StateOptional means the provider has optional or no auth requirements.
	StateOptional
	// StateUnsupported means the provider has no client implementation.
	StateUnsupported
)

// Status represents catalog-acquisition authentication status.
type Status struct {
	State           State
	Summary         string                  // Brief one-line summary
	APIKey          *APIKeyDetails          // API key details, when applicable
	CredentialChain *CredentialChainDetails // Cloud credential-chain details, when applicable
}

// CredentialChainDetails describes which catalog-acquisition chain was selected.
// Details can contain provider-specific inspection data but never a credential value.
type CredentialChainDetails struct {
	Method  catalogs.ProviderCatalogAuthMethod
	Source  string
	Details any
}

// APIKeyDetails contains API key authentication details.
type APIKeyDetails struct {
	EnvVar  string // Environment variable name
	IsSet   bool   // Whether the env var is set
	IsValid bool   // Whether the value matches required pattern
	Source  string // Where credentials come from (e.g., "env")
}

// CredentialResolver checks one catalog-acquisition credential method.
type CredentialResolver interface {
	Check(*catalogs.Provider) *Status
}

// CredentialResolverFunc adapts a function to CredentialResolver.
type CredentialResolverFunc func(*catalogs.Provider) *Status

// Check implements CredentialResolver.
func (f CredentialResolverFunc) Check(provider *catalogs.Provider) *Status {
	return f(provider)
}

// CheckerOption configures a Checker.
type CheckerOption func(*Checker)

// WithCredentialResolver registers or replaces a credential resolver.
func WithCredentialResolver(
	method catalogs.ProviderCatalogAuthMethod,
	resolver CredentialResolver,
) CheckerOption {
	return func(checker *Checker) {
		checker.resolvers[method] = resolver
	}
}

// Checker checks authentication status for providers.
type Checker struct {
	resolvers map[catalogs.ProviderCatalogAuthMethod]CredentialResolver
}

// NewChecker creates a new authentication checker.
func NewChecker(options ...CheckerOption) *Checker {
	checker := &Checker{resolvers: map[catalogs.ProviderCatalogAuthMethod]CredentialResolver{
		catalogs.ProviderCatalogAuthNone:          CredentialResolverFunc(checkNoCredentials),
		catalogs.ProviderCatalogAuthAPIKey:        CredentialResolverFunc(checkAPIKey),
		catalogs.ProviderCatalogAuthGoogleDefault: CredentialResolverFunc(checkGoogleDefault),
	}}
	for _, option := range options {
		option(checker)
	}
	return checker
}
