package reconciler

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/sources"
)

func TestAuthoredCorpusSelectsNewestSourceObservation(t *testing.T) {
	baseline := authoredOnlyCatalog(t, "baseline", "Baseline")
	older := authoredOnlyCatalog(t, "older", "Older")
	newer := authoredOnlyCatalog(t, "newer", "Newer")
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	for _, reverse := range []bool{false, true} {
		inputs := []sources.Observation{
			{SourceID: sources.ReleaseArtifactID, Catalog: baseline},
			{SourceID: sources.ReleaseArtifactID, Catalog: older, ObservedAt: at},
			{SourceID: sources.ReleaseArtifactID, Catalog: newer, ObservedAt: at.Add(time.Minute)},
		}
		if reverse {
			inputs[1], inputs[2] = inputs[2], inputs[1]
		}
		collector := newCollector(inputs, "")
		if collector.authoredBootstrap() != newer {
			t.Fatal("an older or synthetic baseline hid the newest reviewed definitions")
		}
	}
}
