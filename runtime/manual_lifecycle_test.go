package runtime

import (
	"context"
	stderrors "errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestManualConcurrentCallsRetainEveryDistinctBatch(t *testing.T) {
	connected, options := manualTestRuntime(t, storage.NewMemory())
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	const count = 8
	observations := make([]sources.Observation, count)
	for index := range observations {
		observations[index] = manualTestObservation(t, fmt.Sprintf("model-%d", index), at.Add(time.Duration(index)*time.Minute), false)
	}
	start := make(chan struct{})
	results := make(chan error, count)
	var group sync.WaitGroup
	for _, observation := range observations {
		group.Go(func() {
			<-start
			_, err := connected.PublishObservations(t.Context(), observation)
			results <- err
		})
	}
	close(start)
	group.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(manualBatches(connected.layers.manual)) != count {
		t.Fatal("concurrent manual calls lost a distinct batch")
	}
	before := connected.State()
	provider, err := before.Catalog.Provider("manual-provider")
	if err != nil || len(provider.Models) != count {
		t.Fatalf("concurrent publication lost models: %v", err)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != before.GenerationID || reopened.State().PayloadChecksum != before.PayloadChecksum {
		t.Fatal("restart changed the concurrent manual catalog")
	}
}

func TestManualPublicationRejectsInvalidAndCanceledInputs(t *testing.T) {
	for _, name := range []string{"empty", "duplicate", "invalid-receipt", "canceled", "closed"} {
		t.Run(name, func(t *testing.T) {
			connected, _ := manualTestRuntime(t, storage.NewMemory())
			before := connected.State()
			observation := manualTestObservation(t, "rejected", time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC), false)
			input := []sources.Observation{observation}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch name {
			case "empty":
				input = nil
			case "duplicate":
				input = append(input, observation)
			case "invalid-receipt":
				input[0].ID = "forged"
			case "canceled":
				cancel()
			case "closed":
				if err := connected.Close(); err != nil {
					t.Fatal(err)
				}
			}
			_, err := connected.PublishObservations(ctx, input...)
			if err == nil {
				t.Fatal("invalid manual publication succeeded")
			}
			if name == "canceled" && !stderrors.Is(err, context.Canceled) {
				t.Fatalf("canceled publication = %v", err)
			}
			if name == "closed" && !errors.IsConflict(err) {
				t.Fatalf("closed publication = %v", err)
			}
			if connected.State().GenerationID != before.GenerationID || connected.layers.manual != nil {
				t.Fatal("rejected manual inputs changed active state")
			}
		})
	}
}
