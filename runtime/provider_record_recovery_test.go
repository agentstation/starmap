package runtime

import (
	"bytes"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestProviderOptionalRecordRecoveryPreservesPresence(t *testing.T) {
	for _, scenario := range []struct {
		record catalogs.ModelRecord
		field  string
	}{
		{catalogs.ModelRecordDeprecatedAt, "DeprecatedAt"},
		{catalogs.ModelRecordRetiresAt, "RetiresAt"},
		{catalogs.ModelRecordMetadata, "Metadata"},
		{catalogs.ModelRecordLineage, "Lineage"},
		{catalogs.ModelRecordFeatures, "Features"},
		{catalogs.ModelRecordAttachments, "Attachments"},
		{catalogs.ModelRecordGeneration, "Generation"},
		{catalogs.ModelRecordReasoning, "Reasoning"},
		{catalogs.ModelRecordReasoningTokens, "ReasoningTokens"},
		{catalogs.ModelRecordVerbosity, "Verbosity"},
		{catalogs.ModelRecordTools, "Tools"},
		{catalogs.ModelRecordDelivery, "Delivery"},
		{catalogs.ModelRecordPricing, "Pricing"},
		{catalogs.ModelRecordLimits, "Limits"},
	} {
		for _, replace := range []bool{false, true} {
			mode := "retained"
			if replace {
				mode = "replaced"
			}
			t.Run(string(scenario.record)+"/"+mode, func(t *testing.T) {
				policy, found := authority.New().Find("model", scenario.field)
				if !found {
					t.Fatal("optional record requires an authority policy")
				}
				at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
				seed := manualProviderObservation(t, 0, at).Catalog
				payload, err := catalogs.EncodeCatalogPayload(seed)
				if err != nil {
					t.Fatal(err)
				}
				source := newStubSource("composite-recovery")
				source.replies = []SourceRead{testSourceRead(t, "composite-baseline", payload, at), testSourceRead(t, "composite-refresh", payload, at.Add(time.Hour))}
				store, err := storage.NewFilesystem(privateRuntimeDirectory(t))
				if err != nil {
					t.Fatal(err)
				}
				options := []Option{WithSource(source), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(store))}
				connected := openTestRuntime(t, options...)
				if _, err := connected.RefreshSource(t.Context()); err != nil {
					t.Fatal(err)
				}
				observe := func(partial bool) sources.Observation {
					builder, err := catalogs.NewBuilderFrom(seed)
					if err != nil {
						t.Fatal(err)
					}
					provider, err := builder.Provider("provider")
					if err != nil {
						t.Fatal(err)
					}
					model := provider.Models["model"]
					if partial || replace {
						if !model.SetRecordUnknown(scenario.record) {
							t.Fatal("unsupported optional record")
						}
					} else {
						model.UnsetRecord(scenario.record)
					}
					if err := builder.SetProvider(provider); err != nil {
						t.Fatal(err)
					}
					catalog, err := catalogs.NewObservationCatalog(builder)
					if err != nil {
						t.Fatal(err)
					}
					metadata := sources.ObservationMetadata{ObservedAt: at.Add(2 * time.Minute), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded}
					if partial {
						metadata.ObservedAt = at.Add(time.Minute)
						metadata.Completeness, metadata.Status = sources.ObservationCompletenessPartial, sources.ObservationStatusDegraded
						metadata.Issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "provider/invalid", Message: "A source record is invalid."}}
					}
					observation, err := sources.NewObservation(sources.ProvidersID, catalog, metadata)
					if err != nil {
						t.Fatal(err)
					}
					return observation
				}
				prior, current := observe(true), observe(false)
				originalProvider, err := prior.Catalog.Provider("provider")
				if err != nil {
					t.Fatal(err)
				}
				expected, present := compositeJSONValue(t, originalProvider.Models["model"], []string{string(scenario.record)})
				if !present {
					t.Fatal("fixture omitted its claimed value")
				}
				for _, observation := range []sources.Observation{prior, current} {
					if _, err := connected.PublishObservations(t.Context(), observation); err != nil {
						t.Fatal(err)
					}
				}
				assertState := func(connected *Runtime) {
					t.Helper()
					catalog := connected.State().Catalog
					provider, err := catalog.Provider("provider")
					if err != nil {
						t.Fatal(err)
					}
					model := provider.Models["model"]
					if model == nil || model.ModelRef == "" {
						t.Fatal("recovery lost linked membership")
					}
					actual, present := compositeJSONValue(t, model, []string{string(scenario.record)})
					if !present || !bytes.Equal(actual, expected) {
						t.Errorf("accepted value=%s/%t, want %s", actual, present, expected)
					}
					wantID := prior.ID
					if replace {
						wantID = current.ID
					}
					entries := catalog.Provenance().FindModelField("provider", "model", policy.Evidence())
					if len(entries) != 1 || entries[0].ObservationID != wantID {
						t.Errorf("contribution receipt=%+v, want %s", entries, wantID)
					}
					generation, err := store.Current(t.Context())
					if err != nil {
						t.Fatal(err)
					}
					oldFound, newFound := false, false
					for _, receipt := range generation.Manifest.SourceObservations {
						oldFound = oldFound || receipt.ObservationID == prior.ID
						newFound = newFound || receipt.ObservationID == current.ID
					}
					if oldFound == replace || !newFound || generation.Manifest.Degraded == replace {
						t.Errorf("receipts old=%t new=%t degraded=%t, replacement=%t", oldFound, newFound, generation.Manifest.Degraded, replace)
					}
					if len(manualBatches(connected.layers.manual)) != 2 {
						t.Error("recovery changed original observation history")
					}
				}
				assertState(connected)
				if err := connected.Close(); err != nil {
					t.Fatal(err)
				}
				reopened := openTestRuntime(t, options...)
				assertState(reopened)
				if _, err := reopened.RefreshSource(t.Context()); err != nil {
					t.Fatal(err)
				}
				assertState(reopened)
			})
		}
	}
}
