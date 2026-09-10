package runtime

import (
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// authorityPermissions separates the highest required revision from the revision the runtime can enforce.
// Callers serialize updates and publish copies with the immutable catalog state.
type authorityPermissions struct {
	authorityID string
	policyID    string
	required    catalogs.CatalogPermissionEnvelope
	highest     catalogs.CatalogAuthorityHead
	enforced    catalogs.CatalogAuthorityHead
	retained    bool
	pending     bool
}

// observe records an authenticated receipt before catalog decoding or activation.
// The caller must retain the result before confirming it. Invalid authenticated input preserves known heads and refuses new attempts.
func (p authorityPermissions) observe(next catalogs.CatalogPermissionEnvelope) (authorityPermissions, error) {
	p.retained = false
	p.pending = false
	if err := next.Validate(); err != nil {
		return p, err
	}
	if next.Head.AuthorityID != p.authorityID || next.Head.PolicyID != p.policyID {
		return p, &errors.ValidationError{Field: "catalog_authority.identity", Message: "does not match the configured authority and policy"}
	}
	if p.highest.Sequence != 0 {
		if err := p.highest.ValidateSuccessor(next.Head); err != nil {
			return p, err
		}
	}
	if p.required.Version != 0 {
		if err := p.required.ValidateSuccessor(next); err != nil {
			return p, err
		}
	}
	p.required = next
	p.highest = next.Head
	p.pending = true
	return p, nil
}

// confirmRetention marks only the current receipt as durable or explicitly ephemeral.
// A late write cannot confirm a newer receipt that the runtime did not retain.
func (p authorityPermissions) confirmRetention(receipt catalogs.CatalogPermissionEnvelope) (authorityPermissions, error) {
	return p.confirmCheckpoint(receipt, receipt.Head)
}

func (p authorityPermissions) confirmCheckpoint(receipt catalogs.CatalogPermissionEnvelope, head catalogs.CatalogAuthorityHead) (authorityPermissions, error) {
	if !p.pending || head != p.highest || receipt.Version != p.required.Version || receipt.Head != p.required.Head ||
		!receipt.IssuedAt.Equal(p.required.IssuedAt) || !receipt.ValidUntil.Equal(p.required.ValidUntil) {
		return p, &errors.ConflictError{Resource: "catalog permission retention", Message: "receipt differs from the highest required permission state"}
	}
	p.retained = true
	p.pending = false
	return p, nil
}

// observeHead records a trusted manifest requirement without inventing a permission receipt.
func (p authorityPermissions) observeHead(head catalogs.CatalogAuthorityHead) (authorityPermissions, error) {
	p.retained, p.pending = false, false
	if err := head.Validate(); err != nil {
		return p, err
	}
	if head.AuthorityID != p.authorityID || head.PolicyID != p.policyID {
		return p, &errors.ValidationError{Field: "catalog_authority.identity", Message: "does not match the configured authority and policy"}
	}
	if p.highest.Sequence != 0 {
		if err := p.highest.ValidateSuccessor(head); err != nil {
			return p, err
		}
	}
	p.highest, p.pending = head, true
	return p, nil
}

// activate records the permission revision of a validated and activated catalog.
// The caller proves the generation and payload binding before this transition.
func (p authorityPermissions) activate(head catalogs.CatalogAuthorityHead) (authorityPermissions, error) {
	if err := head.Validate(); err != nil {
		return p, err
	}
	if !head.SupportsPermissions() {
		return p, &errors.ValidationError{Field: "catalog_authority.permission_schema_version", Message: "cannot enforce these permission semantics"}
	}
	if err := head.ValidateSuccessor(p.highest); err != nil {
		return p, err
	}
	if p.enforced.Sequence != 0 {
		if err := p.enforced.ValidateSuccessor(head); err != nil {
			return p, err
		}
	}
	if head.RequiredPermissionRevision != p.highest.RequiredPermissionRevision {
		return p, &errors.ConflictError{Resource: "catalog permission revision", Message: "catalog cannot enforce the highest required revision"}
	}
	p.enforced = head
	return p, nil
}

// allowsNewAttempt reads only the validated snapshot and the caller's qualified clock sample.
// Existing admitted work can retain its prior snapshot. Every new attempt must read the current snapshot.
func (p authorityPermissions) allowsNewAttempt(now time.Time, uncertainty time.Duration, clockKnown bool) bool {
	return p.retained && !p.pending && p.enforced.Sequence != 0 && p.highest.SupportsPermissions() && p.required.Head.SupportsPermissions() &&
		p.enforced.RequiredPermissionRevision == p.highest.RequiredPermissionRevision &&
		p.required.Head.RequiredPermissionRevision == p.highest.RequiredPermissionRevision &&
		p.required.ValidAt(now, uncertainty, clockKnown)
}
