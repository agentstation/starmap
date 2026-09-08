package runtime

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestObservationInputsKeepTrustedBaselineSeparateFromLocalFacts(t *testing.T) {
	connected, options := manualTestRuntime(t, storage.NewMemory())
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	first := manualTestObservation(t, "first", at, false)
	if _, err := connected.PublishObservations(t.Context(), first); err != nil {
		t.Fatal(err)
	}
	before := connected.State()
	inputs, err := connected.ObservationInputs(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if inputs.Current.GenerationID != before.GenerationID || inputs.Current.Catalog != before.Catalog {
		t.Fatal("observation preparation did not receive the current immutable state")
	}
	if inputs.Baseline.GenerationID != "manual-baseline-generation" {
		t.Fatal("observation baseline did not retain the selected source identity")
	}
	if _, err := inputs.Baseline.Catalog.Provider("manual-provider"); !errors.IsNotFound(err) {
		t.Fatal("local acquisition facts entered the selected baseline")
	}
	baseline, err := inputs.Baseline.Catalog.Provider("baseline-provider")
	if err != nil || baseline.Models["baseline-model"] == nil {
		t.Fatal("selected baseline lost its reviewed model")
	}
	if connected.State().Sequence != before.Sequence {
		t.Fatal("reading observation inputs published a generation")
	}
	second := manualTestObservation(t, "second", at.Add(time.Minute), false)
	updated, err := connected.UpdateObservations(t.Context(), func(_ context.Context, current ObservationInputs) ([]sources.Observation, error) {
		if current.Current.GenerationID != before.GenerationID || current.Baseline.GenerationID != inputs.Baseline.GenerationID {
			t.Fatal("callback did not receive the selected input pair")
		}
		return []sources.Observation{second}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inputs.Current.Catalog.Provider("manual-provider"); err != nil {
		t.Fatal(err)
	}
	previous, _ := inputs.Current.Catalog.Provider("manual-provider")
	if previous.Models["second"] != nil {
		t.Fatal("publication mutated an earlier input snapshot")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != updated.GenerationID || reopened.State().PayloadChecksum != updated.PayloadChecksum {
		t.Fatal("callback publication did not survive restart")
	}
}

func TestObservationInputsUseEmbeddedBaselineWithoutSelectedSource(t *testing.T) {
	connected := openTestRuntime(t, WithCatalogSource("embedded"))
	inputs, err := connected.ObservationInputs(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	embedded := connected.Client().EmbeddedCatalogState()
	if inputs.Baseline.Catalog != embedded.Catalog || inputs.Baseline.GenerationID != embedded.GenerationID || inputs.Baseline.PayloadChecksum != embedded.PayloadChecksum {
		t.Fatal("preparation did not receive the compiled baseline")
	}
}

func TestObservationUpdatePreservesStateOnEmptyOrFailedPreparation(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "empty"
		if failed {
			name = "failed"
		}
		t.Run(name, func(t *testing.T) {
			connected, _ := manualTestRuntime(t, storage.NewMemory())
			before := connected.State()
			failure := stderrors.New("source preparation failed")
			state, err := connected.UpdateObservations(t.Context(), func(context.Context, ObservationInputs) ([]sources.Observation, error) {
				if failed {
					return nil, failure
				}
				return nil, nil
			})
			if failed && !stderrors.Is(err, failure) {
				t.Fatalf("preparation error = %v", err)
			}
			if !failed && (err != nil || state.GenerationID != before.GenerationID) {
				t.Fatalf("empty preparation = %v", err)
			}
			if connected.State().Sequence != before.Sequence || connected.layers.manual != nil {
				t.Fatal("empty or failed preparation changed accepted state")
			}
			if pending, err := connected.store.loadInputPublication(); err != nil || pending != nil {
				t.Fatalf("preparation changed retention recovery state: %v", err)
			}
		})
	}
}

func TestObservationUpdateRejectsLateCallbackResultAfterClose(t *testing.T) {
	connected, _ := manualTestRuntime(t, storage.NewMemory())
	before := connected.State()
	observation := manualTestObservation(t, "late", time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC), false)
	entered := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := connected.UpdateObservations(t.Context(), func(ctx context.Context, _ ObservationInputs) ([]sources.Observation, error) {
			close(entered)
			<-ctx.Done()
			return []sources.Observation{observation}, nil
		})
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("observation callback did not start")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !stderrors.Is(err, context.Canceled) {
			t.Fatalf("closed callback = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("close did not join the observation callback")
	}
	if connected.State().Sequence != before.Sequence || connected.layers.manual != nil {
		t.Fatal("late callback changed state after close")
	}
}
