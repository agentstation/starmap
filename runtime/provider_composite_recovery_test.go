package runtime

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestProviderCompositeRecoveryPreservesContributions(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name, prior, current, field string
		path                        []string
		member                      string
	}{
		{name: "empty-architecture", prior: `{"metadata":{"architecture":{}}}`, current: `{"metadata":{"tags":["chat"]}}`, field: "metadata.architecture.present", path: []string{"metadata", "architecture"}},
		{name: "empty-mode-provider", prior: `{"modes":{"fast":{"provider":{}}}}`, current: `{"modes":{"fast":{"pricing":{"currency":"USD","tokens":{"input":{"per_1m_tokens":0}}}}}}`, field: `modes["fast"].provider.present`, path: []string{"modes", "fast", "provider"}},
		{name: "empty-extension", prior: `{"extensions":{"retained":{}}}`, current: `{"extensions":{"current":{}}}`, field: `extensions["retained"].present`, path: []string{"extensions", "retained"}},
		{name: "open-true", prior: `{"metadata":{"open_weights":true}}`, current: `{"metadata":{"tags":["chat"]}}`, field: "metadata.open_weights", path: []string{"metadata", "open_weights"}},
		{name: "open-false", prior: `{"metadata":{"open_weights":false}}`, current: `{"metadata":{"tags":["chat"]}}`, field: "metadata.open_weights", path: []string{"metadata", "open_weights"}},
		{name: "open-unknown", prior: `{"metadata":{"open_weights":null}}`, current: `{"metadata":{"tags":["chat"]}}`, field: "metadata.open_weights", path: []string{"metadata", "open_weights"}},
		{name: "release-date", prior: `{"metadata":{"release_date":"2025-01-01T00:00:00Z"}}`, current: `{"metadata":{"tags":["chat"]}}`, field: "metadata.release_date", path: []string{"metadata", "release_date"}},
		{name: "knowledge-cutoff", prior: `{"metadata":{"knowledge_cutoff":"2025-01-01T00:00:00Z"}}`, current: `{"metadata":{"tags":["chat"]}}`, field: "metadata.knowledge_cutoff", path: []string{"metadata", "knowledge_cutoff"}},
		{name: "architecture", prior: `{"metadata":{"architecture":{"parameter_count":"7B","fine_tuned":true}}}`, current: `{"metadata":{"tags":["chat"]}}`, field: "metadata.architecture.parameter_count", path: []string{"metadata", "architecture", "parameter_count"}},
		{name: "architecture-flag", prior: `{"metadata":{"architecture":{"fine_tuned":true}}}`, current: `{"metadata":{"tags":["chat"]}}`, field: "metadata.architecture.fine_tuned", path: []string{"metadata", "architecture", "fine_tuned"}},
		{name: "architecture-base", prior: `{"metadata":{"architecture":{"base_model":"author/base"}}}`, current: `{"metadata":{"tags":["chat"]}}`, field: "metadata.architecture.base_model", path: []string{"metadata", "architecture", "base_model"}},
		{name: "tag", prior: `{"metadata":{"tags":["retained"]}}`, current: `{"metadata":{"tags":["chat"]}}`, field: `metadata.tags["retained"]`, path: []string{"metadata", "tags"}, member: "retained"},
		{name: "mode-header", prior: `{"modes":{"fast":{"provider":{"headers":{"prior":"present"}}}}}`, current: `{"modes":{"fast":{"provider":{"headers":{"current":"present"}}}}}`, field: `modes["fast"].provider.headers["prior"]`, path: []string{"modes", "fast", "provider", "headers", "prior"}},
		{name: "mode-key", prior: `{"modes":{"fast.a":{"provider":{"headers":{"b.c":"prior"}}}}}`, current: `{"modes":{"fast":{"provider":{"headers":{"a.b.c":"current"}}}}}`, field: `modes["fast.a"].provider.headers["b.c"]`, path: []string{"modes", "fast.a", "provider", "headers", "b.c"}},
		{name: "mode-null", prior: `{"modes":{"fast":{"provider":{"body":{"prior":null}}}}}`, current: `{"modes":{"fast":{"provider":{"body":{"current":false}}}}}`, field: `modes["fast"].provider.body["prior"]`, path: []string{"modes", "fast", "provider", "body", "prior"}},
		{name: "mode-paid-price", prior: `{"modes":{"fast":{"pricing":{"currency":"USD","tokens":{"input":{"per_1m_tokens":2.5},"output":{"per_1m_tokens":7}}}}}}`, current: `{"modes":{"fast":{"provider":{"headers":{"current":"present"}}}}}`, field: `modes["fast"].pricing`, path: []string{"modes", "fast", "pricing"}},
		{name: "mode-price", prior: `{"modes":{"fast":{"pricing":{"currency":"USD","tokens":{"input":{"per_1m_tokens":0}}}}}}`, current: `{"modes":{"fast":{"provider":{"headers":{"current":"present"}}}}}`, field: `modes["fast"].pricing`, path: []string{"modes", "fast", "pricing"}},
		{name: "extension-false", prior: `{"extensions":{"models.dev":{"fields":{"prior":false}}}}`, current: `{"extensions":{"models.dev":{"fields":{"current":true}}}}`, field: `extensions["models.dev"].fields["prior"]`, path: []string{"extensions", "models.dev", "fields", "prior"}},
		{name: "extension-null", prior: `{"extensions":{"models.dev":{"fields":{"prior":null}}}}`, current: `{"extensions":{"models.dev":{"fields":{"current":true}}}}`, field: `extensions["models.dev"].fields["prior"]`, path: []string{"extensions", "models.dev", "fields", "prior"}},
		{name: "extension-nested", prior: `{"extensions":{"models.dev":{"fields":{"prior":{"max":9007199254740993,"items":[false,null,"a"]}}}}}`, current: `{"extensions":{"models.dev":{"fields":{"current":true}}}}`, field: `extensions["models.dev"].fields["prior"]`, path: []string{"extensions", "models.dev", "fields", "prior"}},
		{name: "extension-key", prior: `{"extensions":{"models.dev":{"fields":{"a.b":"prior"}}}}`, current: `{"extensions":{"models":{"fields":{"dev.a.b":"current"}}}}`, field: `extensions["models.dev"].fields["a.b"]`, path: []string{"extensions", "models.dev", "fields", "a.b"}},
	} {
		for _, replace := range []bool{false, true} {
			mode := "retained"
			if replace {
				mode = "replaced"
			}
			t.Run(scenario.name+"/"+mode, func(t *testing.T) {
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
					encoded := scenario.current
					if partial || replace {
						encoded = scenario.prior
					}
					var fields catalogs.Model
					if err := json.Unmarshal([]byte(encoded), &fields); err != nil {
						t.Fatal(err)
					}
					builder, err := catalogs.NewBuilderFrom(seed)
					if err != nil {
						t.Fatal(err)
					}
					provider, err := builder.Provider("provider")
					if err != nil {
						t.Fatal(err)
					}
					model := provider.Models["model"]
					model.Metadata, model.Modes, model.Extensions = fields.Metadata, fields.Modes, fields.Extensions
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
				expected, present := compositeJSONValue(t, originalProvider.Models["model"], scenario.path)
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
					actual, present := compositeJSONValue(t, model, scenario.path)
					if scenario.member == "" {
						if !present || !bytes.Equal(actual, expected) {
							t.Errorf("accepted value=%s/%t, want %s", actual, present, expected)
						}
					} else {
						var members []string
						if err := json.Unmarshal(actual, &members); err != nil {
							t.Fatal(err)
						}
						if !slices.Contains(members, scenario.member) {
							t.Error("accepted tag disappeared")
						}
					}
					wantID := prior.ID
					if replace {
						wantID = current.ID
					}
					entries := catalog.Provenance().FindModelField("provider", "model", scenario.field)
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

func compositeJSONValue(t *testing.T, model *catalogs.Model, path []string) (json.RawMessage, bool) {
	t.Helper()
	raw, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range path {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			t.Fatal(err)
		}
		var present bool
		raw, present = object[key]
		if !present {
			return nil, false
		}
	}
	return raw, true
}
