package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestProviderResetFailurePreservesAcceptedHistory(t *testing.T) {
	for _, name := range []string{"empty", "missing-scope", "degraded", "callback-error", "canceled", "duplicate-scope", "partial-binding"} {
		t.Run(name, func(t *testing.T) {
			connected, _, at := providerResetRuntime(t, storage.NewMemory())
			original := manualProviderObservation(t, 200, at)
			if _, err := connected.PublishObservations(t.Context(), original); err != nil {
				t.Fatal(err)
			}
			before := connected.State()
			history := connected.layers.manual
			observations := []sources.Observation{manualProviderObservation(t, 0, at.Add(time.Minute))}
			resets := []ProviderObservationReset{{ProviderID: "provider"}}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var callbackErr error
			switch name {
			case "empty":
				observations = nil
			case "missing-scope":
				resets[0].ProviderID = "missing"
			case "degraded":
				var err error
				observations[0], err = sources.NewObservation(sources.ProvidersID, observations[0].Catalog, sources.ObservationMetadata{ObservedAt: at.Add(time.Minute), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded, Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "provider/invalid", Message: "fixture rejected record"}}})
				if err != nil {
					t.Fatal(err)
				}
			case "callback-error":
				callbackErr = fmt.Errorf("fixture acquisition failed")
			case "canceled":
				cancel()
			case "duplicate-scope":
				resets = append(resets, resets[0])
			case "partial-binding":
				resets[0].BindingID = "binding"
			}
			_, err := connected.UpdateObservations(ctx, func(context.Context, ObservationInputs) ([]sources.Observation, error) {
				return observations, callbackErr
			}, resets...)
			if err == nil {
				t.Fatal("failed replacement accepted a reset")
			}
			after := connected.State()
			if after.GenerationID != before.GenerationID || after.Sequence != before.Sequence || connected.layers.manual != history {
				t.Fatal("failed reset changed accepted state")
			}
			if pending, err := connected.store.loadInputPublication(); err != nil || pending != nil {
				t.Fatal("failed preparation changed the publication journal")
			}
		})
	}
}

func TestProviderResetRejectedCommitPreservesHistory(t *testing.T) {
	store := &retentionRejectingStore{Memory: storage.NewMemory()}
	connected, options, at := providerResetRuntime(t, store)
	original := manualProviderObservation(t, 200, at)
	if _, err := connected.PublishObservations(t.Context(), original); err != nil {
		t.Fatal(err)
	}
	before := connected.State()
	store.reject.Store(true)
	_, err := connected.UpdateObservations(t.Context(), func(context.Context, ObservationInputs) ([]sources.Observation, error) {
		return []sources.Observation{manualProviderObservation(t, 0, at.Add(time.Minute))}, nil
	}, ProviderObservationReset{ProviderID: "provider"})
	if err == nil {
		t.Fatal("rejected reset committed")
	}
	history, err := connected.store.loadManualHistory(t.Context())
	if err != nil || len(history.resets) != 0 || connected.State().GenerationID != before.GenerationID {
		t.Fatal("rejected reset changed retained history")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	store.reject.Store(false)
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != before.GenerationID || len(reopened.layers.manual.resets) != 0 {
		t.Fatal("restart accepted rejected reset")
	}
}
