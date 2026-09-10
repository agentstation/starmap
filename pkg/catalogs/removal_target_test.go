package catalogs

import "testing"

func TestScopedRemovalTargetPreservesAccountIdentity(t *testing.T) {
	scope := testMembershipScope()
	target, err := NewScopedRemovalTarget(scope, "model/opaque")
	if err != nil {
		t.Fatal(err)
	}
	rotated := scope
	rotated.BindingID, rotated.BindingRevision = "replacement-credential-binding", "2"
	if !target.MatchesScope(rotated, "model/opaque") {
		t.Fatal("credential rotation cleared the account target")
	}
	for name, mutate := range map[string]func(*ProviderMembershipScope){
		"publisher":    func(s *ProviderMembershipScope) { s.PublisherID = "other" },
		"provider":     func(s *ProviderMembershipScope) { s.ProviderID = "other" },
		"account":      func(s *ProviderMembershipScope) { s.AccountID = "other" },
		"project":      func(s *ProviderMembershipScope) { s.ProjectID = "other" },
		"region":       func(s *ProviderMembershipScope) { s.Region = "other" },
		"API surface":  func(s *ProviderMembershipScope) { s.APISurface = "other" },
		"public scope": func(s *ProviderMembershipScope) { s.Public = true; s.AccountID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			peer := scope
			mutate(&peer)
			if target.MatchesScope(peer, "model/opaque") {
				t.Fatal("removal crossed an account scope boundary")
			}
		})
	}
	if target.MatchesScope(scope, "model/OPAQUE") {
		t.Fatal("removal normalized an opaque model ID")
	}
	if target.DefinitionID != "" || target.Kind != CatalogRemovalScoped {
		t.Fatal("scoped removal became canonical removal")
	}
}

func TestRemovalTargetRequiresExplicitCanonicalAction(t *testing.T) {
	scoped, err := NewScopedRemovalTarget(testMembershipScope(), "model")
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := NewCanonicalRemovalTarget("author/model")
	if err != nil {
		t.Fatal(err)
	}
	if canonical.Kind != CatalogRemovalCanonical || canonical.DefinitionID != "author/model" || canonical.Scope != nil || canonical.ProviderModelID != "" {
		t.Fatal("canonical target contains an implicit account selector")
	}
	if canonical.MatchesScope(testMembershipScope(), "model") {
		t.Fatal("canonical action passed as an account action")
	}
	for name, target := range map[string]CatalogRemovalTarget{
		"zero":                       {},
		"unknown kind":               {Kind: "all"},
		"canonical with scope":       {Kind: CatalogRemovalCanonical, DefinitionID: "author/model", Scope: scoped.Scope},
		"canonical with provider ID": {Kind: CatalogRemovalCanonical, DefinitionID: "author/model", ProviderModelID: "model"},
		"scoped with canonical ID":   {Kind: CatalogRemovalScoped, DefinitionID: "author/model", Scope: scoped.Scope, ProviderModelID: "model"},
		"scoped without scope":       {Kind: CatalogRemovalScoped, ProviderModelID: "model"},
		"scoped without model":       {Kind: CatalogRemovalScoped, Scope: scoped.Scope},
		"invalid definition":         {Kind: CatalogRemovalCanonical, DefinitionID: "invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			if target.Validate() == nil {
				t.Fatal("ambiguous removal target accepted")
			}
		})
	}
}

func TestScopedRemovalRejectsInvalidSourceScope(t *testing.T) {
	scope := testMembershipScope()
	scope.AccountID = ""
	if _, err := NewScopedRemovalTarget(scope, "model"); err == nil {
		t.Fatal("invalid source scope accepted")
	}
	if _, err := NewScopedRemovalTarget(testMembershipScope(), "model\n"); err == nil {
		t.Fatal("control character accepted in removal target")
	}
	scope = testMembershipScope()
	scope.Public = true
	scope.AccountID = ""
	target, err := NewScopedRemovalTarget(scope, "model")
	if err != nil {
		t.Fatal(err)
	}
	if !target.MatchesScope(scope, "model") {
		t.Fatal("public scope target lost exact match")
	}
}

func TestScopedRemovalMatchingAllocatesNoMemory(t *testing.T) {
	scope := testMembershipScope()
	target, err := NewScopedRemovalTarget(scope, "model")
	if err != nil {
		t.Fatal(err)
	}
	matched := false
	if got := testing.AllocsPerRun(100, func() { matched = target.MatchesScope(scope, "model") }); got != 0 {
		t.Fatalf("matching allocates %v, want zero", got)
	}
	if !matched {
		t.Fatal("fixture did not match")
	}
}
