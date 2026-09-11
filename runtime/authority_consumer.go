package runtime

import "github.com/agentstation/starmap/pkg/catalogs"

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
