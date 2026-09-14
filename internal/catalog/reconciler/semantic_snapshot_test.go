package reconciler

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestSemanticEvidenceMatchesCanonicalSnapshot(t *testing.T) {
	for _, record := range catalogs.PublishedModelRecords() {
		t.Run(string(record), func(t *testing.T) {
			model := knownRecordModel(t, record)
			merger := newMerger(nil, nil, nil)
			value := merger.modelFieldValue(&model, recordPolicyPaths[record])
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			decoder := json.NewDecoder(bytes.NewReader(encoded))
			decoder.UseNumber()
			var restored any
			if err := decoder.Decode(&restored); err != nil {
				t.Fatal(err)
			}
			if !semanticValueEqual(recordPolicyPaths[record], value, restored) {
				t.Fatalf("typed evidence differs from its canonical snapshot: %s", encoded)
			}
		})
	}
}

func TestKnownControlRecordReceiptSurvivesStorage(t *testing.T) {
	for _, record := range []catalogs.ModelRecord{catalogs.ModelRecordReasoning, catalogs.ModelRecordVerbosity} {
		for _, format := range []string{"payload", "workspace"} {
			t.Run(string(record)+"/"+format, func(t *testing.T) {
				at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
				original := sourceIdentityObservation(t, sources.ProvidersID, sourceIdentityCatalog(t, "", knownRecordModel(t, record)), at)
				generated := sourceIdentityReconcile(t, sources.ProvidersID, original)
				var baseline *catalogs.Catalog
				if format == "workspace" {
					path := filepath.Join(t.TempDir(), "catalog")
					if err := generated.Catalog.SaveTo(path); err != nil {
						t.Fatal(err)
					}
					baseline = sourceIdentityLoad(t, path)
				} else {
					data, err := catalogs.EncodeCatalogPayload(generated.Catalog)
					if err != nil {
						t.Fatal(err)
					}
					baseline, err = catalogs.DecodeCatalogPayload(data)
					if err != nil {
						t.Fatal(err)
					}
				}
				local := sourceIdentityObservation(t, sources.LocalCatalogID, baseline, at.Add(time.Minute))
				for _, permitted := range []bool{false, true} {
					engine, err := New(WithBaseline(baseline), WithProjectedEvidencePolicy(func(_ catalogs.ProviderID, entry provenance.Entry) bool {
						return entry.Field != recordPolicyPaths[record] || (permitted && entry.ObservationID == original.ID)
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
					want := catalogs.ValueMissing
					if permitted {
						want = catalogs.ValueKnown
					}
					if provider.Models["shared"].RecordPresence(record) != want {
						t.Errorf("permitted=%v: restored record does not enforce the original receipt", permitted)
					}
					if permitted {
						assertModelEvidenceSource(t, result.Catalog, recordPolicyPaths[record], sources.ProvidersID, original.ID)
					}
				}
			})
		}
	}
}
