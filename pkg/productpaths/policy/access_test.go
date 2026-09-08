package policy

import "testing"

func TestRoleAccessBoundary(t *testing.T) {
	for _, test := range []struct{ role, access string }{
		{"configuration", "owner-only"}, {"dotenv", "owner-only"},
		{"catalog-store", "owner-only"}, {"runtime-owner", "owner-only"},
		{"runtime-evidence", "owner-only"}, {"github-discovery", "owner-only"},
		{"workspace", "deployment-controlled"}, {"workspace-preparing", "owner-only"},
		{"workspace-backup", "deployment-controlled"}, {"baseline", "public-read"},
		{"credentials", "external-system"},
	} {
		t.Run(test.role, func(t *testing.T) {
			got, err := ForRole(test.role)
			if err != nil || got != test.access {
				t.Fatalf("access = %q, %v, want %q", got, err, test.access)
			}
			if err := Require(test.role, test.access); err != nil {
				t.Fatal(err)
			}
			if err := Require(test.role, "unsupported"); err == nil {
				t.Fatal("role accepted an incompatible access adapter")
			}
		})
	}
	for _, role := range []string{"", "unknown", "Catalog-Store"} {
		if access, err := ForRole(role); err == nil || access != "" {
			t.Fatalf("unknown role received access %q: %v", access, err)
		}
		if err := Require(role, OwnerOnly); err == nil {
			t.Fatal("unknown role accepted a private adapter")
		}
	}
}

func TestPrivatePOSIXReason(t *testing.T) {
	for _, test := range []struct {
		name, want string
		input      POSIXMetadata
	}{
		{"private", "", POSIXMetadata{Mode: 0o600, OwnerKnown: true, OwnerMatches: true}},
		{"read-only", "", POSIXMetadata{Mode: 0o400, OwnerKnown: true, OwnerMatches: true}},
		{"no owner access", "", POSIXMetadata{Mode: 0, OwnerKnown: true, OwnerMatches: true}},
		{"group read", GroupOrOtherModeBits, POSIXMetadata{Mode: 0o640, OwnerKnown: true, OwnerMatches: true}},
		{"other execute", GroupOrOtherModeBits, POSIXMetadata{Mode: 0o701, OwnerKnown: true, OwnerMatches: true}},
		{"wrong owner", DifferentEffectiveOwner, POSIXMetadata{Mode: 0o600, OwnerKnown: true}},
		{"unknown owner", OwnerUnavailable, POSIXMetadata{Mode: 0o600}},
		{"known mode conflict", GroupOrOtherModeBits, POSIXMetadata{Mode: 0o640}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := PrivatePOSIXReason(test.input); got != test.want {
				t.Fatalf("reason = %q, want %q", got, test.want)
			}
		})
	}
}
