package reconciler

import (
	"testing"

	"github.com/agentstation/starmap/internal/catalog/authority"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestAuthorityStrategySelectsEveryPolicyRankDeterministically(t *testing.T) {
	table := authority.New()
	strategy := NewAuthorityStrategy(table)
	for _, resource := range []evidence.ResourceType{
		evidence.ResourceTypeModel,
		evidence.ResourceTypeProvider,
		evidence.ResourceTypeAuthor,
	} {
		for _, policy := range table.Policies(resource) {
			t.Run(resource.String()+"/"+policy.Path, func(t *testing.T) {
				for firstPresent := range policy.SourceOrder {
					values := make(map[sources.ID]any, len(policy.SourceOrder)-firstPresent)
					for index := len(policy.SourceOrder) - 1; index >= firstPresent; index-- {
						source := policy.SourceOrder[index]
						values[source] = source.String()
					}
					value, source, _ := strategy.ResolveResourceConflict(resource, policy.Path, values)
					want := policy.SourceOrder[firstPresent]
					if source != want || value != want.String() {
						t.Fatalf(
							"first present rank %d selected (%q, %v), want (%q, %q)",
							firstPresent,
							source,
							value,
							want,
							want,
						)
					}
				}
			})
		}
	}
}

func TestAuthorityStrategyAppliesEveryPolicyEmptyContract(t *testing.T) {
	table := authority.New()
	strategy := NewAuthorityStrategy(table)
	for _, resource := range []evidence.ResourceType{
		evidence.ResourceTypeModel,
		evidence.ResourceTypeProvider,
		evidence.ResourceTypeAuthor,
	} {
		for _, policy := range table.Policies(resource) {
			t.Run(resource.String()+"/"+policy.Path, func(t *testing.T) {
				if len(policy.SourceOrder) < 2 {
					t.Fatalf("source order has %d entries; need at least two to test empty fallback", len(policy.SourceOrder))
				}
				first := policy.SourceOrder[0]
				second := policy.SourceOrder[1]
				value, source, _ := strategy.ResolveResourceConflict(resource, policy.Path, map[sources.ID]any{
					second: "fallback",
					first:  "",
				})
				if policy.Empty == authority.EmptyAuthoritative {
					if source != first || value != "" {
						t.Fatalf("authoritative empty selected (%q, %v), want (%q, empty)", source, value, first)
					}
					return
				}
				if source != second || value != "fallback" {
					t.Fatalf("absent empty selected (%q, %v), want (%q, fallback)", source, value, second)
				}
			})
		}
	}
}
