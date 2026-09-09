package reconciler

import (
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestCompositePolicyPreservesAcceptedFeatureContributions(t *testing.T) {
	at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	provider := sourceIdentityObservation(t, sources.ProvidersID, sourceIdentityCatalog(t, "", catalogs.Model{
		ID: "shared", Name: "Shared", Features: &catalogs.ModelFeatures{ToolCalls: true},
	}), at)
	supplemental := sourceIdentityObservation(t, sources.ModelsDevHTTPID, sourceIdentityCatalog(t, "", catalogs.Model{
		ID: "shared", Name: "Shared", Features: &catalogs.ModelFeatures{Attachments: true},
	}), at.Add(time.Minute))
	generated := sourceIdentityReconcile(t, sources.ProvidersID, provider, supplemental)
	for _, retained := range []sources.Observation{provider, supplemental} {
		t.Run(string(retained.SourceID), func(t *testing.T) {
			localCatalog := snapshotForTest(t, generated.Catalog)
			local := sourceIdentityObservation(t, sources.LocalCatalogID, localCatalog, at.Add(2*time.Minute))
			reconcile, err := New(WithBaseline(localCatalog), WithProjectedEvidencePolicy(func(_ catalogs.ProviderID, entry provenance.Entry) bool {
				return entry.ObservationID == retained.ID
			}))
			if err != nil {
				t.Fatal(err)
			}
			result, err := reconcile.Sources(t.Context(), sources.LocalCatalogID, []sources.Observation{local})
			if err != nil {
				t.Fatal(err)
			}
			selected, err := result.Catalog.Provider("provider-a")
			if err != nil {
				t.Fatal(err)
			}
			features := selected.Models["shared"].Features
			wantTools := retained.SourceID == sources.ProvidersID
			if features == nil || features.ToolCalls != wantTools || features.Attachments == wantTools {
				t.Fatalf("features=%+v, want only the accepted source contribution", features)
			}
			field := "Features.attachments"
			if wantTools {
				field = "Features.tool_calls"
			}
			for _, field := range []string{field, "Features.present", "Features"} {
				assertModelEvidenceSource(t, result.Catalog, field, retained.SourceID, retained.ID)
			}
		})
	}
}

func TestCompositePolicyClearsRejectedValuesAndCurrentReceipts(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "current"
		if legacy {
			name = "legacy-aggregate"
		}
		t.Run(name, func(t *testing.T) {
			seed := authorshipCatalog(t, catalogs.Author{ID: "first-author", Name: "First"})
			builder, err := catalogs.NewBuilderFrom(seed)
			if err != nil {
				t.Fatal(err)
			}
			provider, err := builder.Provider("provider-a")
			if err != nil {
				t.Fatal(err)
			}
			model := provider.Models["shared"]
			model.Metadata = &catalogs.ModelMetadata{}
			model.Metadata.SetOpenWeights(true)
			model.Modes = map[string]catalogs.ModelMode{"fast": {Provider: &catalogs.ModelProviderMode{Headers: map[string]string{"value": "prior"}}}}
			model.Extensions = catalogs.SourceExtensions{"source": {Fields: map[string]any{"value": true}}}
			model.Features = &catalogs.ModelFeatures{ToolCalls: true}
			if err := builder.SetProvider(provider); err != nil {
				t.Fatal(err)
			}
			input, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
			original := sourceIdentityObservation(t, sources.ProvidersID, input, at)
			generated := sourceIdentityReconcile(t, sources.ProvidersID, original)
			builder, err = catalogs.NewBuilderFrom(snapshotForTest(t, generated.Catalog))
			if err != nil {
				t.Fatal(err)
			}
			if legacy {
				entries := builder.Provenance().Map()
				for key, values := range entries {
					if len(values) == 0 {
						continue
					}
					for _, prefix := range []string{"metadata.", "modes[", "extensions[", "Authors[", "Features."} {
						if strings.HasPrefix(values[0].Field, prefix) {
							delete(entries, key)
							break
						}
					}
				}
				builder.SetProvenance(entries)
			}
			localCatalog, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			local := sourceIdentityObservation(t, sources.LocalCatalogID, localCatalog, at.Add(time.Minute))
			composite := func(field string) bool {
				for _, prefix := range []string{"metadata", "modes", "extensions", "Authors", "Features"} {
					if strings.HasPrefix(field, prefix) {
						return true
					}
				}
				return false
			}
			reconcile, err := New(WithBaseline(localCatalog), WithProjectedEvidencePolicy(func(_ catalogs.ProviderID, entry provenance.Entry) bool {
				return !composite(entry.Field) || entry.ObservationID != original.ID
			}))
			if err != nil {
				t.Fatal(err)
			}
			result, err := reconcile.Sources(t.Context(), sources.LocalCatalogID, []sources.Observation{local})
			if err != nil {
				t.Fatal(err)
			}
			provider, err = result.Catalog.Provider("provider-a")
			if err != nil {
				t.Fatal(err)
			}
			model = provider.Models["shared"]
			for _, scenario := range []struct {
				name, field string
				retained    bool
			}{
				{"metadata", "metadata.open_weights", model.Metadata != nil}, {"modes", `modes["fast"].provider.headers["value"]`, len(model.Modes) > 0}, {"extensions", `extensions["source"].fields["value"]`, len(model.Extensions) > 0}, {"Authors", `Authors["first-author"].present`, len(model.Authors) > 0}, {"Features", "Features.tool_calls", model.Features != nil && model.Features.ToolCalls},
			} {
				t.Run(scenario.name, func(t *testing.T) {
					if scenario.retained {
						t.Error("rejected value survived")
					}
					for _, field := range []string{scenario.name, scenario.field} {
						for _, entry := range result.Catalog.Provenance().FindModelField("provider-a", "shared", field) {
							if entry.ObservationID == original.ID {
								t.Error("rejected current receipt survived")
								return
							}
						}
					}
				})
			}
		})
	}
}

func aggregateOnlyCompositeCatalog(t testing.TB, input *catalogs.Catalog) *catalogs.Catalog {
	t.Helper()
	builder, err := catalogs.NewBuilderFrom(input)
	if err != nil {
		t.Fatal(err)
	}
	entries := builder.Provenance().Map()
	for key, values := range entries {
		if len(values) == 0 {
			continue
		}
		for _, prefix := range []string{"metadata.", "modes[", "extensions[", "Authors[", "Features."} {
			if strings.HasPrefix(values[0].Field, prefix) {
				delete(entries, key)
				break
			}
		}
	}
	builder.SetProvenance(entries)
	result, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	return result
}
