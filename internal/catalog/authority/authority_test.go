package authority_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/catalog/authority"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestPoliciesCoverReconciledCatalogFields(t *testing.T) {
	tests := []struct {
		resource catalogmeta.ResourceType
		typ      reflect.Type
		ignored  map[string]bool
	}{
		{
			resource: catalogmeta.ResourceTypeModel,
			typ:      reflect.TypeFor[catalogs.Model](),
			ignored:  map[string]bool{"ID": true, "CreatedAt": true, "UpdatedAt": true},
		},
		{
			resource: catalogmeta.ResourceTypeProvider,
			typ:      reflect.TypeFor[catalogs.Provider](),
			ignored:  map[string]bool{"ID": true},
		},
		{
			resource: catalogmeta.ResourceTypeAuthor,
			typ:      reflect.TypeFor[catalogs.Author](),
			ignored:  map[string]bool{"ID": true, "CreatedAt": true, "UpdatedAt": true},
		},
	}
	table := authority.New()
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
	table := authority.New()
	for _, resource := range []catalogmeta.ResourceType{
		catalogmeta.ResourceTypeModel,
		catalogmeta.ResourceTypeProvider,
		catalogmeta.ResourceTypeAuthor,
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
			wantEvidence := policy.Path
			if policy.EvidencePath != "" {
				wantEvidence = policy.EvidencePath
			}
			if got := policy.Evidence(); got != wantEvidence || strings.TrimSpace(got) == "" {
				t.Errorf("%s policy %q evidence = %q, want %q", resource, policy.Path, got, wantEvidence)
			}
		}
		policies[0].SourceOrder[0] = "mutated"
		if table.Policies(resource)[0].SourceOrder[0] == "mutated" {
			t.Fatalf("%s policies share caller-owned source order", resource)
		}
	}
}

func TestProviderScopedDynamicFactsAndOperatorConfigurationHaveCanonicalOrder(t *testing.T) {
	table := authority.New()
	tests := []struct {
		resource catalogmeta.ResourceType
		path     string
		want     catalogmeta.SourceID
	}{
		{catalogmeta.ResourceTypeModel, "ModelRef", catalogmeta.ProvidersID},
		{catalogmeta.ResourceTypeModel, "Pricing", catalogmeta.ProvidersID},
		{catalogmeta.ResourceTypeModel, "Limits", catalogmeta.ProvidersID},
		{catalogmeta.ResourceTypeModel, "Metadata", catalogmeta.ModelsDevHTTPID},
		{catalogmeta.ResourceTypeProvider, "Name", catalogmeta.ProvidersID},
		{catalogmeta.ResourceTypeProvider, "Catalog", catalogmeta.LocalCatalogID},
		{catalogmeta.ResourceTypeProvider, "Credentials", catalogmeta.LocalCatalogID},
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

func TestVerifiedReleaseIsAboveEmbeddedAndBelowHumanAuthority(t *testing.T) {
	t.Parallel()

	table := authority.New()
	for _, resource := range []catalogmeta.ResourceType{
		catalogmeta.ResourceTypeModel,
		catalogmeta.ResourceTypeProvider,
		catalogmeta.ResourceTypeAuthor,
	} {
		for _, policy := range table.Policies(resource) {
			releaseIndex := -1
			embeddedIndex := -1
			localIndex := -1
			for index, source := range policy.SourceOrder {
				switch source {
				case catalogmeta.ReleaseArtifactID:
					releaseIndex = index
				case catalogmeta.EmbeddedCatalogID:
					embeddedIndex = index
				case catalogmeta.LocalCatalogID:
					localIndex = index
				}
			}
			if releaseIndex < 0 || embeddedIndex < 0 || localIndex < 0 {
				t.Fatalf("%s/%s source order = %v", resource, policy.Path, policy.SourceOrder)
			}
			if releaseIndex >= embeddedIndex || localIndex >= releaseIndex {
				t.Fatalf(
					"%s/%s source order = %v; want local above release above embedded",
					resource,
					policy.Path,
					policy.SourceOrder,
				)
			}
		}
	}
}

func hasPolicyForField(policies []authority.Policy, field string) bool {
	for _, policy := range policies {
		if policy.Path == field || strings.HasPrefix(policy.Path, field+".") {
			return true
		}
	}
	return false
}

func TestAuthorityScoreDerivesFromSourceOrder(t *testing.T) {
	policy, found := authority.New().Find(catalogmeta.ResourceTypeModel, "Pricing")
	if !found {
		t.Fatal("pricing policy not found")
	}
	if got := policy.Authority(catalogmeta.ProvidersID); got != 1 {
		t.Fatalf("provider authority = %v, want 1", got)
	}
	if got := policy.Authority("unknown"); got != 0 {
		t.Fatalf("unknown authority = %v, want 0", got)
	}
}

func TestEveryPolicyAuthorityRankIsUniqueAndStrictlyDescending(t *testing.T) {
	table := authority.New()
	for _, resource := range []catalogmeta.ResourceType{
		catalogmeta.ResourceTypeModel,
		catalogmeta.ResourceTypeProvider,
		catalogmeta.ResourceTypeAuthor,
	} {
		for _, policy := range table.Policies(resource) {
			t.Run(resource.String()+"/"+policy.Path, func(t *testing.T) {
				seen := make(map[catalogmeta.SourceID]struct{}, len(policy.SourceOrder))
				previous := 2.0
				for _, source := range policy.SourceOrder {
					if _, duplicate := seen[source]; duplicate {
						t.Fatalf("source order contains duplicate %q", source)
					}
					seen[source] = struct{}{}
					score := policy.Authority(source)
					if score <= 0 || score >= previous {
						t.Fatalf("authority(%q) = %v after %v; want positive strict descent", source, score, previous)
					}
					previous = score
				}
				if score := policy.Authority("unconfigured"); score != 0 {
					t.Fatalf("unconfigured authority = %v, want 0", score)
				}
			})
		}
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
		if got := authority.MatchesPattern(test.fieldPath, test.pattern); got != test.want {
			t.Errorf("MatchesPattern(%q, %q) = %v, want %v", test.fieldPath, test.pattern, got, test.want)
		}
	}
}
