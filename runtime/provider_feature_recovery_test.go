package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestProviderFeatureRecoveryPreservesContributingReceipts(t *testing.T) {
	for _, scenario := range []struct {
		name              string
		prior             catalogs.ValuePresence
		priorValue        bool
		current           catalogs.ValuePresence
		currentValue      bool
		modalities        bool
		replaceModalities bool
		omitRecord        bool
		keepPrior         bool
	}{
		{name: "missing", prior: catalogs.ValueKnown, priorValue: true, keepPrior: true},
		{name: "missing-record", prior: catalogs.ValueKnown, priorValue: true, keepPrior: true, omitRecord: true},
		{name: "explicit-false", prior: catalogs.ValueKnown, priorValue: true, current: catalogs.ValueKnown},
		{name: "unknown", prior: catalogs.ValueKnown, priorValue: true, current: catalogs.ValueUnknown, keepPrior: true},
		{name: "retained-false", prior: catalogs.ValueKnown, keepPrior: true},
		{name: "restored", prior: catalogs.ValueKnown, priorValue: true, current: catalogs.ValueKnown, currentValue: true},
		{name: "retained-unknown", prior: catalogs.ValueUnknown, keepPrior: true},
		{name: "replaced-unknown", prior: catalogs.ValueUnknown, current: catalogs.ValueUnknown},
		{name: "retained-modalities", modalities: true, keepPrior: true},
		{name: "replaced-modalities", modalities: true, replaceModalities: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
			baseline := manualProviderObservation(t, 100, at).Catalog
			payload, err := catalogs.EncodeCatalogPayload(baseline)
			if err != nil {
				t.Fatal(err)
			}
			source := newStubSource("feature-recovery")
			source.replies = []SourceRead{
				testSourceRead(t, "feature-baseline", payload, at),
				testSourceRead(t, "feature-refresh", payload, at.Add(time.Hour)),
			}
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
				when := at.Add(time.Minute)
				state, supported := scenario.prior, scenario.priorValue
				if !partial {
					when = at.Add(2 * time.Minute)
					state, supported = scenario.current, scenario.currentValue
				}
				seed := manualProviderObservation(t, 200, when)
				builder, err := catalogs.NewBuilderFrom(seed.Catalog)
				if err != nil {
					t.Fatal(err)
				}
				provider, err := builder.Provider("provider")
				if err != nil {
					t.Fatal(err)
				}
				features := &catalogs.ModelFeatures{}
				if state == catalogs.ValueKnown {
					features.SetSupport(catalogs.ModelFeatureTools, supported)
				}
				if state == catalogs.ValueUnknown {
					features.SetSupportUnknown(catalogs.ModelFeatureTools)
				}
				if !partial {
					features.SetSupport(catalogs.ModelFeatureStreaming, true)
				}
				if scenario.modalities {
					if partial || scenario.replaceModalities {
						features.Modalities.Input = []catalogs.ModelModality{catalogs.ModelModalityImage}
						features.Modalities.Output = []catalogs.ModelModality{catalogs.ModelModalityAudio}
					}
					if !partial {
						features.Modalities.Input = append(features.Modalities.Input, catalogs.ModelModalityText)
						features.Modalities.Output = append(features.Modalities.Output, catalogs.ModelModalityText)
					}
				}
				provider.Models["model"].Features = features
				if !partial && scenario.omitRecord {
					provider.Models["model"].Features = nil
				}
				if err := builder.SetProvider(provider); err != nil {
					t.Fatal(err)
				}
				catalog, err := catalogs.NewObservationCatalog(builder)
				if err != nil {
					t.Fatal(err)
				}
				metadata := sources.ObservationMetadata{ObservedAt: when, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded}
				if partial {
					metadata.Completeness, metadata.Status = sources.ObservationCompletenessPartial, sources.ObservationStatusDegraded
					metadata.Issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "provider/invalid", Message: "A source record is invalid."}}
				}
				result, err := sources.NewObservation(sources.ProvidersID, catalog, metadata)
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			prior, current := observe(true), observe(false)
			for _, observation := range []sources.Observation{prior, current} {
				if _, err := connected.PublishObservations(t.Context(), observation); err != nil {
					t.Fatal(err)
				}
			}
			assertEvidence := func(connected *Runtime) {
				t.Helper()
				generation, err := store.Current(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				oldFound, newFound := false, false
				for _, link := range generation.Manifest.SourceObservations {
					oldFound = oldFound || link.ObservationID == prior.ID
					newFound = newFound || link.ObservationID == current.ID
				}
				if oldFound != scenario.keepPrior || !newFound || generation.Manifest.Degraded != scenario.keepPrior {
					t.Errorf("receipts old=%t new=%t degraded=%t, want old/degraded=%t", oldFound, newFound, generation.Manifest.Degraded, scenario.keepPrior)
				}
				catalog := connected.State().Catalog
				provider, err := catalog.Provider("provider")
				if err != nil {
					t.Fatal(err)
				}
				model := provider.Models["model"]
				if model == nil || model.ModelRef == "" {
					t.Fatal("recovery lost linked membership")
				}
				wantID := current.ID
				if scenario.keepPrior {
					wantID = prior.ID
				}
				fields := []string{"Features.tools"}
				if scenario.modalities {
					fields = []string{"Features.modalities.input.image", "Features.modalities.output.audio"}
				}
				for _, field := range fields {
					entries := catalog.Provenance().FindModelField("provider", "model", field)
					if len(entries) != 1 || entries[0].ObservationID != wantID {
						t.Errorf("%s lost its contributing receipt: got %+v, want %s", field, entries, wantID)
					}
				}
				if !scenario.modalities {
					wantValue, wantState := scenario.currentValue, scenario.current
					if scenario.keepPrior {
						wantValue, wantState = scenario.priorValue, scenario.prior
					}
					value, state := model.Features.Support(catalogs.ModelFeatureTools)
					if value != wantValue || state != wantState {
						t.Errorf("tools=%t/%v, want %t/%v", value, state, wantValue, wantState)
					}
				} else if len(model.Features.Modalities.Input) != 2 || len(model.Features.Modalities.Output) != 2 {
					t.Error("recovery lost documented modalities")
				}
				streaming, presence := model.Features.Support(catalogs.ModelFeatureStreaming)
				entries := catalog.Provenance().FindModelField("provider", "model", "Features.streaming")
				if scenario.omitRecord {
					if streaming || presence != catalogs.ValueMissing || len(entries) != 0 {
						t.Error("recovery invented a streaming claim")
					}
				} else {
					if !streaming || presence != catalogs.ValueKnown {
						t.Error("recovery lost the new streaming capability")
					}
					if len(entries) != 1 || entries[0].ObservationID != current.ID {
						t.Error("streaming lost its current receipt")
					}
				}
				if len(manualBatches(connected.layers.manual)) != 2 {
					t.Error("recovery changed durable observation history")
				}
			}
			assertEvidence(connected)
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			reopened := openTestRuntime(t, options...)
			assertEvidence(reopened)
			if _, err := reopened.RefreshSource(t.Context()); err != nil {
				t.Fatal(err)
			}
			assertEvidence(reopened)
		})
	}
}
