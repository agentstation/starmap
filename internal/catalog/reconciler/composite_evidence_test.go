package reconciler

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestCompositePriceAliasesExcludeOpaqueFields(t *testing.T) {
	left := map[string]any{"tokens": map[string]any{"input": map[string]any{"per_1m_tokens": json.Number("2.5"), "per_token": json.Number("0")}}}
	right := map[string]any{"tokens": map[string]any{"input": map[string]any{"per_1m": json.Number("2.5")}}}
	for _, scenario := range []struct {
		path  string
		equal bool
	}{
		{path: "pricing", equal: true},
		{path: `modes["fast"].pricing`, equal: true},
		{path: `modes["fast.a\\\"b"].pricing`, equal: true},
		{path: `modes["fast"].provider.body["pricing"]`},
		{path: `extensions["pricing"].fields["price"]`},
		{path: `modes[fast].pricing`},
		{path: `modes['x'].pricing`},
		{path: `modes["fast"].pricing.extra`},
		{path: `modes["fast"].provider.body["price"].pricing`},
	} {
		t.Run(scenario.path, func(t *testing.T) {
			if got := semanticValueEqual(scenario.path, left, right); got != scenario.equal {
				t.Errorf("semantic equality=%t, want %t", got, scenario.equal)
			}
		})
	}
}

func TestSemanticNumericEvidenceKeepsTypesAndPrecision(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		left, right any
		equal       bool
	}{
		{name: "typed-number", left: json.Number("2.5"), right: 2.5, equal: true},
		{name: "number-and-string", left: json.Number("0"), right: "0"},
		{name: "distinct-large-integers", left: json.Number("9007199254740993"), right: json.Number("9007199254740992")},
		{name: "nested-number-and-string", left: map[string]any{"value": json.Number("0")}, right: map[string]any{"value": "0"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if got := semanticValueEqual(`extensions["source"].fields["value"]`, scenario.left, scenario.right); got != scenario.equal {
				t.Errorf("semantic equality=%t, want %t", got, scenario.equal)
			}
		})
	}
}

func TestCompositeWorkspaceRetainsOriginalReceipts(t *testing.T) {
	metadata := &catalogs.ModelMetadata{Tags: []catalogs.ModelTag{"chat"}}
	metadata.SetOpenWeights(false)
	input := sourceIdentityCatalog(t, "", catalogs.Model{
		ID: "shared", Name: "Shared", Metadata: metadata,
		Modes: map[string]catalogs.ModelMode{"fast": {
			Pricing: sourceIdentityPricing(2.5),
			Provider: &catalogs.ModelProviderMode{
				Headers: map[string]string{"value": "original"}, Body: map[string]any{"null": nil},
			},
		}},
		Extensions: catalogs.SourceExtensions{"source": {Fields: map[string]any{"false": false, "null": nil}}},
	})
	original := sourceIdentityObservation(t, sources.ProvidersID, input, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
	generated := sourceIdentityReconcile(t, sources.ProvidersID, original)
	for _, format := range []string{"payload", "workspace"} {
		t.Run(format, func(t *testing.T) {
			local := snapshotForTest(t, generated.Catalog)
			if format == "workspace" {
				workspace := filepath.Join(t.TempDir(), "catalog")
				if err := generated.Catalog.SaveTo(workspace); err != nil {
					t.Fatal(err)
				}
				local = sourceIdentityLoad(t, workspace)
			}
			observation := sourceIdentityObservation(t, sources.LocalCatalogID, local, original.ObservedAt.Add(time.Minute))
			result := sourceIdentityReconcile(t, sources.LocalCatalogID, observation)
			for _, field := range []string{
				"metadata.open_weights", `metadata.tags["chat"]`, `modes["fast"].pricing`,
				`modes["fast"].provider.headers["value"]`, `modes["fast"].provider.body["null"]`,
				`extensions["source"].fields["false"]`, `extensions["source"].fields["null"]`,
			} {
				t.Run(field, func(t *testing.T) {
					assertModelEvidenceSource(t, result.Catalog, field, sources.ProvidersID, original.ID)
				})
			}
		})
	}
}

func TestCompositePartialUpdatePreservesBaselineFields(t *testing.T) {
	at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	baseline := sourceIdentityCatalog(t, "", catalogs.Model{
		ID: "shared", Name: "Shared",
		Metadata: &catalogs.ModelMetadata{Tags: []catalogs.ModelTag{"existing"}},
		Modes: map[string]catalogs.ModelMode{"fast": {Provider: &catalogs.ModelProviderMode{
			Headers: map[string]string{"existing": "keep"}, Body: map[string]any{"existing": false},
		}}},
	})
	candidate := sourceIdentityCatalog(t, "", catalogs.Model{
		ID: "shared", Name: "Shared",
		Metadata: &catalogs.ModelMetadata{Tags: []catalogs.ModelTag{"existing", "added"}},
		Modes: map[string]catalogs.ModelMode{"fast": {Provider: &catalogs.ModelProviderMode{
			Headers: map[string]string{"added": "new"}, Body: map[string]any{"added": true},
		}}},
	})
	observation := sourceIdentityObservation(t, sources.ProvidersID, candidate, at)
	result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{observation})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := result.Catalog.Provider("provider-a")
	if err != nil {
		t.Fatal(err)
	}
	model := provider.Models["shared"]
	if !slices.Equal(model.Metadata.Tags, []catalogs.ModelTag{"existing", "added"}) {
		t.Errorf("tags=%v, want each baseline and new tag once", model.Metadata.Tags)
	}
	mode := model.Modes["fast"].Provider
	if mode == nil || mode.Headers["existing"] != "keep" || mode.Headers["added"] != "new" || mode.Body["existing"] != false || mode.Body["added"] != true {
		t.Errorf("mode=%+v, want baseline and new overrides", mode)
	}
}

