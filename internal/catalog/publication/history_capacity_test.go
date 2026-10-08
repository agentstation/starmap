package publication

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/sources"
	catalogruntime "github.com/agentstation/starmap/runtime"
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

func TestPublicationRestoresAcceptedHistoryAboveRetainedBound(t *testing.T) {
	// The runtime retained payload bound. An accepted checkpoint can exceed it until compaction.
	const retainedBound = 64 << 20
	profile, run := admissionFixture(t)
	profile.Scopes[0].Scope.Source = sources.ModelsDevHTTPID
	profile.Scopes[0].Scope.Binding = nil
	empty, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	state, err := NewState(publicationBaselineFixture(t, empty), "accepted-publisher")
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for index := 0; total <= retainedBound; index++ {
		at := run.StartedAt.Add(time.Duration(index) * time.Hour)
		observation := sizedUnresolvedPublicationObservation(t, at, strings.Repeat(string(rune('a'+index)), 6<<20))
		payload, err := catalogs.EncodeCatalogPayload(observation.Catalog)
		if err != nil {
			t.Fatal(err)
		}
		total += len(payload)
		state.history = append(state.history, observation)
	}
	candidate, err := catalogruntime.ReplayAcquisition(t.Context(), state.baseline, state.publisherID, nil, state.history)
	if err != nil {
		t.Fatal(err)
	}
	state.current, err = candidate.Generation("accepted", state.history[len(state.history)-1].ObservedAt)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := artifact.Build(state.current)
	if err != nil {
		t.Fatal(err)
	}
	state.bundle = &bundle
	accepted, err := EncodeState(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreState(t.Context(), accepted.Data, accepted.Checksum)
	if err != nil {
		t.Fatalf("accepted history above the retained bound did not restore: %v", err)
	}
	at := run.StartedAt.Add(time.Duration(len(state.history)) * time.Hour)
	observation := sizedUnresolvedPublicationObservation(t, at, "current")
	run = Run{StartedAt: at, CompletedAt: at, Attempts: []Attempt{{Scope: profile.Scopes[0].Scope, Outcome: Succeeded, Observation: &observation}}}
	prepared, err := preparePublication(t.Context(), restored, profile, run, "compacted")
	if err != nil {
		t.Fatal(err)
	}
	retained := 0
	for _, observation := range prepared.Next.history {
		payload, err := catalogs.EncodeCatalogPayload(observation.Catalog)
		if err != nil {
			t.Fatal(err)
		}
		retained += len(payload)
	}
	checkpoint, err := EncodeState(prepared.Next)
	if err != nil {
		t.Fatal(err)
	}
	if retained > retainedBound || len(checkpoint.Data) > retainedBound {
		t.Fatalf("compacted checkpoint retained %d payload bytes in %d checkpoint bytes; want each at most %d", retained, len(checkpoint.Data), retainedBound)
	}
	if _, err := RestoreState(t.Context(), checkpoint.Data, checkpoint.Checksum); err != nil {
		t.Fatal(err)
	}
}

func unresolvedPublicationObservation(t *testing.T, at time.Time) sources.Observation {
	t.Helper()
	return sizedUnresolvedPublicationObservation(t, at, "")
}

// sizedUnresolvedPublicationObservation carries the description in a quarantined offering, so it changes only the payload size.
func sizedUnresolvedPublicationObservation(t *testing.T, at time.Time, description string) sources.Observation {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider", Models: map[string]*catalogs.Model{"unresolved": {ID: "unresolved", Name: "Unresolved", Description: description}}}); err != nil {
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
