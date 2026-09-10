package permission

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestPrepareGenerationRejectsInvalidSourceAndIdentity(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*catalogs.Generation, *GenerationConfig)
	}{
		{"empty authority", func(_ *catalogs.Generation, c *GenerationConfig) { c.AuthorityID = "" }},
		{"empty policy", func(_ *catalogs.Generation, c *GenerationConfig) { c.PolicyID = "" }},
		{"oversize identity", func(_ *catalogs.Generation, c *GenerationConfig) { c.AuthorityID = strings.Repeat("x", 257) }},
		{"invalid UTF-8", func(_ *catalogs.Generation, c *GenerationConfig) { c.PolicyID = "\xff" }},
		{"zero sequence", func(_ *catalogs.Generation, c *GenerationConfig) { c.Sequence = 0 }},
		{"empty source", func(g *catalogs.Generation, _ *GenerationConfig) { *g = catalogs.Generation{} }},
		{"authority relabel", func(g *catalogs.Generation, _ *GenerationConfig) { *g = issuerGeneration(t, "upstream-authority", 10) }},
		{"unbound authority", func(g *catalogs.Generation, _ *GenerationConfig) {
			g.Manifest.AuthorityHead = issuerGeneration(t, "another", 1).Manifest.AuthorityHead
		}},
		{"changed payload", func(g *catalogs.Generation, _ *GenerationConfig) { g.Payload = append(g.Payload, ' ') }},
		{"different schema", func(g *catalogs.Generation, _ *GenerationConfig) { g.Manifest.SchemaVersion-- }},
		{"unknown schema", func(g *catalogs.Generation, _ *GenerationConfig) { g.Manifest.SchemaVersion++ }},
		{"failed validation", func(g *catalogs.Generation, _ *GenerationConfig) {
			g.Manifest.Validation.Status = catalogs.GenerationValidationFailed
		}},
		{"invalid catalog bytes", func(g *catalogs.Generation, _ *GenerationConfig) {
			g.Payload = []byte(`{"schema_version":9}`)
			g.Manifest.Payload = catalogs.DescribeCatalogPayload(g.Payload)
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			input := originInput(t, true)
			config := GenerationConfig{AuthorityID: "enterprise", PolicyID: "production", Sequence: 1}
			scenario.change(&input, &config)
			output, err := PrepareGeneration(input, config)
			if err == nil || !errors.IsValidationError(err) || !reflect.DeepEqual(output, catalogs.Generation{}) {
				t.Fatalf("invalid origin preparation returned %s / %v", output.Manifest.GenerationID, err)
			}
		})
	}
}

func originScope(input catalogs.Generation) catalogs.ProviderMembershipScope {
	observation := input.Manifest.SourceObservations[0]
	return catalogs.ProviderMembershipScope{
		PublisherID: "enterprise", BindingID: "account", BindingRevision: "v1", ProviderID: "provider",
		AccountID: "account-one", Region: "global", APISurface: "inference", Authority: catalogs.MembershipScopeAuthority,
		Inventory: &catalogs.MembershipInventory{ObservationID: observation.ObservationID, ObservedAt: observation.ObservedAt, ModelIDs: []string{"served-model"}},
	}
}

func TestPrepareGenerationBindsAccountMembership(t *testing.T) {
	input := originInput(t, true)
	scope := originScope(input)
	input = changeOriginCatalog(t, input, func(b *catalogs.Builder) error {
		return b.SetMembershipScopes([]catalogs.ProviderMembershipScope{scope})
	})
	config := GenerationConfig{AuthorityID: "enterprise", PolicyID: "production", Sequence: 1}
	first := prepareOrigin(t, input, config)
	for _, scenario := range []struct {
		name   string
		change func(*catalogs.ProviderMembershipScope)
	}{
		{"known absence", func(s *catalogs.ProviderMembershipScope) { s.Inventory.ModelIDs = []string{} }},
		{"unknown inventory", func(s *catalogs.ProviderMembershipScope) { s.Inventory = nil }},
		{"different account", func(s *catalogs.ProviderMembershipScope) { s.AccountID = "account-two" }},
		{"different region", func(s *catalogs.ProviderMembershipScope) { s.Region = "another-region" }},
		{"different binding", func(s *catalogs.ProviderMembershipScope) { s.BindingRevision = "v2" }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			nextScope := originScope(input)
			scenario.change(&nextScope)
			candidate := changeOriginCatalog(t, input, func(b *catalogs.Builder) error {
				return b.SetMembershipScopes([]catalogs.ProviderMembershipScope{nextScope})
			})
			next := prepareOrigin(t, candidate, config)
			if next.Manifest.AuthorityHead.RequiredPermissionRevision == first.Manifest.AuthorityHead.RequiredPermissionRevision {
				t.Fatal("scope change reused a prior account permission revision")
			}
		})
	}
	input.Manifest.SourceObservations[0].ObservationID = "unrelated-observation"
	if _, err := PrepareGeneration(input, config); err == nil {
		t.Fatal("origin adopted membership without its accepted source evidence")
	}
}