func TestCompositeContributionsPreserveEmptyRecordsAndCallerOwnership(t *testing.T) {
	policy := authority.New()
	merger := newMerger(policy, NewAuthorityStrategy(policy), nil)
	parent := "base"
	original := &catalogs.Model{ID: "model", Name: "Model", Metadata: &catalogs.ModelMetadata{Architecture: &catalogs.ModelArchitecture{}}, Modes: map[string]catalogs.ModelMode{"fast": {Provider: &catalogs.ModelProviderMode{}}}}
	models, _, err := merger.Models(map[sources.ID][]*catalogs.Model{sources.ProvidersID: {original}})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].Metadata.Architecture == nil || models[0].Modes["fast"].Provider == nil {
		t.Fatal("reconciliation lost a present empty record")
	}
	original.Metadata.Architecture.BaseModel = &parent
	original.Modes["fast"].Provider.Body = map[string]any{"nested": map[string]any{"value": "original"}}
	original.Extensions = catalogs.SourceExtensions{"source": {Fields: map[string]any{"nested": map[string]any{"value": "original"}}}}
	models, _, err = merger.Models(map[sources.ID][]*catalogs.Model{sources.ProvidersID: {original}})
	if err != nil {
		t.Fatal(err)
	}
	*models[0].Metadata.Architecture.BaseModel = "changed"
	models[0].Modes["fast"].Provider.Body["nested"].(map[string]any)["value"] = "changed"
	models[0].Extensions["source"].Fields["nested"].(map[string]any)["value"] = "changed"
	if parent != "base" || original.Modes["fast"].Provider.Body["nested"].(map[string]any)["value"] != "original" || original.Extensions["source"].Fields["nested"].(map[string]any)["value"] != "original" {
		t.Fatal("reconciliation returned mutable source aliases")
	}
}

