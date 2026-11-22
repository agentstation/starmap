package runtime

import (
	"context"
	"sync"

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

// AllowsCatalogAttempt checks permission for a consumer's separately accepted catalog.
// The consumer must bind this validated authority head to its exact catalog before publication.
// A retained catalog can serve new attempts only while its permission revision remains current.
// Equal permission revisions allow route preparation without interrupting the previous catalog.
// Ordinary sources use the same admission policy as AllowsNewAttempt.
// Call this memory-only check for every attempt, retry, and cached response delivery.
func (r *Runtime) AllowsCatalogAttempt(head catalogs.CatalogAuthorityHead) bool {
	if r == nil || r.ctx == nil || r.ctx.Err() != nil {
		return false
	}
	if !r.requiresAuthority() {
		return r.AllowsNewAttempt()
	}
	if head.Validate() != nil || !head.SupportsPermissions() {
		return false
	}
	r.mu.RLock()
	p, state := r.permissions, r.effective
	r.mu.RUnlock()
	if state.Catalog == nil || head.AuthorityID != p.authorityID || head.PolicyID != p.policyID ||
		head.Sequence > p.highest.Sequence || head.RequiredPermissionRevision != p.highest.RequiredPermissionRevision {
		return false
	}
	for _, known := range []catalogs.CatalogAuthorityHead{p.highest, p.enforced, p.activeReceipt.Head} {
		if head.Sequence == known.Sequence && head != known {
			return false
		}
	}
	clock := r.readPermissionClock()
	return p.allowsNewAttempt(clock.Time, clock.Uncertainty, clock.Known)
}

// ReadPermission issues an origin receipt or returns a confirmed upstream receipt without renewing it.
// Origins read current authoritative storage with a qualified clock. Relays read retained receipts from memory.
// It forwards a required revision even when this runtime cannot activate its catalog or permission schema.
// A newer manifest without a matching receipt, uncertain retention, or unknown clock validity prevents delivery.
// Callers must authenticate the downstream connection and enforce the receipt's original expiry.
func (r *Runtime) ReadPermission(ctx context.Context) (catalogs.CatalogPermissionEnvelope, error) {
	if r == nil || ctx == nil {
		return catalogs.CatalogPermissionEnvelope{}, &errors.ValidationError{Field: "permission_relay", Message: "requires a runtime and context"}
	}
	if err := ctx.Err(); err != nil {
		return catalogs.CatalogPermissionEnvelope{}, err
	}
	if err := r.ctx.Err(); err != nil {
		return catalogs.CatalogPermissionEnvelope{}, err
	}
	if r.config.origin != nil {
		return r.config.origin.issuer.ReadPermission(ctx)
	}
	if !r.requiresAuthority() {
		return catalogs.CatalogPermissionEnvelope{}, &errors.ConfigError{Component: "permission relay", Message: "requires an internal catalog authority"}
	}
	r.mu.RLock()
	p := r.permissions
	r.mu.RUnlock()
	clock := r.readPermissionClock()
	if !p.retained || p.pending || p.activeReceipt.Head != p.highest || !p.activeReceipt.ValidAt(clock.Time, clock.Uncertainty, clock.Known) {
		return catalogs.CatalogPermissionEnvelope{}, &errors.ConflictError{Resource: "catalog permission", Message: "no confirmed valid receipt matches the highest required publication"}
	}
	return p.activeReceipt, nil
}

// authorityObservationGroup tracks source callbacks until shutdown joins them.
type authorityObservationGroup struct {
	mu     sync.Mutex
	active sync.WaitGroup
	closed bool
}

func (g *authorityObservationGroup) begin() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return false
	}
	g.active.Add(1)
	return true
}

func (g *authorityObservationGroup) close() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
}

type authorityPublicationContextKey struct{}

// The nonzero capability has one identity per runtime. It never reaches source or provider callbacks.
type authorityPublicationCapability struct{ owned bool }

func (owner *authorityPublicationCapability) guard(ctx context.Context) error {
	capability, _ := ctx.Value(authorityPublicationContextKey{}).(*authorityPublicationCapability)
	if capability == nil || capability != owner || !capability.owned {
		return &errors.ConflictError{Resource: "catalog authority publication", Message: "the connected runtime owns publication into this client"}
	}
	return nil
}

func (r *Runtime) authorityPublicationContext(ctx context.Context) context.Context {
	if !r.requiresAuthority() && r.config.origin == nil {
		return ctx
	}
	return context.WithValue(ctx, authorityPublicationContextKey{}, r.config.publicationCapability)
}
