package reconciler

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestOptionalRecordUnknownSurvivesSelection(t *testing.T) {
	for _, record := range catalogs.PublishedModelRecords() {
		t.Run(string(record), func(t *testing.T) {
			policies := authority.New()
			engine := newMerger(policies, NewAuthorityStrategy(policies), nil)
			unknown := &catalogs.Model{ID: "model", Name: "Model"}
			if !unknown.SetRecordUnknown(record) {
				t.Fatal("unsupported record")
			}
			models, _, err := engine.Models(map[sources.ID][]*catalogs.Model{
				sources.ModelsDevHTTPID: {unknown},
				sources.LocalCatalogID:  {{ID: "model", Name: "Model"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(models) != 1 {
				t.Fatalf("models=%d, want one", len(models))
			}
			if state := models[0].RecordPresence(record); state != catalogs.ValueUnknown {
				t.Fatalf("record presence=%v, want unknown", state)
			}
		})
	}
}

func TestOptionalRecordProjectionRequiresOriginalReceipt(t *testing.T) {
	for _, record := range catalogs.PublishedModelRecords() {
		t.Run(string(record), func(t *testing.T) {
			model := catalogs.Model{ID: "shared", Name: "Shared"}
			model.SetRecordUnknown(record)
			at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
			original := sourceIdentityObservation(t, sources.ProvidersID, sourceIdentityCatalog(t, "", model), at)
			generated := sourceIdentityReconcile(t, sources.ProvidersID, original)
			policy, found := authority.New().Find("model", recordPolicyPaths[record])
			if !found {
				t.Fatal("record policy is required")
			}
			for _, permitted := range []bool{false, true} {
				name := "refused"
				if permitted {
					name = "permitted"
				}
				t.Run(name, func(t *testing.T) {
					baseline := snapshotForTest(t, generated.Catalog)
					local := sourceIdentityObservation(t, sources.LocalCatalogID, baseline, at.Add(time.Minute))
					engine, err := New(WithBaseline(baseline), WithProjectedEvidencePolicy(func(_ catalogs.ProviderID, entry provenance.Entry) bool {
						return entry.Field != policy.Evidence() || (permitted && entry.ObservationID == original.ID)
					}))
					if err != nil {
						t.Fatal(err)
					}
					result, err := engine.Sources(t.Context(), sources.LocalCatalogID, []sources.Observation{local})
					if err != nil {
						t.Fatal(err)
					}
					provider, err := result.Catalog.Provider("provider-a")
					if err != nil {
						t.Fatal(err)
					}
					current := provider.Models["shared"]
					want := catalogs.ValueMissing
					if permitted {
						want = catalogs.ValueUnknown
					}
					if state := current.RecordPresence(record); state != want {
						t.Errorf("record presence=%v, want %v", state, want)
					}
					entries := result.Catalog.Provenance().FindModelField("provider-a", "shared", policy.Evidence())
					if permitted {
						if len(entries) != 1 || entries[0].ObservationID != original.ID {
							t.Errorf("record receipt=%+v, want original", entries)
						}
					} else {
						for _, entry := range entries {
							if entry.Source != "" || entry.ObservationID != "" || entry.Value != nil {
								t.Errorf("refused record retains receipt: %+v", entry)
							}
						}
					}
				})
			}
		})
	}
}

func TestOptionalRecordKnownFallbackAndMissingState(t *testing.T) {
	for _, record := range catalogs.PublishedModelRecords() {
		t.Run(string(record), func(t *testing.T) {
			model := func(state catalogs.ValuePresence) *catalogs.Model {
				result := &catalogs.Model{ID: "model", Name: "Model"}
				if record == catalogs.ModelRecordReasoning || record == catalogs.ModelRecordReasoningTokens {
					result.Features = &catalogs.ModelFeatures{Reasoning: true}
				}
				switch state {
				case catalogs.ValueUnknown:
					result.SetRecordUnknown(record)
				case catalogs.ValueKnown:
					value := `{}`
					switch record {
					case catalogs.ModelRecordDeprecatedAt, catalogs.ModelRecordRetiresAt:
						value = `"2026-09-01T00:00:00Z"`
					case catalogs.ModelRecordPricing:
						value = `{"currency":"USD","tokens":{"input":{"per_1m_tokens":1}}}`
					}
					payload := fmt.Sprintf(`{%q:%s}`, record, value)
					// Decode the record separately because Model.UnmarshalJSON clears reused state.
					var decoded catalogs.Model
					if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
						t.Fatal(err)
					}
					policies := authority.New()
					engine := newMerger(policies, NewAuthorityStrategy(policies), nil)
					engine.setModelFieldValue(result, recordPolicyPaths[record], engine.modelFieldValue(&decoded, recordPolicyPaths[record]))
				}
				return result
			}
			for _, test := range []struct {
				name                  string
				upstream, local, want catalogs.ValuePresence
			}{
				{"unknown uses known fallback", catalogs.ValueUnknown, catalogs.ValueKnown, catalogs.ValueKnown},
				{"known leads unknown", catalogs.ValueKnown, catalogs.ValueUnknown, catalogs.ValueKnown},
				{"missing stays missing", catalogs.ValueMissing, catalogs.ValueMissing, catalogs.ValueMissing},
			} {
				t.Run(test.name, func(t *testing.T) {
					policies := authority.New()
					engine := newMerger(policies, NewAuthorityStrategy(policies), nil)
					models, _, err := engine.Models(map[sources.ID][]*catalogs.Model{
						sources.ModelsDevHTTPID: {model(test.upstream)}, sources.LocalCatalogID: {model(test.local)},
					})
					if err != nil {
						t.Fatal(err)
					}
					if len(models) != 1 {
						t.Fatalf("models=%d", len(models))
					}
					if state := models[0].RecordPresence(record); state != test.want {
						t.Errorf("record presence=%v, want %v", state, test.want)
					}
				})
			}
		})
	}
}
