package catalogs

import (
	"testing"
	"time"
)

func testMembershipScope() ProviderMembershipScope {
	return ProviderMembershipScope{PublisherID: "enterprise-catalog", BindingID: "account-a", BindingRevision: "1", ProviderID: "provider", AccountID: "account-a", Region: "region", APISurface: "models.list", Authority: MembershipScopeAuthority}
}

func TestMembershipScopeDistinguishesUnknownAndEmptyInventory(t *testing.T) {
	scope := testMembershipScope()
	at := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	if err := scope.Validate(); err != nil {
		t.Fatal(err)
	}
	if present, known := scope.Membership("model"); present || known {
		t.Fatal("missing inventory granted membership")
	}
	scope.Inventory = &MembershipInventory{ObservationID: "complete", ObservedAt: at, ModelIDs: []string{}}
	if err := scope.Validate(); err != nil {
		t.Fatal(err)
	}
	if present, known := scope.Membership("model"); present || !known {
		t.Fatal("complete empty inventory lost known absence")
	}
	scope.Additions = []MembershipPresence{{ModelID: "model", ObservationID: "partial", ObservedAt: at.Add(time.Minute)}}
	if err := scope.Validate(); err != nil {
		t.Fatal(err)
	}
	if present, known := scope.Membership("model"); !present || !known {
		t.Fatal("new partial positive evidence was discarded")
	}
	if present, known := scope.Membership("other"); present || !known {
		t.Fatal("partial evidence changed unrelated complete absence")
	}
}

func TestMembershipScopeRejectsAmbiguousAuthorityAndEvidence(t *testing.T) {
	at := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	for name, mutate := range map[string]func(*ProviderMembershipScope){
		"publisher":                   func(s *ProviderMembershipScope) { s.PublisherID = "" },
		"binding revision":            func(s *ProviderMembershipScope) { s.BindingRevision = "" },
		"unscoped account":            func(s *ProviderMembershipScope) { s.AccountID = "" },
		"public account":              func(s *ProviderMembershipScope) { s.Public = true },
		"account provider authority":  func(s *ProviderMembershipScope) { s.Authority = MembershipProviderAuthority },
		"unknown authority":           func(s *ProviderMembershipScope) { s.Authority = "unknown" },
		"inventory without authority": func(s *ProviderMembershipScope) { s.Authority = MembershipEvidenceOnly },
		"inventory without receipt":   func(s *ProviderMembershipScope) { s.Inventory.ObservationID = "" },
		"inventory without time":      func(s *ProviderMembershipScope) { s.Inventory.ObservedAt = time.Time{} },
		"null inventory":              func(s *ProviderMembershipScope) { s.Inventory.ModelIDs = nil },
		"duplicate models":            func(s *ProviderMembershipScope) { s.Inventory.ModelIDs = []string{"model", "model"} },
		"old positive": func(s *ProviderMembershipScope) {
			s.Additions = []MembershipPresence{{ModelID: "model", ObservationID: "partial", ObservedAt: at}}
		},
		"control character": func(s *ProviderMembershipScope) { s.Region = "region\n" },
	} {
		t.Run(name, func(t *testing.T) {
			scope := testMembershipScope()
			scope.Inventory = &MembershipInventory{ObservationID: "complete", ObservedAt: at, ModelIDs: []string{}}
			mutate(&scope)
			if err := scope.Validate(); err == nil {
				t.Fatal("invalid membership scope accepted")
			}
		})
	}
}
