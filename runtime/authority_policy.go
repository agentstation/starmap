package runtime

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func (p SourcePolicy) validateAuthority() error {
	if p.StartupPolicy != StartupRequireAuthority {
		if p.AuthorityID != "" || p.PolicyID != "" {
			return &errors.ValidationError{Field: "source_policy.authority", Message: "requires require_authority startup policy"}
		}
		return nil
	}
	if p.Kind != SourceStarmap {
		return &errors.ValidationError{Field: "source_policy.kind", Message: "require_authority needs a starmap source"}
	}
	if err := catalogs.ValidateCatalogAuthorityIdentity(p.AuthorityID, p.PolicyID); err != nil {
		return &errors.ValidationError{Field: "source_policy.authority", Message: "requires bounded authority and policy identities without surrounding whitespace or control characters"}
	}
	return nil
}

// WithSourceAuthority pins the authority and policy used by require_authority.
// It changes neither the source address nor the startup policy.
func WithSourceAuthority(authorityID, policyID string) Option {
	return func(o *options) error {
		o.source.AuthorityID, o.source.PolicyID = authorityID, policyID
		return nil
	}
}

// WithSourceAuthorityID selects the authority identity required by the source policy.
func WithSourceAuthorityID(identity string) Option {
	return func(o *options) error { o.source.AuthorityID = identity; return nil }
}

// WithSourcePolicyID selects the permission policy within the configured authority.
func WithSourcePolicyID(identity string) Option {
	return func(o *options) error { o.source.PolicyID = identity; return nil }
}

func (r *Runtime) requiresAuthority() bool {
	return r != nil && r.config.source.StartupPolicy == StartupRequireAuthority
}

// validateStoredAuthoritySelection prevents omitted settings from removing stored authority.
// It runs before workspace recovery, input publication, or background work.
func (r options) validateStoredAuthoritySelection(head catalogs.CatalogAuthorityHead) error {
	if head == (catalogs.CatalogAuthorityHead{}) || r.origin != nil || r.source.StartupPolicy == StartupRequireAuthority {
		return nil
	}
	return &errors.ConfigError{
		Component: "catalog authority",
		Message:   "stored catalog requires origin configuration or the require_authority startup policy",
	}
}

// AllowsNewAttempt checks current catalog permission using memory only.
// Call it for every new attempt, including retries and cached response delivery.
// It does not replace model, destination, account, or budget authorization.
func (r *Runtime) AllowsNewAttempt() bool {
	if r == nil || r.ctx.Err() != nil {
		return false
	}
	r.mu.RLock()
	p, state := r.permissions, r.effective
	r.mu.RUnlock()
	if !r.requiresAuthority() {
		return state.Catalog != nil
	}
	clock := r.readPermissionClock()
	return state.Catalog != nil && p.allowsNewAttempt(clock.Time, clock.Uncertainty, clock.Known)
}
