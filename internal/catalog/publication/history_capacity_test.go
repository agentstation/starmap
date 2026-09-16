package publication

import (
	"strconv"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestPublicationStableInputsHaveBoundedReplayHistory(t *testing.T) {
	for _, name := range []string{"providers", "models_dev_http", "unresolved_metadata"} {
		t.Run(name, func(t *testing.T) {
			sourceID := sources.ModelsDevHTTPID
			if name == "providers" {
				sourceID = sources.ProvidersID
			}
			profile, run := admissionFixture(t)
			profile.Scopes[0].Scope.Source = sourceID
			if sourceID != sources.ProvidersID {
				profile.Scopes[0].Scope.Binding = nil
			}
			empty, err := catalogs.NewEmpty().Build()
			if err != nil {
				t.Fatal(err)
			}
			state, err := NewState(publicationBaselineFixture(t, empty), "stable-publisher")
			if err != nil {
				t.Fatal(err)
			}
			start := run.StartedAt
			for i := range 8 {
				at := start.Add(time.Duration(i) * 4 * time.Hour)
				observation := admissionObservation(t, profile.Scopes[0].Scope, at)
				if name == "unresolved_metadata" {
					observation = unresolvedPublicationObservation(t, at)
				}
				run = Run{StartedAt: at, CompletedAt: at, Attempts: []Attempt{{Scope: profile.Scopes[0].Scope, Outcome: Succeeded, Observation: &observation}}}
				prepared, err := preparePublication(t.Context(), state, profile, run, "stable-"+strconv.Itoa(i))
				if err != nil {
					t.Fatal(err)
				}
				if name == "unresolved_metadata" {
					receipt, err := artifact.DecodePublicationReceipt(prepared.Receipt.Data)
					if err != nil {
						t.Fatal(err)
					}
					if len(receipt.Reviews) != 1 || receipt.Reviews[0].SourceObservationID != observation.ID {
						t.Fatal("current review lost the latest original observation")
					}
				}
				checkpoint, err := EncodeState(prepared.Next)
				if err != nil {
					t.Fatal(err)
				}
				state, err = RestoreState(t.Context(), checkpoint.Data, checkpoint.Checksum)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("run=%d observations=%d checkpoint_bytes=%d reused_artifact=%t", i+1, len(state.history), len(checkpoint.Data), prepared.ReusedArtifact)
			}
			// These stable inputs need only their initial and current evidence.
			if len(state.history) > 2 {
				t.Fatalf("eight stable runs retained %d full observations; want at most two", len(state.history))
			}
		})
	}
}

func unresolvedPublicationObservation(t *testing.T, at time.Time) sources.Observation {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider", Models: map[string]*catalogs.Model{"unresolved": {ID: "unresolved", Name: "Unresolved"}}}); err != nil {
		t.Fatal(err)
	}
	catalog, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ModelsDevHTTPID, catalog, sources.ObservationMetadata{
		ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	return observation
}