func TestCompositeProjectionPolicyRejectsCarriedFactsAndAllowsEdits(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, edited := range []bool{false, true} {
			t.Run("legacy="+strconv.FormatBool(legacy)+"/edited="+strconv.FormatBool(edited), func(t *testing.T) {
				metadata := &catalogs.ModelMetadata{Architecture: &catalogs.ModelArchitecture{}}
				metadata.SetOpenWeights(true)
				input := sourceIdentityCatalog(t, "", catalogs.Model{ID: "shared", Name: "Shared", Metadata: metadata, Modes: map[string]catalogs.ModelMode{"fast": {Provider: &catalogs.ModelProviderMode{Headers: map[string]string{"value": "original"}}}}, Extensions: catalogs.SourceExtensions{"source": {Fields: map[string]any{"value": "original"}}}})
				original := sourceIdentityObservation(t, sources.ProvidersID, input, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
				generated := sourceIdentityReconcile(t, sources.ProvidersID, original)
				local := snapshotForTest(t, generated.Catalog)
				if legacy {
					local = aggregateOnlyCompositeCatalog(t, local)
				}
				if edited {
					builder, err := catalogs.NewBuilderFrom(local)
					if err != nil {
						t.Fatal(err)
					}
					provider, err := builder.Provider("provider-a")
					if err != nil {
						t.Fatal(err)
					}
					model := provider.Models["shared"]
					model.Metadata.SetOpenWeights(false)
					model.Modes["fast"].Provider.Headers["value"] = "edited"
					model.Extensions["source"].Fields["value"] = "edited"
					if err := builder.SetProvider(provider); err != nil {
						t.Fatal(err)
					}
					local, err = builder.Build()
					if err != nil {
						t.Fatal(err)
					}
				}
				observation := sourceIdentityObservation(t, sources.LocalCatalogID, local, original.ObservedAt.Add(time.Minute))
				reconcile, err := New(WithBaseline(local), WithProjectedEvidencePolicy(func(_ catalogs.ProviderID, entry provenance.Entry) bool {
					composite := strings.HasPrefix(entry.Field, "metadata") || strings.HasPrefix(entry.Field, "modes") || strings.HasPrefix(entry.Field, "extensions")
					return !composite || entry.ObservationID != original.ID
				}))
				if err != nil {
					t.Fatal(err)
				}
				result, err := reconcile.Sources(t.Context(), sources.LocalCatalogID, []sources.Observation{observation})
				if err != nil {
					t.Fatal(err)
				}
				provider, err := result.Catalog.Provider("provider-a")
				if err != nil {
					t.Fatal(err)
				}
				model := provider.Models["shared"]
				if !edited {
					if model.Metadata != nil || len(model.Modes) != 0 || len(model.Extensions) != 0 {
						t.Fatalf("rejected composite facts survived: metadata=%+v modes=%+v extensions=%+v", model.Metadata, model.Modes, model.Extensions)
					}
					return
				}
				open, presence := model.Metadata.OpenWeightsValue()
				if open || presence != catalogs.ValueKnown || model.Modes["fast"].Provider.Headers["value"] != "edited" || model.Extensions["source"].Fields["value"] != "edited" {
					t.Fatal("projection policy discarded an explicit operator edit")
				}
				for _, field := range []string{"metadata.present", `modes["fast"].present`, `modes["fast"].provider.present`, `extensions["source"].present`} {
					assertModelEvidenceSource(t, result.Catalog, field, sources.LocalCatalogID, observation.ID)
				}
				snapshot := snapshotForTest(t, result.Catalog)
				next := sourceIdentityObservation(t, sources.LocalCatalogID, snapshot, observation.ObservedAt.Add(time.Minute))
				reconcile, err = New(WithBaseline(snapshot), WithProjectedEvidencePolicy(func(_ catalogs.ProviderID, entry provenance.Entry) bool {
					composite := strings.HasPrefix(entry.Field, "metadata") || strings.HasPrefix(entry.Field, "modes") || strings.HasPrefix(entry.Field, "extensions")
					return !composite || (entry.ObservationID != original.ID && entry.ObservationID != observation.ID)
				}))
				if err != nil {
					t.Fatal(err)
				}
				withdrawn, err := reconcile.Sources(t.Context(), sources.LocalCatalogID, []sources.Observation{next})
				if err != nil {
					t.Fatal(err)
				}
				provider, err = withdrawn.Catalog.Provider("provider-a")
				if err != nil {
					t.Fatal(err)
				}
				model = provider.Models["shared"]
				if model.Metadata != nil || len(model.Modes) != 0 || len(model.Extensions) != 0 {
					t.Fatal("a derived record survived withdrawal of all contributing evidence")
				}
			})
		}
	}
}

func TestCompositeStaleFallbackPreservesAcceptedBaseline(t *testing.T) {
	at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	baselineModel := catalogs.Model{ID: "shared", Name: "Shared", Metadata: &catalogs.ModelMetadata{}, Modes: map[string]catalogs.ModelMode{"fast": {Provider: &catalogs.ModelProviderMode{Headers: map[string]string{"value": "accepted"}}}}, Extensions: catalogs.SourceExtensions{"source": {Fields: map[string]any{"value": "accepted"}}}}
	baselineModel.Metadata.SetOpenWeights(false)
	baseline := sourceIdentityCatalog(t, "", baselineModel)
	candidateModel := catalogs.DeepCopyModel(baselineModel)
	candidateModel.Metadata.SetOpenWeights(true)
	candidateModel.Modes["fast"].Provider.Headers["value"] = "stale"
	candidateModel.Extensions["source"].Fields["value"] = "stale"
	candidate := sourceIdentityCatalog(t, "", candidateModel)
	observation, err := sources.NewObservation(sources.ProvidersID, candidate, sources.ObservationMetadata{ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded, Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeStaleFallback, Code: sources.ObservationIssueCodeStaleFallback, Message: "A source returned stale fallback data."}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{observation})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := result.Catalog.Provider("provider-a")
	if err != nil {
		t.Fatal(err)
	}
	model := provider.Models["shared"]
	open, presence := model.Metadata.OpenWeightsValue()
	if open || presence != catalogs.ValueKnown {
		t.Error("stale metadata replaced the accepted baseline")
	}
	mode, exists := model.Modes["fast"]
	if !exists || mode.Provider == nil || mode.Provider.Headers["value"] != "accepted" {
		t.Error("stale mode data erased or replaced the accepted baseline")
	}
	if model.Extensions["source"].Fields["value"] != "accepted" {
		t.Error("stale extensions erased or replaced the accepted baseline")
	}
	for _, field := range []string{"metadata", "modes", "extensions"} {
		for _, entry := range result.Catalog.Provenance().FindModelField("provider-a", "shared", field) {
			if entry.ObservationID == observation.ID {
				t.Errorf("%s summary claimed rejected stale evidence", field)
			}
		}
	}
}
