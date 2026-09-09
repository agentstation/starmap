package reconciler

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestDescriptionPresenceSelectsKnownBeforeUnknown(t *testing.T) {
	for _, test := range []struct {
		name                  string
		upstream, local, want catalogs.ValuePresence
		text, wantText        string
	}{
		{name: "unknown alone", upstream: catalogs.ValueUnknown, want: catalogs.ValueUnknown},
		{name: "missing alone", want: catalogs.ValueMissing},
		{name: "unknown with known fallback", upstream: catalogs.ValueUnknown, local: catalogs.ValueKnown, text: "fallback", want: catalogs.ValueKnown, wantText: "fallback"},
		{name: "unknown with empty fallback", upstream: catalogs.ValueUnknown, local: catalogs.ValueKnown, want: catalogs.ValueKnown},
		{name: "missing with unknown fallback", local: catalogs.ValueUnknown, want: catalogs.ValueUnknown},
		{name: "empty leads unknown", upstream: catalogs.ValueKnown, local: catalogs.ValueUnknown, want: catalogs.ValueKnown},
		{name: "unknown leads unknown", upstream: catalogs.ValueUnknown, local: catalogs.ValueUnknown, want: catalogs.ValueUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := func(presence catalogs.ValuePresence, value string) *catalogs.Model {
				m := &catalogs.Model{ID: "model", Name: "Model"}
				switch presence {
				case catalogs.ValueKnown:
					m.SetDescription(value)
				case catalogs.ValueUnknown:
					m.SetDescriptionUnknown()
				}
				return m
			}
			policies := authority.New()
			merger := newMerger(policies, NewAuthorityStrategy(policies), nil)
			models, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
				sources.ModelsDevHTTPID: {model(test.upstream, "")},
				sources.LocalCatalogID:  {model(test.local, test.text)},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(models) != 1 {
				t.Fatalf("models=%d, want one", len(models))
			}
			if value, presence := models[0].DescriptionValue(); value != test.wantText || presence != test.want {
				t.Errorf("description=%q/%v, want %q/%v", value, presence, test.wantText, test.want)
			}
		})
	}
}

func TestMissingLocalDescriptionPreservesBaselineWithoutNewReceipt(t *testing.T) {
	for _, present := range []bool{false, true} {
		name := "missing baseline"
		if present {
			name = "known baseline"
		}
		t.Run(name, func(t *testing.T) {
			policies := authority.New()
			merger := newMerger(policies, NewAuthorityStrategy(policies), nil)
			policy, found := policies.Find(evidence.ResourceTypeModel, "Description")
			if !found {
				t.Fatal("description policy is required")
			}
			model := &catalogs.Model{ID: "model", Name: "Model"}
			if present {
				model.SetDescription("baseline")
			}
			history := make(map[string]provenance.Field)
			merger.applyModelPolicy(modelIdentity{providerID: "provider", modelID: "model"}, model, policy, map[sources.ID]*catalogs.Model{
				sources.LocalCatalogID: {ID: "model", Name: "Model"},
			}, &history)
			value, state := model.DescriptionValue()
			if present {
				if value != "baseline" || state != catalogs.ValueKnown {
					t.Errorf("baseline description=%q/%v", value, state)
				}
			} else if state != catalogs.ValueMissing {
				t.Errorf("missing baseline gained description state %v", state)
			}
			if len(history) != 0 {
				t.Errorf("omitted description created evidence: %+v", history)
			}
		})
	}
}

func TestDescriptionProjectionRequiresPermittedOriginalReceipt(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		name := "known"
		if unknown {
			name = "unknown"
		}
		t.Run(name, func(t *testing.T) {
			model := catalogs.Model{ID: "shared", Name: "Shared"}
			if unknown {
				model.SetDescriptionUnknown()
			} else {
				model.SetDescription("original")
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
						return entry.Field != "Description" || (permitted && entry.ObservationID == original.ID)
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
					value, presence := provider.Models["shared"].DescriptionValue()
					want, wantText := catalogs.ValueMissing, ""
					if permitted {
						if unknown {
							want = catalogs.ValueUnknown
						} else {
							want, wantText = catalogs.ValueKnown, "original"
						}
					}
					if value != wantText || presence != want {
						t.Errorf("description=%q/%v, want %q/%v", value, presence, wantText, want)
					}
					entries := result.Catalog.Provenance().FindModelField("provider-a", "shared", "Description")
					if permitted {
						if len(entries) != 1 || entries[0].ObservationID != original.ID {
							t.Errorf("description receipt=%+v, want original", entries)
						}
					} else {
						for _, entry := range entries {
							if entry.Source != "" || entry.ObservationID != "" || entry.Value != nil {
								t.Errorf("refused description retains a source claim: %+v", entry)
							}
						}
					}
				})
			}
		})
	}
}
