package authority

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestCanonicalAttributePolicyInventoryCoversSchema(t *testing.T) {
	tests := []struct {
		resource sources.ResourceType
		typeOf   reflect.Type
	}{
		{sources.ResourceTypeModelDefinition, reflect.TypeOf(catalogs.ModelDefinition{})},
		{sources.ResourceTypeProviderOffering, reflect.TypeOf(catalogs.ProviderOffering{})},
	}
	for _, test := range tests {
		t.Run(test.resource.String(), func(t *testing.T) {
			policies := CanonicalPolicies(test.resource)
			if len(policies) == 0 {
				t.Fatal("canonical policy inventory is empty")
			}
			for _, path := range canonicalLeafPaths(test.typeOf, "") {
				policy, found := FindCanonicalPolicy(test.resource, path)
				if !found {
					t.Errorf("canonical attribute %s has no authority policy", path)
					continue
				}
				if len(policy.AuthorityOrder) == 0 || policy.Merge == "" || policy.Empty == "" || strings.TrimSpace(policy.Rationale) == "" {
					t.Errorf("canonical attribute %s has incomplete policy: %#v", path, policy)
				}
			}
		})
	}
}

func TestCanonicalPoliciesAreUniqueAndCallerOwned(t *testing.T) {
	for _, resource := range []sources.ResourceType{sources.ResourceTypeModelDefinition, sources.ResourceTypeProviderOffering} {
		policies := CanonicalPolicies(resource)
		seen := make(map[string]struct{}, len(policies))
		for _, policy := range policies {
			if _, duplicate := seen[policy.Path]; duplicate {
				t.Fatalf("duplicate %s policy %q", resource, policy.Path)
			}
			seen[policy.Path] = struct{}{}
		}
		policies[0].AuthorityOrder[0] = "mutated"
		if CanonicalPolicies(resource)[0].AuthorityOrder[0] == "mutated" {
			t.Fatalf("%s policies share caller-owned authority order", resource)
		}
	}
}

func TestCanonicalPricingPolicyIsProviderFirstAtomicAndAbsentAware(t *testing.T) {
	policy, found := FindCanonicalPolicy(sources.ResourceTypeProviderOffering, "Pricing.Tokens.Input")
	if !found {
		t.Fatal("pricing policy not found")
	}
	if policy.AuthorityOrder[0] != sources.ProvidersID || policy.Merge != MergeReplace || policy.Empty != EmptyAbsent {
		t.Fatalf("pricing policy = %#v", policy)
	}
}

func canonicalLeafPaths(value reflect.Type, prefix string) []string {
	paths := make([]string, 0)
	for index := range value.NumField() {
		field := value.Field(index)
		if field.PkgPath != "" {
			continue
		}
		path := field.Name
		if prefix != "" {
			path = prefix + "." + path
		}
		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct && fieldType.PkgPath() == value.PkgPath() {
			paths = append(paths, canonicalLeafPaths(fieldType, path)...)
			continue
		}
		paths = append(paths, path)
	}
	return paths
}
