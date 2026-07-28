package authority

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestPoliciesCoverReconciledCatalogFields(t *testing.T) {
	tests := []struct {
		resource sources.ResourceType
		typ      reflect.Type
		ignored  map[string]bool
	}{
		{
			resource: sources.ResourceTypeModel,
			typ:      reflect.TypeFor[catalogs.Model](),
			ignored:  map[string]bool{"ID": true, "CreatedAt": true, "UpdatedAt": true},
		},
		{
			resource: sources.ResourceTypeProvider,
			typ:      reflect.TypeFor[catalogs.Provider](),
			ignored:  map[string]bool{"ID": true, "EnvVarValues": true},
		},
		{
			resource: sources.ResourceTypeAuthor,
			typ:      reflect.TypeFor[catalogs.Author](),
			ignored:  map[string]bool{"ID": true, "CreatedAt": true, "UpdatedAt": true},
		},
	}
	table := New()
	for _, test := range tests {
		t.Run(test.resource.String(), func(t *testing.T) {
			for index := range test.typ.NumField() {
				field := test.typ.Field(index)
				if field.PkgPath != "" || test.ignored[field.Name] {
					continue
				}
				if !hasPolicyForField(table.Policies(test.resource), field.Name) {
					t.Errorf("%s field %s has no executable authority policy", test.resource, field.Name)
				}
			}
		})
	}
}

func TestPoliciesAreCompleteUniqueAndCallerOwned(t *testing.T) {
	table := New()
	for _, resource := range []sources.ResourceType{
		sources.ResourceTypeModel,
		sources.ResourceTypeProvider,
		sources.ResourceTypeAuthor,
	} {
		policies := table.Policies(resource)
		if len(policies) == 0 {
			t.Fatalf("%s policy table is empty", resource)
		}
		seen := make(map[string]struct{}, len(policies))
		for _, policy := range policies {
			if _, duplicate := seen[policy.Path]; duplicate {
				t.Fatalf("duplicate %s policy %q", resource, policy.Path)
			}
			seen[policy.Path] = struct{}{}
			if policy.Resource != resource ||
				len(policy.SourceOrder) == 0 ||
				policy.Merge == "" ||
				policy.Empty == "" ||
				strings.TrimSpace(policy.Rationale) == "" {
				t.Errorf("incomplete %s policy: %#v", resource, policy)
			}
		}
		policies[0].SourceOrder[0] = "mutated"
		if table.Policies(resource)[0].SourceOrder[0] == "mutated" {
			t.Fatalf("%s policies share caller-owned source order", resource)
		}
	}
}

func TestProviderScopedDynamicFactsAndOperatorConfigurationHaveCanonicalOrder(t *testing.T) {
	table := New()
	tests := []struct {
		resource sources.ResourceType
		path     string
		want     sources.ID
	}{
		{sources.ResourceTypeModel, "Pricing", sources.ProvidersID},
		{sources.ResourceTypeModel, "Limits", sources.ProvidersID},
		{sources.ResourceTypeModel, "Metadata", sources.ModelsDevHTTPID},
		{sources.ResourceTypeProvider, "Name", sources.ProvidersID},
		{sources.ResourceTypeProvider, "Catalog", sources.LocalCatalogID},
		{sources.ResourceTypeProvider, "APIKey", sources.LocalCatalogID},
	}
	for _, test := range tests {
		policy, found := table.Find(test.resource, test.path)
		if !found {
			t.Fatalf("Find(%s, %q) returned no policy", test.resource, test.path)
		}
		if got := policy.SourceOrder[0]; got != test.want {
			t.Errorf("Find(%s, %q) first source = %q, want %q", test.resource, test.path, got, test.want)
		}
	}
}

func hasPolicyForField(policies []Policy, field string) bool {
	for _, policy := range policies {
		if policy.Path == field || strings.HasPrefix(policy.Path, field+".") {
			return true
		}
	}
	return false
}

func TestAuthorityScoreDerivesFromSourceOrder(t *testing.T) {
	policy, found := New().Find(sources.ResourceTypeModel, "Pricing")
	if !found {
		t.Fatal("pricing policy not found")
	}
	if got := policy.Authority(sources.ProvidersID); got != 1 {
		t.Fatalf("provider authority = %v, want 1", got)
	}
	if got := policy.Authority("unknown"); got != 0 {
		t.Fatalf("unknown authority = %v, want 0", got)
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		fieldPath string
		pattern   string
		want      bool
	}{
		{fieldPath: "Pricing", pattern: "Pricing", want: true},
		{fieldPath: "Catalog.Endpoint.URL", pattern: "Catalog.*", want: true},
		{fieldPath: "Metadata.ReleaseDate", pattern: "Metadata.*Date", want: true},
		{fieldPath: "Metadata.ReleaseDate", pattern: "Pricing.*", want: false},
		{fieldPath: "Metadata.ReleaseDate", pattern: "[", want: false},
	}
	for _, test := range tests {
		if got := MatchesPattern(test.fieldPath, test.pattern); got != test.want {
			t.Errorf("MatchesPattern(%q, %q) = %v, want %v", test.fieldPath, test.pattern, got, test.want)
		}
	}
}
