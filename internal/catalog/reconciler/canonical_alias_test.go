package reconciler

import (
	"bytes"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestCanonicalAliasReconciliationPreservesOriginalReferences(t *testing.T) {
	for _, state := range []catalogs.CanonicalAliasState{catalogs.CanonicalAliasActive, catalogs.CanonicalAliasRemoved} {
		t.Run(string(state), func(t *testing.T) {
			baselineBuilder, err := catalogs.NewBuilderFrom(corpusCatalog(t, "current", "Current Model"))
			if err != nil {
				t.Fatal(err)
			}
			if err := baselineBuilder.SetCanonicalAliasRecords([]catalogs.CanonicalAlias{{
				ID: "author/old", TargetID: "author/current", PublisherID: "upstream", State: state,
			}}); err != nil {
				t.Fatal(err)
			}
			baseline, err := baselineBuilder.Build()
			if err != nil {
				t.Fatal(err)
			}
			olderBuilder, err := catalogs.NewBuilderFrom(authoredOnlyCatalog(t, "old", "Old Model"))
			if err != nil {
				t.Fatal(err)
			}
			if err := olderBuilder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider", Models: map[string]*catalogs.Model{
				"new-wire": {ID: "new-wire", Name: "Another Offering", ModelRef: "author/old"},
			}}); err != nil {
				t.Fatal(err)
			}
			older, err := olderBuilder.Build()
			if err != nil {
				t.Fatal(err)
			}
			before, err := catalogs.EncodeCatalogPayload(older)
			if err != nil {
				t.Fatal(err)
			}
			result := reconcileCorpus(t, baseline, []sources.Observation{
				completeCorpusObservation(sources.EmbeddedCatalogID, older),
			})
			if len(result.Definitions()) != 1 {
				t.Fatal("stale embedded authoring recreated a retired model")
			}
			offering, err := result.Offering("provider", "new-wire")
			if err != nil || offering.DefinitionID != "author/current" {
				t.Fatalf("retained canonical reference = %s, %v", offering.DefinitionID, err)
			}
			if target, gotState, found := result.CanonicalAliases().Lookup("author/old"); !found || gotState != state || target != "author/current" {
				t.Fatal("reconciliation changed accepted rename history")
			}
			after, err := catalogs.EncodeCatalogPayload(older)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("reconciliation rewrote original observation bytes")
			}
		})
	}
}

func TestCanonicalAliasRejectsRetiredLocalDefinition(t *testing.T) {
	builder, err := catalogs.NewBuilderFrom(corpusCatalog(t, "current", "Current Model"))
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.SetCanonicalAliasRecords([]catalogs.CanonicalAlias{{
		ID: "author/old", TargetID: "author/current", PublisherID: "upstream", State: catalogs.CanonicalAliasActive,
	}}); err != nil {
		t.Fatal(err)
	}
	baseline, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(WithBaseline(baseline))
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Sources(t.Context(), sources.LocalCatalogID, []sources.Observation{
		completeCorpusObservation(sources.LocalCatalogID, corpusCatalog(t, "old", "Local Edit")),
	})
	if !errors.IsConflict(err) {
		t.Fatalf("local retired ID = %v, want actionable conflict", err)
	}
}
