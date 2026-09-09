package runtime

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/goccy/go-yaml"
)

func TestProviderGenerationRecoveryPreservesNestedPresence(t *testing.T) {
	for _, input := range []struct{ name, value string }{
		{"bounds", `{"temperature":{"min":null,"default":0},"top_k":{"max":42,"default":null},"max_tokens":null,"top_logprobs":0}`},
		{"controls", `{"top_p":null,"top_k":null,"n":{"min":0,"max":null},"max_output_tokens":null}`},
	} {
		for _, format := range []string{"json", "yaml"} {
			t.Run(input.name+"/"+format, func(t *testing.T) {
				at := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
				seed := manualProviderObservation(t, 0, at).Catalog
				payload, err := catalogs.EncodeCatalogPayload(seed)
				if err != nil {
					t.Fatal(err)
				}
				upstream := newStubSource("generation-recovery")
				upstream.replies = []SourceRead{testSourceRead(t, "generation-baseline", payload, at), testSourceRead(t, "generation-refresh", payload, at.Add(time.Hour))}
				store, err := storage.NewFilesystem(privateRuntimeDirectory(t))
				if err != nil {
					t.Fatal(err)
				}
				options := []Option{WithSource(upstream), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(store))}
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
					model.UnsetRecord(catalogs.ModelRecordGeneration)
					if partial {
						model.Generation = &catalogs.ModelGeneration{}
						if format == "json" {
							err = json.Unmarshal([]byte(input.value), model.Generation)
						} else {
							err = yaml.Unmarshal([]byte(input.value), model.Generation)
						}
						if err != nil {
							t.Fatal(err)
						}
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
				for _, observation := range []sources.Observation{prior, current} {
					if _, err := connected.PublishObservations(t.Context(), observation); err != nil {
						t.Fatal(err)
					}
				}
				policy, found := authority.New().Find("model", "Generation")
				if !found {
					t.Fatal("generation authority is missing")
				}
				var want any
				if err := json.Unmarshal([]byte(input.value), &want); err != nil {
					t.Fatal(err)
				}
				assertState := func(active *Runtime) {
					t.Helper()
					catalog := active.State().Catalog
					provider, err := catalog.Provider("provider")
					if err != nil {
						t.Fatal(err)
					}
					raw, present := compositeJSONValue(t, provider.Models["model"], []string{"generation"})
					if !present {
						t.Fatal("generation record disappeared")
					}
					var got any
					if err := json.Unmarshal(raw, &got); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("generation=%s, want %s", raw, input.value)
					}
					receipts := catalog.Provenance().FindModelField("provider", "model", policy.Evidence())
					if len(receipts) != 1 || receipts[0].ObservationID != prior.ID {
						t.Errorf("generation lost original receipt: %+v", receipts)
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
					if !oldFound || !newFound || !generation.Manifest.Degraded {
						t.Errorf("source receipts old=%t new=%t degraded=%t", oldFound, newFound, generation.Manifest.Degraded)
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
