// Package auth resolves and reports catalog-acquisition credentials.
package auth

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// State represents the authentication state of a provider.
type State int

const (
	// StateConfigured means the provider has credentials configured.
	StateConfigured State = iota
	// StateMissing means the provider requires credentials but has none.
	StateMissing
	// StateInvalid means the configured credentials failed validation.
	StateInvalid
	// StateOptional means the provider has optional or no auth requirements.
	StateOptional
	// StateUnsupported means the provider has no client implementation.
	StateUnsupported
)

// Status represents catalog-acquisition authentication status.
type Status struct {
	State   State
	Summary string
	Profile *ProfileDetails
}

// ProfileDetails identifies the selected secret-free authentication profile.
type ProfileDetails struct {
	ID        catalogs.ProviderCredentialProfileID
	Primitive catalogs.ProviderAuthenticationPrimitive
}

// CheckerOption configures a Checker.
type CheckerOption func(*Checker)

// WithCredentialResolver replaces catalog credential resolution.
func WithCredentialResolver(resolver sources.ProviderCredentialResolver) CheckerOption {
	return func(checker *Checker) {
		checker.resolver = resolver
	}
}

// Checker checks authentication status for providers.
type Checker struct {
	resolver sources.ProviderCredentialResolver
	ctx      context.Context
}

// NewChecker creates a new authentication checker.
func NewChecker(options ...CheckerOption) *Checker {
	checker := &Checker{resolver: NewResolver(), ctx: context.Background()}
	for _, option := range options {
		option(checker)
	}
	return checker
}

func (c *Checker) context() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}
