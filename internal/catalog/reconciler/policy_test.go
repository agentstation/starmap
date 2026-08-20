package reconciler

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

type seamAuthority struct {
	policy authority.Policy
}

func (a seamAuthority) Find(resource evidence.ResourceType, path string) (authority.Policy, bool) {
	if resource != a.policy.Resource || path != a.policy.Path {
		return authority.Policy{}, false
	}
	return a.policy, true
}

func (a seamAuthority) Policies(resource evidence.ResourceType) []authority.Policy {
	if resource != a.policy.Resource {
		return nil
	}
	return []authority.Policy{a.policy}
}

func TestSeamConformanceAuthorityAcceptsCustomAdapter(t *testing.T) {
	auth := seamAuthority{policy: authority.Policy{
		Resource:    evidence.ResourceTypeModel,
		Path:        "Pricing",
		SourceOrder: []sources.ID{sources.LocalCatalogID, sources.ModelsDevHTTPID},
		Merge:       authority.MergeReplace,
		Empty:       authority.EmptyAbsent,
		Rationale:   "test replacement policy",
	}}
	strategy := NewAuthorityStrategy(auth)

	_, source, _ := strategy.ResolveResourceConflict(
		evidence.ResourceTypeModel,
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

func TestExecutablePolicyPathsReferenceCatalogFields(t *testing.T) {
	tests := []struct {
		resource evidence.ResourceType
		typ      reflect.Type
	}{
		{resource: evidence.ResourceTypeModel, typ: reflect.TypeFor[catalogs.Model]()},
		{resource: evidence.ResourceTypeProvider, typ: reflect.TypeFor[catalogs.Provider]()},
		{resource: evidence.ResourceTypeAuthor, typ: reflect.TypeFor[catalogs.Author]()},
	}
	table := authority.New()
	for _, test := range tests {
		t.Run(test.resource.String(), func(t *testing.T) {
			for _, policy := range table.Policies(test.resource) {
				if policy.Resource != test.resource {
					t.Fatalf("policy %q has resource %q, want %q", policy.Path, policy.Resource, test.resource)
				}
				if !hasReflectPath(test.typ, policy.Path) {
					t.Fatalf("%s policy %q does not exist on %s", test.resource, policy.Path, test.typ.Name())
				}
				resolved, found := table.Find(test.resource, policy.Path)
				if !found || resolved.Path != policy.Path {
					t.Fatalf("%s policy %q is not its own executable winner", test.resource, policy.Path)
				}
			}
		})
	}
}

func TestFieldPolicyAuthorityResolution(t *testing.T) {
	resolver := NewAuthorityStrategy(authority.New())
	values := map[sources.ID]any{
		sources.LocalCatalogID:  "local",
		sources.ModelsDevHTTPID: "models.dev",
		sources.ProvidersID:     "provider",
	}
	tests := []struct {
		resource evidence.ResourceType
		field    string
		want     sources.ID
	}{
		{evidence.ResourceTypeModel, "ModelRef", sources.ProvidersID},
		{evidence.ResourceTypeModel, "Name", sources.ProvidersID},
		{evidence.ResourceTypeModel, "Description", sources.ModelsDevHTTPID},
		{evidence.ResourceTypeProvider, "Catalog", sources.LocalCatalogID},
		{evidence.ResourceTypeProvider, "Name", sources.ProvidersID},
		{evidence.ResourceTypeProvider, "Models", sources.ProvidersID},
		{evidence.ResourceTypeAuthor, "Website", sources.ModelsDevHTTPID},
	}
	for _, test := range tests {
		_, got, _ := resolver.ResolveResourceConflict(test.resource, test.field, values)
		if got != test.want {
			t.Errorf("ResolveResourceConflict(%s, %q) source = %q, want %q", test.resource, test.field, got, test.want)
		}
	}
}

func TestModelReferenceUsesExplicitAuthorityAndNeverProviderIdentity(t *testing.T) {
	t.Parallel()

	authorities := authority.New()
	tracker := provenance.NewTracker(true)
	merger := newMergerWithProvenance(
		authorities,
		NewAuthorityStrategy(authorities),
		tracker,
		nil,
	)
	models, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.LocalCatalogID: {{
			ID: "kimi-k2.5", ModelRef: "local-fallback/kimi-k2.5", Name: "Kimi K2.5",
		}},
		sources.ProvidersID: {{
			ID: "kimi-k2.5", ModelRef: "moonshot-ai/kimi-k2.5", Name: "Kimi K2.5",
		}},
	})
	if err != nil {
		t.Fatalf("MergeModels: %v", err)
	}
	if len(models) != 1 || models[0].ModelRef != "moonshot-ai/kimi-k2.5" {
		t.Fatalf("merged model reference = %#v, want explicit Moonshot identity", models)
	}
}

