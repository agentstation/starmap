package reconciler

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

type seamAuthority struct {
	field authority.Field
}

func (a seamAuthority) Find(sources.ResourceType, string) *authority.Field { return &a.field }
func (a seamAuthority) ModelFields() []authority.Field                     { return []authority.Field{a.field} }
func (a seamAuthority) ProviderFields() []authority.Field                  { return []authority.Field{a.field} }
func (a seamAuthority) AuthorFields() []authority.Field                    { return []authority.Field{a.field} }

func TestSeamConformanceAuthorityAcceptsCustomAdapter(t *testing.T) {
	auth := seamAuthority{field: authority.Field{
		Path:     "Pricing",
		Source:   sources.LocalCatalogID,
		Priority: 999,
	}}
	strategy := NewAuthorityStrategy(auth)

	_, source, _ := strategy.ResolveResourceConflict(
		sources.ResourceTypeModel,
		"Pricing",
		map[sources.ID]any{
			sources.LocalCatalogID:  "curated",
			sources.ModelsDevHTTPID: "upstream",
		},
	)
	if source != sources.LocalCatalogID {
		t.Fatalf("custom authority selected %q, want %q", source, sources.LocalCatalogID)
	}
}

func TestFieldRulesReferenceCatalogFields(t *testing.T) {
	tests := []struct {
		resource sources.ResourceType
		typ      reflect.Type
	}{
		{resource: sources.ResourceTypeModel, typ: reflect.TypeOf(catalogs.Model{})},
		{resource: sources.ResourceTypeProvider, typ: reflect.TypeOf(catalogs.Provider{})},
		{resource: sources.ResourceTypeAuthor, typ: reflect.TypeOf(catalogs.Author{})},
	}

	for _, tt := range tests {
		t.Run(string(tt.resource), func(t *testing.T) {
			seen := make(map[string]bool)

			for _, rule := range fieldRulesFor(tt.resource) {
				if rule.resource != tt.resource {
					t.Fatalf("rule %q has resource %q, want %q", rule.reflectPath, rule.resource, tt.resource)
				}
				if rule.reflectPath == "" {
					t.Fatal("field rule has empty reflect path")
				}
				if seen[rule.reflectPath] {
					t.Fatalf("duplicate field rule for %q", rule.reflectPath)
				}
				seen[rule.reflectPath] = true

				if !hasReflectPath(tt.typ, rule.reflectPath) {
					t.Fatalf("%s rule %q does not exist on %s", tt.resource, rule.reflectPath, tt.typ.Name())
				}
			}
		})
	}
}

func TestModelProvenanceRulesReferenceCatalogFields(t *testing.T) {
	modelType := reflect.TypeOf(catalogs.Model{})
	seen := make(map[string]bool)

	for provenancePath, rule := range modelProvenanceFieldRules {
		if rule.resource != sources.ResourceTypeModel {
			t.Fatalf("rule %q has resource %q, want %q", provenancePath, rule.resource, sources.ResourceTypeModel)
		}
		if rule.provenance() != provenancePath {
			t.Fatalf("rule key %q has provenance path %q", provenancePath, rule.provenance())
		}
		if seen[provenancePath] {
			t.Fatalf("duplicate model provenance rule for %q", provenancePath)
		}
		seen[provenancePath] = true

		if !hasReflectPath(modelType, rule.reflectPath) {
			t.Fatalf("model provenance rule %q reflect path %q does not exist on Model", provenancePath, rule.reflectPath)
		}
	}
}

func TestFieldRulesHaveAuthorities(t *testing.T) {
	authorities := authority.New()

	for _, resource := range []sources.ResourceType{
		sources.ResourceTypeModel,
		sources.ResourceTypeProvider,
		sources.ResourceTypeAuthor,
	} {
		t.Run(string(resource), func(t *testing.T) {
			for _, rule := range fieldRulesFor(resource) {
				if authorities.Find(rule.resource, rule.authority()) == nil {
					t.Fatalf("missing authority for %s field %q", rule.resource, rule.authority())
				}
			}
		})
	}

	for provenancePath, rule := range modelProvenanceFieldRules {
		if authorities.Find(rule.resource, rule.authority()) == nil {
			t.Fatalf("missing authority for model provenance field %q authority path %q", provenancePath, rule.authority())
		}
	}
}

func TestSourceExtensionsExcludedFromAuthorityRules(t *testing.T) {
	for _, tt := range []struct {
		resource sources.ResourceType
		rules    []fieldRule
	}{
		{resource: sources.ResourceTypeModel, rules: fieldRulesFor(sources.ResourceTypeModel)},
		{resource: sources.ResourceTypeProvider, rules: fieldRulesFor(sources.ResourceTypeProvider)},
	} {
		for _, rule := range tt.rules {
			if rule.reflectPath == "Extensions" {
				t.Fatalf("%s extensions must stay out of authority field rules", tt.resource)
			}
		}
	}
}

func TestFieldRuleAuthorityResolution(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	var resolver resourceConflictResolver = strategy

	tests := []struct {
		name     string
		resource sources.ResourceType
		field    string
		want     sources.ID
	}{
		{
			name:     "model name uses provider API",
			resource: sources.ResourceTypeModel,
			field:    "Name",
			want:     sources.ProvidersID,
		},
		{
			name:     "model description uses local catalog",
			resource: sources.ResourceTypeModel,
			field:    "Description",
			want:     sources.LocalCatalogID,
		},
		{
			name:     "provider catalog uses local catalog",
			resource: sources.ResourceTypeProvider,
			field:    "Catalog",
			want:     sources.LocalCatalogID,
		},
		{
			name:     "provider models use provider API",
			resource: sources.ResourceTypeProvider,
			field:    "Models",
			want:     sources.ProvidersID,
		},
		{
			name:     "author website uses local catalog",
			resource: sources.ResourceTypeAuthor,
			field:    "Website",
			want:     sources.LocalCatalogID,
		},
	}

	values := map[sources.ID]any{
		sources.LocalCatalogID:  "local",
		sources.ModelsDevHTTPID: "models.dev",
		sources.ProvidersID:     "provider",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got, _ := resolver.ResolveResourceConflict(tt.resource, tt.field, values)
			if got != tt.want {
				t.Fatalf("ResolveResourceConflict(%s, %q) source = %q, want %q", tt.resource, tt.field, got, tt.want)
			}
		})
	}
}

func TestMergeModelsUsesFieldRuleProvenancePaths(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	tracker := provenance.NewTracker(true)
	merger := newMergerWithProvenance(authorities, strategy, tracker, nil)

	_, prov, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.LocalCatalogID: {
			{
				ID:          "model-1",
				Name:        "Local Name",
				Description: "Curated description",
			},
		},
		sources.ProvidersID: {
			{
				ID:          "model-1",
				Name:        "Provider Name",
				Description: "Provider description",
			},
		},
	})
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}

	for _, field := range []string{"Name", "Description"} {
		key := "models.model-1." + field
		if _, ok := prov[key]; !ok {
			t.Fatalf("expected provenance key %q", key)
		}
	}
}

func hasReflectPath(typ reflect.Type, path string) bool {
	current := typ
	for _, part := range strings.Split(path, ".") {
		if current.Kind() == reflect.Ptr {
			current = current.Elem()
		}
		if current.Kind() != reflect.Struct {
			return false
		}
		field, ok := current.FieldByName(part)
		if !ok {
			return false
		}
		current = field.Type
	}
	return true
}
