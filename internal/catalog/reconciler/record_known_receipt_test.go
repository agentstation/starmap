package reconciler

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestKnownRecordProjectionRequiresOriginalReceipt(t *testing.T) {
	for _, record := range catalogs.PublishedModelRecords() {
		t.Run(string(record), func(t *testing.T) {
			model := knownRecordModel(t, record)
			at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
			original := sourceIdentityObservation(t, sources.ProvidersID, sourceIdentityCatalog(t, "", model), at)
			generated := sourceIdentityReconcile(t, sources.ProvidersID, original)
			policy, found := authority.New().Find(evidence.ResourceTypeModel, recordPolicyPaths[record])
			if !found {
				t.Fatal("record policy is required")
			}
			owns := func(field string) bool {
				return field == policy.Evidence() || strings.HasPrefix(field, policy.Evidence()+".")
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
						return !owns(entry.Field) || (permitted && entry.ObservationID == original.ID)
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
						want = catalogs.ValueKnown
					}
					if state := current.RecordPresence(record); state != want {
						t.Errorf("record presence=%v, want %v", state, want)
					}
					hasOriginal := false
					for field, entries := range result.Catalog.Provenance().FindModel("provider-a", "shared") {
						if !owns(field) || len(entries) == 0 {
							continue
						}
						current := entries[0]
						hasOriginal = hasOriginal || current.ObservationID == original.ID
						if !permitted && (current.Source != "" || current.ObservationID != "" || current.Value != nil) {
							t.Errorf("refused record retains current claim: %+v", current)
						}
					}
					if permitted && !hasOriginal {
						t.Error("known record lost its original receipt")
					}
				})
			}
		})
	}
}

func knownRecordModel(t *testing.T, record catalogs.ModelRecord) catalogs.Model {
	t.Helper()
	var value any = map[string]any{}
	switch record {
	case catalogs.ModelRecordDeprecatedAt, catalogs.ModelRecordRetiresAt:
		value = "2026-09-01T00:00:00Z"
	case catalogs.ModelRecordPricing:
		value = map[string]any{"currency": "USD", "tokens": map[string]any{"input": map[string]any{"per_1m_tokens": 1}}}
	}
	payload := map[string]any{"id": "shared", "name": "Shared", string(record): value}
	if record == catalogs.ModelRecordReasoning || record == catalogs.ModelRecordReasoningTokens {
		payload["features"] = map[string]any{"reasoning": true}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var model catalogs.Model
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatal(err)
	}
	return model
}

func TestLineageRecordPresenceCannotRestoreRefusedLeaves(t *testing.T) {
	for _, refused := range []string{"lineage.parent", "lineage.present", "all-leaves"} {
		t.Run(refused, func(t *testing.T) {
			root, parent := "root-model", "parent-model"
			model := catalogs.Model{ID: "shared", Name: "Shared", Lineage: &catalogs.ModelLineage{
				Family: "family", Root: &root, Parent: &parent,
			}}
			at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
			original := sourceIdentityObservation(t, sources.ProvidersID, sourceIdentityCatalog(t, "", model), at)
			generated := sourceIdentityReconcile(t, sources.ProvidersID, original)
			baseline := snapshotForTest(t, generated.Catalog)
			local := sourceIdentityObservation(t, sources.LocalCatalogID, baseline, at.Add(time.Minute))
			engine, err := New(WithBaseline(baseline), WithProjectedEvidencePolicy(func(_ catalogs.ProviderID, entry provenance.Entry) bool {
				if refused == "all-leaves" {
					return entry.Field != "lineage.family" && entry.Field != "lineage.root" && entry.Field != "lineage.parent"
				}
				return entry.Field != refused
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
			if current.RecordPresence(catalogs.ModelRecordLineage) != catalogs.ValueKnown {
				t.Fatal("permitted presence or child evidence lost the lineage container")
			}
			if refused == "lineage.parent" || refused == "all-leaves" {
				if current.Lineage.Parent != nil {
					t.Error("permitted record presence restored a refused parent")
				}
			} else if current.Lineage.Parent == nil || *current.Lineage.Parent != parent {
				t.Error("record presence refusal erased a permitted parent")
			}
			if refused == "all-leaves" {
				if current.Lineage.Root != nil || current.Lineage.Family != "" {
					t.Error("permitted record presence restored refused lineage facts")
				}
			} else if current.Lineage.Root == nil || *current.Lineage.Root != root || current.Lineage.Family != "family" {
				t.Error("leaf refusal erased unrelated permitted lineage facts")
			}
		})
	}
}
