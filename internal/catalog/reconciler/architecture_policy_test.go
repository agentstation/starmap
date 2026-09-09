package reconciler

import (
	"encoding/json"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
	"testing"
	"time"
)

func TestArchitectureQuantizedRequiresPermittedOriginalReceipt(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		name := "known"
		if unknown {
			name = "unknown"
		}
		t.Run(name, func(t *testing.T) {
			model := catalogs.Model{ID: "shared", Name: "Shared"}
			if unknown {
				model.Metadata = &catalogs.ModelMetadata{Architecture: &catalogs.ModelArchitecture{}}
				if err := json.Unmarshal([]byte(`{"quantized":null}`), model.Metadata.Architecture); err != nil {
					t.Fatal(err)
				}
			} else {
				model.Metadata = &catalogs.ModelMetadata{Architecture: &catalogs.ModelArchitecture{}}
				if err := json.Unmarshal([]byte(`{"quantized":false}`), model.Metadata.Architecture); err != nil {
					t.Fatal(err)
				}
			}
			at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
			original := sourceIdentityObservation(t, sources.ProvidersID, sourceIdentityCatalog(t, "", model), at)
			generated := sourceIdentityReconcile(t, sources.ProvidersID, original)
			for _, permitted := range []bool{false, true} {
				name := "refused"
				if permitted {
					name = "permitted"
				}
				t.Run(name, func(t *testing.T) {
					baseline := snapshotForTest(t, generated.Catalog)
					local := sourceIdentityObservation(t, sources.LocalCatalogID, baseline, at.Add(time.Minute))
					engine, err := New(WithBaseline(baseline), WithProjectedEvidencePolicy(func(_ catalogs.ProviderID, entry provenance.Entry) bool {
						return entry.Field != "metadata.architecture.quantized" || (permitted && entry.ObservationID == original.ID)
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
					value, presence := architectureProbeClaim(t, provider.Models["shared"], "quantized")
					want, wantText := catalogs.ValueMissing, false
					if permitted {
						if unknown {
							want = catalogs.ValueUnknown
						} else {
							want, wantText = catalogs.ValueKnown, false
						}
					}
					if value != wantText || presence != want {
						t.Errorf("architecture=%v/%v, want %v/%v", value, presence, wantText, want)
					}
					entries := result.Catalog.Provenance().FindModelField("provider-a", "shared", "metadata.architecture.quantized")
					if permitted {
						if len(entries) != 1 || entries[0].ObservationID != original.ID {
							t.Errorf("architecture receipt=%+v, want original", entries)
						}
					} else {
						for _, entry := range entries {
							if entry.Source != "" || entry.ObservationID != "" || entry.Value != nil {
								t.Errorf("refused architecture retains a source claim: %+v", entry)
							}
						}
					}
				})
			}
		})
	}
}

func TestArchitectureFineTunedRequiresPermittedOriginalReceipt(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		name := "known"
		if unknown {
			name = "unknown"
		}
		t.Run(name, func(t *testing.T) {
			model := catalogs.Model{ID: "shared", Name: "Shared"}
			if unknown {
				model.Metadata = &catalogs.ModelMetadata{Architecture: &catalogs.ModelArchitecture{}}
				if err := json.Unmarshal([]byte(`{"fine_tuned":null}`), model.Metadata.Architecture); err != nil {
					t.Fatal(err)
				}
			} else {
				model.Metadata = &catalogs.ModelMetadata{Architecture: &catalogs.ModelArchitecture{}}
				if err := json.Unmarshal([]byte(`{"fine_tuned":false}`), model.Metadata.Architecture); err != nil {
					t.Fatal(err)
				}
			}
			at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
			original := sourceIdentityObservation(t, sources.ProvidersID, sourceIdentityCatalog(t, "", model), at)
			generated := sourceIdentityReconcile(t, sources.ProvidersID, original)
			for _, permitted := range []bool{false, true} {
				name := "refused"
				if permitted {
					name = "permitted"
				}
				t.Run(name, func(t *testing.T) {
					baseline := snapshotForTest(t, generated.Catalog)
					local := sourceIdentityObservation(t, sources.LocalCatalogID, baseline, at.Add(time.Minute))
					engine, err := New(WithBaseline(baseline), WithProjectedEvidencePolicy(func(_ catalogs.ProviderID, entry provenance.Entry) bool {
						return entry.Field != "metadata.architecture.fine_tuned" || (permitted && entry.ObservationID == original.ID)
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
					value, presence := architectureProbeClaim(t, provider.Models["shared"], "fine_tuned")
					want, wantText := catalogs.ValueMissing, false
					if permitted {
						if unknown {
							want = catalogs.ValueUnknown
						} else {
							want, wantText = catalogs.ValueKnown, false
						}
					}
					if value != wantText || presence != want {
						t.Errorf("architecture=%v/%v, want %v/%v", value, presence, wantText, want)
					}
					entries := result.Catalog.Provenance().FindModelField("provider-a", "shared", "metadata.architecture.fine_tuned")
					if permitted {
						if len(entries) != 1 || entries[0].ObservationID != original.ID {
							t.Errorf("architecture receipt=%+v, want original", entries)
						}
					} else {
						for _, entry := range entries {
							if entry.Source != "" || entry.ObservationID != "" || entry.Value != nil {
								t.Errorf("refused architecture retains a source claim: %+v", entry)
							}
						}
					}
				})
			}
		})
	}
}

func architectureProbeClaim(t *testing.T, model *catalogs.Model, field string) (bool, catalogs.ValuePresence) {
	t.Helper()
	if model.Metadata == nil || model.Metadata.Architecture == nil {
		return false, catalogs.ValueMissing
	}
	data, err := json.Marshal(model.Metadata.Architecture)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatal(err)
	}
	raw, ok := values[field]
	if !ok {
		return false, catalogs.ValueMissing
	}
	if string(raw) == "null" {
		return false, catalogs.ValueUnknown
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value, catalogs.ValueKnown
}
