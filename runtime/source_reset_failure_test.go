package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestSourceResetFailurePreservesAcceptedHistory(t *testing.T) {
	for _, name := range []string{"local", "embedded", "release", "unknown", "binding", "overlap", "duplicate", "wrong-source", "degraded", "canceled", "callback-error"} {
		t.Run(name, func(t *testing.T) {
			connected, _, at := providerResetRuntime(t, storage.NewMemory())
			original := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 200, at).Catalog, at, nil)
			if _, err := connected.PublishObservations(t.Context(), original); err != nil {
				t.Fatal(err)
			}
			before, history := connected.State(), connected.layers.manual
			replacement := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 0, at).Catalog, at.Add(time.Minute), nil)
			resets := []ObservationReset{{SourceID: sources.ModelsDevHTTPID}}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var callbackErr error
			switch name {
			case "local":
				resets[0].SourceID = sources.LocalCatalogID
			case "embedded":
				resets[0].SourceID = sources.EmbeddedCatalogID
			case "release":
				resets[0].SourceID = sources.ReleaseArtifactID
			case "unknown":
				resets[0].SourceID = "unknown"
			case "binding":
				resets[0].BindingID, resets[0].BindingRevision = "binding", "1"
			case "overlap":
				resets = append(resets, ObservationReset{SourceID: sources.ModelsDevHTTPID, ProviderID: "provider"})
			case "duplicate":
				resets = append(resets, resets[0])
			case "wrong-source":
				resets[0].SourceID = sources.ModelsDevGitID
			case "degraded":
				var err error
				replacement, err = sources.NewObservation(sources.ModelsDevHTTPID, replacement.Catalog, sources.ObservationMetadata{ObservedAt: at.Add(time.Minute), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded, Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "provider/invalid", Message: "fixture rejected record"}}})
				if err != nil {
					t.Fatal(err)
				}
			case "canceled":
				cancel()
			case "callback-error":
				callbackErr = fmt.Errorf("fixture acquisition failed")
			}
			_, err := connected.UpdateObservations(ctx, func(context.Context, ObservationInputs) ([]sources.Observation, error) {
				return []sources.Observation{replacement}, callbackErr
			}, resets...)
			if err == nil {
				t.Fatal("invalid reset succeeded")
			}
			if connected.State().GenerationID != before.GenerationID || connected.State().Sequence != before.Sequence || connected.layers.manual != history {
				t.Fatal("failed reset changed accepted history")
			}
			if pending, err := connected.store.loadInputPublication(); err != nil || pending != nil {
				t.Fatal("failed preparation changed the journal")
			}
		})
	}
}
