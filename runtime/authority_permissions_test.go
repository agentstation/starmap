package runtime

import (
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func authorityPermissionFixture() catalogs.CatalogPermissionEnvelope {
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	return catalogs.CatalogPermissionEnvelope{Version: catalogs.CatalogPermissionEnvelopeVersion,
		Head: catalogs.CatalogAuthorityHead{AuthorityID: "enterprise", PolicyID: "production", Sequence: 1,
			GenerationID: "approved-one", PayloadChecksum: "sha256:" + strings.Repeat("a", 64),
			RequiredPermissionRevision: "sha256:" + strings.Repeat("b", 64), PermissionSchemaVersion: catalogs.CatalogPermissionSchemaVersion},
		IssuedAt: at, ValidUntil: at.Add(catalogs.MaxCatalogPermissionValidity)}
}

func retainedAuthorityPermissions(t *testing.T) authorityPermissions {
	t.Helper()
	p := authorityPermissions{authorityID: "enterprise", policyID: "production"}
	receipt := authorityPermissionFixture()
	p, err := p.observe(receipt)
	if err != nil {
		t.Fatal(err)
	}
	p, err = p.confirmRetention(receipt)
	if err != nil {
		t.Fatal(err)
	}
	p, err = p.activate(receipt.Head)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAuthorityPermissionsRequireRetentionAndActivation(t *testing.T) {
	p := authorityPermissions{authorityID: "enterprise", policyID: "production"}
	receipt := authorityPermissionFixture()
	if p.allowsNewAttempt(receipt.IssuedAt, 0, true) {
		t.Fatal("cold authority allowed an attempt")
	}
	p, err := p.observe(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if p.allowsNewAttempt(receipt.IssuedAt, 0, true) {
		t.Fatal("unretained receipt allowed an attempt")
	}
	p, err = p.confirmRetention(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if p.allowsNewAttempt(receipt.IssuedAt, 0, true) {
		t.Fatal("receipt without an activated catalog allowed an attempt")
	}
	p, err = p.activate(receipt.Head)
	if err != nil {
		t.Fatal(err)
	}
	if !p.allowsNewAttempt(receipt.IssuedAt, 0, true) {
		t.Fatal("retained and enforced permission refused an attempt")
	}
}

func TestAuthorityPermissionsKnownWithdrawalBlocksFailedActivation(t *testing.T) {
	previous := retainedAuthorityPermissions(t)
	next := previous.required
	next.Head.Sequence++
	next.Head.GenerationID = "withdrawn-two"
	next.Head.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
	next.Head.PermissionSchemaVersion++
	current, err := previous.observe(next)
	if err != nil {
		t.Fatal(err)
	}
	if current.allowsNewAttempt(next.IssuedAt, 0, true) {
		t.Fatal("pending withdrawal retention kept admission open")
	}
	current, err = current.confirmRetention(next)
	if err != nil {
		t.Fatal(err)
	}
	refused, err := current.activate(next.Head)
	if err == nil {
		t.Fatal("activated unknown mandatory permission semantics")
	}
	if refused.required.Head != next.Head || refused.enforced != previous.enforced || refused.allowsNewAttempt(next.IssuedAt, 0, true) {
		t.Fatal("failed activation lost the withdrawal or restored permission")
	}
	if !previous.allowsNewAttempt(next.IssuedAt, 0, true) {
		t.Fatal("immutable snapshot changed for previously admitted work")
	}
}

func TestAuthorityPermissionsRetainCompatibleCatalogAcrossMetadataUpdate(t *testing.T) {
	previous := retainedAuthorityPermissions(t)
	next := previous.required
	next.Head.Sequence++
	next.Head.GenerationID = "metadata-two"
	next.Head.PayloadChecksum = "sha256:" + strings.Repeat("c", 64)
	next.IssuedAt = next.IssuedAt.Add(time.Minute)
	next.ValidUntil = next.ValidUntil.Add(time.Minute)
	current, err := previous.observe(next)
	if err != nil {
		t.Fatal(err)
	}
	current, err = current.confirmRetention(next)
	if err != nil {
		t.Fatal(err)
	}
	if current.enforced != previous.enforced || !current.allowsNewAttempt(next.IssuedAt, 0, true) {
		t.Fatal("unchanged permissions invalidated the compatible prior catalog")
	}
	if _, err := current.confirmRetention(previous.required); err == nil {
		t.Fatal("older write confirmed a newer receipt")
	}
	if _, err := current.observe(previous.required); err == nil {
		t.Fatal("accepted an older publication")
	}
}

func TestAuthorityPermissionsRejectIdentityAndRevisionChanges(t *testing.T) {
	current := retainedAuthorityPermissions(t)
	for name, change := range map[string]func(*catalogs.CatalogPermissionEnvelope){
		"authority": func(e *catalogs.CatalogPermissionEnvelope) { e.Head.AuthorityID = "other" },
		"policy":    func(e *catalogs.CatalogPermissionEnvelope) { e.Head.PolicyID = "other" },
		"same sequence different revision": func(e *catalogs.CatalogPermissionEnvelope) {
			e.Head.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
		},
	} {
		t.Run(name, func(t *testing.T) {
			next := current.required
			change(&next)
			got, err := current.observe(next)
			if err == nil || got.required != current.required || got.enforced != current.enforced || got.allowsNewAttempt(current.required.IssuedAt, 0, true) {
				t.Fatal("invalid receipt changed the active state")
			}
		})
	}
	next := current.required
	next.Head.Sequence++
	next.Head.GenerationID = "permission-two"
	next.Head.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
	pending, err := current.observe(next)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pending.activate(current.enforced); err == nil {
		t.Fatal("older permissions satisfied a new required revision")
	}
}

func TestAuthorityPermissionsClockChecksAllocateNoMemory(t *testing.T) {
	current := retainedAuthorityPermissions(t)
	at := current.required.IssuedAt
	if current.allowsNewAttempt(at, 0, false) {
		t.Fatal("unknown clock allowed an attempt")
	}
	if current.allowsNewAttempt(current.required.ValidUntil.Add(-30*time.Second), 30*time.Second, true) {
		t.Fatal("expired permission allowed an attempt")
	}
	var allowed bool
	allocations := testing.AllocsPerRun(100, func() { allowed = current.allowsNewAttempt(at, 30*time.Second, true) })
	if !allowed || allocations != 0 {
		t.Fatalf("permission snapshot: allowed=%v, allocations=%v", allowed, allocations)
	}
}

func TestAuthorityPermissionsLateRetentionCannotRestoreRejectedReceipt(t *testing.T) {
	current := retainedAuthorityPermissions(t)
	invalid := current.required
	invalid.Head.GenerationID = "changed-content-under-same-sequence"
	refused, err := current.observe(invalid)
	if err == nil {
		t.Fatal("accepted conflicting authority content")
	}
	restored, err := refused.confirmRetention(current.required)
	if err == nil || restored.allowsNewAttempt(current.required.IssuedAt, 0, true) {
		t.Fatal("late retention confirmation restored rejected permission")
	}
}
