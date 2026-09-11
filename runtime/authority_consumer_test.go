package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"

	"github.com/agentstation/starmap/pkg/catalogs/permission"
)

func TestAuthorityConsumerSnapshotRequiresCurrentPermissionRevision(t *testing.T) {
	previous := retainedAuthorityPermissions(t)
	retained := previous.enforced
	next := previous.required
	next.Head.Sequence++
	next.Head.GenerationID = "replacement-catalog"
	next.Head.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
	updated, err := previous.observe(next)
	if err != nil {
		t.Fatal(err)
	}
	updated, err = updated.confirmRetention(next)
	if err != nil {
		t.Fatal(err)
	}
	updated, err = updated.activate(next.Head)
	if err != nil {
		t.Fatal(err)
	}
	r := authorityClockRuntime(t, updated, func() permission.ClockReading {
		return permission.ClockReading{Time: next.IssuedAt, Known: true}
	})
	if retained.RequiredPermissionRevision == updated.enforced.RequiredPermissionRevision {
		t.Fatal("fixture did not retain the previous routing policy")
	}
	if !r.AllowsNewAttempt() {
		t.Fatal("replacement catalog is not ready")
	}
	if r.AllowsCatalogAttempt(retained) {
		t.Fatal("runtime readiness authorized the consumer's withdrawn catalog")
	}
}

func TestAuthorityConsumerSnapshotAllowsUnchangedRevision(t *testing.T) {
	previous := retainedAuthorityPermissions(t)
	retained := previous.enforced
	next := previous.required
	next.Head.Sequence++
	next.Head.GenerationID = "same-permission-replacement"
	updated, err := previous.observe(next)
	if err != nil {
		t.Fatal(err)
	}
	updated, err = updated.confirmRetention(next)
	if err != nil {
		t.Fatal(err)
	}
	updated, err = updated.activate(next.Head)
	if err != nil {
		t.Fatal(err)
	}
	r := authorityClockRuntime(t, updated, func() permission.ClockReading {
		return permission.ClockReading{Time: next.IssuedAt, Known: true}
	})
	for _, head := range []catalogs.CatalogAuthorityHead{retained, next.Head} {
		if !r.AllowsCatalogAttempt(head) {
			t.Fatal("unchanged permission revision interrupted a validated catalog")
		}
	}
	if allocations := testing.AllocsPerRun(100, func() { r.AllowsCatalogAttempt(retained) }); allocations != 0 {
		t.Fatalf("warm consumer permission check allocated %v times", allocations)
	}
}

func TestAuthorityConsumerSnapshotRejectsInvalidHead(t *testing.T) {
	p := retainedAuthorityPermissions(t)
	r := authorityClockRuntime(t, p, func() permission.ClockReading {
		return permission.ClockReading{Time: p.activeReceipt.IssuedAt, Known: true}
	})
	cases := map[string]func(*catalogs.CatalogAuthorityHead){
		"empty":             func(h *catalogs.CatalogAuthorityHead) { *h = catalogs.CatalogAuthorityHead{} },
		"foreign authority": func(h *catalogs.CatalogAuthorityHead) { h.AuthorityID = "foreign" },
		"foreign policy":    func(h *catalogs.CatalogAuthorityHead) { h.PolicyID = "foreign" },
		"zero sequence":     func(h *catalogs.CatalogAuthorityHead) { h.Sequence = 0 },
		"future sequence":   func(h *catalogs.CatalogAuthorityHead) { h.Sequence++ },
		"different revision": func(h *catalogs.CatalogAuthorityHead) {
			h.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
		},
		"unsupported permissions":  func(h *catalogs.CatalogAuthorityHead) { h.PermissionSchemaVersion++ },
		"contradictory generation": func(h *catalogs.CatalogAuthorityHead) { h.GenerationID = "another-generation" },
		"contradictory checksum":   func(h *catalogs.CatalogAuthorityHead) { h.PayloadChecksum = "sha256:" + strings.Repeat("c", 64) },
		"missing generation":       func(h *catalogs.CatalogAuthorityHead) { h.GenerationID = "" },
		"missing checksum":         func(h *catalogs.CatalogAuthorityHead) { h.PayloadChecksum = "" },
		"invalid checksum":         func(h *catalogs.CatalogAuthorityHead) { h.PayloadChecksum = "sha256:wrong" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			head := p.enforced
			change(&head)
			if r.AllowsCatalogAttempt(head) {
				t.Fatal("invalid consumer authority head authorized an attempt")
			}
		})
	}
}

func TestAuthorityConsumerSnapshotRequiresValidRuntimePermission(t *testing.T) {
	p := retainedAuthorityPermissions(t)
	cases := []struct {
		name    string
		reading permission.ClockReading
	}{
		{name: "unknown", reading: permission.ClockReading{Time: p.activeReceipt.IssuedAt}},
		{name: "expired", reading: permission.ClockReading{Time: p.activeReceipt.ValidUntil, Known: true}},
		{name: "excessive error", reading: permission.ClockReading{Time: p.activeReceipt.IssuedAt, Known: true, Uncertainty: time.Minute}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := authorityClockRuntime(t, p, func() permission.ClockReading { return tc.reading })
			if r.AllowsCatalogAttempt(p.enforced) {
				t.Fatal("invalid permission clock authorized a retained catalog")
			}
		})
	}
	t.Run("unretained refusal", func(t *testing.T) {
		refused := p
		refused.retained, refused.pending = false, false
		r := authorityClockRuntime(t, refused, func() permission.ClockReading {
			return permission.ClockReading{Time: p.activeReceipt.IssuedAt, Known: true}
		})
		if r.AllowsCatalogAttempt(p.enforced) {
			t.Fatal("unretained permission authorized a catalog")
		}
	})
	t.Run("closed", func(t *testing.T) {
		r := authorityClockRuntime(t, p, func() permission.ClockReading {
			return permission.ClockReading{Time: p.activeReceipt.IssuedAt, Known: true}
		})
		ctx, cancel := context.WithCancel(t.Context())
		r.ctx = ctx
		cancel()
		if r.AllowsCatalogAttempt(p.enforced) {
			t.Fatal("closed runtime authorized a catalog")
		}
	})
}

func TestOrdinaryConsumerSnapshotUsesOrdinaryAdmission(t *testing.T) {
	r := &Runtime{ctx: t.Context(), config: *defaults(), effective: starmap.CatalogState{Catalog: &catalogs.Catalog{}}}
	if !r.AllowsCatalogAttempt(catalogs.CatalogAuthorityHead{}) {
		t.Fatal("ordinary source required an authority head")
	}
	r.effective = starmap.CatalogState{}
	if r.AllowsCatalogAttempt(catalogs.CatalogAuthorityHead{}) {
		t.Fatal("missing ordinary catalog authorized an attempt")
	}
	for _, absent := range []*Runtime{nil, {}} {
		if absent.AllowsCatalogAttempt(catalogs.CatalogAuthorityHead{}) {
			t.Fatal("absent runtime authorized an attempt")
		}
	}
}
