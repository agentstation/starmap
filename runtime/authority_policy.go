package runtime

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/agentstation/starmap/pkg/errors"
)

const maxAuthorityIdentityBytes = 256

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
	for _, identity := range []string{p.AuthorityID, p.PolicyID} {
		if identity == "" || len(identity) > maxAuthorityIdentityBytes || strings.TrimSpace(identity) != identity ||
			!utf8.ValidString(identity) || strings.ContainsFunc(identity, unicode.IsControl) {
			return &errors.ValidationError{Field: "source_policy.authority", Message: "requires bounded authority and policy identities without surrounding whitespace or control characters"}
		}
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

// WithPermissionClockUncertainty supplies the host's cached clock assurance.
// The callback reads cached memory only. It reports uncertainty for WithClock's time
// and false when its evidence is absent or expired. No callback means unknown.
func WithPermissionClockUncertainty(sample func() (time.Duration, bool)) Option {
	return func(o *options) error {
		if sample == nil {
			return &errors.ValidationError{Field: "permission_clock", Message: "is required"}
		}
		o.permissionClockUncertainty = sample
		return nil
	}
}

func (r *Runtime) requiresAuthority() bool {
	return r != nil && r.config.source.StartupPolicy == StartupRequireAuthority
}

func (r *Runtime) permissionClock() (time.Duration, bool) {
	if r.config.permissionClockUncertainty == nil {
		return 0, false
	}
	return r.config.permissionClockUncertainty()
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
	uncertainty, known := r.permissionClock()
	return state.Catalog != nil && p.allowsNewAttempt(r.config.now(), uncertainty, known)
}