func TestMergeModelsUsesPolicyEvidencePaths(t *testing.T) {
	authorities := authority.New()
	tracker := provenance.NewTracker(true)
	merger := newMergerWithProvenance(
		authorities,
		NewAuthorityStrategy(authorities),
		tracker,
		nil,
	)

	_, prov, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.LocalCatalogID: {{
			ID:          "model-1",
			Name:        "Local Name",
			Description: "Curated description",
		}},
		sources.ProvidersID: {{
			ID:          "model-1",
			Name:        "Provider Name",
			Description: "Provider description",
		}},
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

func TestStructuredModelPoliciesUseInjectedOrder(t *testing.T) {
	tests := []struct {
		name   string
		policy authority.Policy
		local  *catalogs.Model
		live   *catalogs.Model
		assert func(*testing.T, *catalogs.Model)
	}{
		{
			name: "limits",
			policy: authority.Policy{
				Resource:    evidence.ResourceTypeModel,
				Path:        "Limits",
				SourceOrder: []sources.ID{sources.LocalCatalogID, sources.ProvidersID},
				Merge:       authority.MergeFillMissing,
				Empty:       authority.EmptyAbsent,
				Rationale:   "test local-first limit policy",
			},
			local: &catalogs.Model{ID: "model-1", Limits: &catalogs.ModelLimits{ContextWindow: 7}},
			live:  &catalogs.Model{ID: "model-1", Limits: &catalogs.ModelLimits{ContextWindow: 9, OutputTokens: 3}},
			assert: func(t *testing.T, model *catalogs.Model) {
				t.Helper()
				if model.Limits == nil || model.Limits.ContextWindow != 7 || model.Limits.OutputTokens != 3 {
					t.Fatalf("limits = %#v, want local winner with provider gap fill", model.Limits)
				}
			},
		},
		{
			name: "features",
			policy: authority.Policy{
				Resource:    evidence.ResourceTypeModel,
				Path:        "Features",
				SourceOrder: []sources.ID{sources.LocalCatalogID, sources.ProvidersID},
				Merge:       authority.MergeDeep,
				Empty:       authority.EmptyAuthoritative,
				Rationale:   "test local-first feature policy",
			},
			local: &catalogs.Model{
				ID:       "model-1",
				Features: explicitlySupported(catalogs.ModelFeatureTools, false),
			},
			live: &catalogs.Model{ID: "model-1", Features: &catalogs.ModelFeatures{Tools: true}},
			assert: func(t *testing.T, model *catalogs.Model) {
				t.Helper()
				if model.Features == nil || model.Features.Tools {
					t.Fatalf("features = %#v, want explicit local false winner", model.Features)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := seamAuthority{policy: test.policy}
			merger := newMerger(reader, NewAuthorityStrategy(reader), nil)
			models, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
				sources.LocalCatalogID: {test.local},
				sources.ProvidersID:    {test.live},
			})
			if err != nil {
				t.Fatalf("Models: %v", err)
			}
			if len(models) != 1 {
				t.Fatalf("models = %d, want 1", len(models))
			}
			test.assert(t, models[0])
		})
	}
}

func hasReflectPath(typ reflect.Type, path string) bool {
	current := typ
	for part := range strings.SplitSeq(path, ".") {
		for current.Kind() == reflect.Pointer {
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
