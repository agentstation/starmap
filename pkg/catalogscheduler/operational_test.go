package catalogscheduler

import (
	"context"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/types"
)

func TestOperationalStateComposesGenerationFreshnessLastSyncAndScheduler(t *testing.T) {
	now := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
	ledger := NewMemoryRunLedger()
	attempt := AttemptRecord{
		Number: 1, StartedAt: now.Add(-2 * time.Minute), CompletedAt: now.Add(-time.Minute),
		Duration: time.Minute, Status: AttemptStatusSucceeded,
	}
	record := RunRecord{
		ID: "run-1", Trigger: TriggerScheduled, LeaseOwner: "replica-a",
		BaseGenerationID: "generation-old", StartedAt: attempt.StartedAt,
		CompletedAt: attempt.CompletedAt, Duration: time.Minute, Status: RunStatusSucceeded,
		Attempts: []AttemptRecord{attempt}, PublishedGenerationID: "generation-new", SyncRunID: "sync-1",
	}
	begin := record
	begin.CompletedAt = time.Time{}
	begin.Duration = 0
	begin.Status = RunStatusRunning
	begin.Attempts = nil
	begin.PublishedGenerationID = ""
	begin.SyncRunID = ""
	if err := ledger.Begin(context.Background(), begin); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := ledger.RecordAttempt(context.Background(), record.ID, attempt); err != nil {
		t.Fatalf("RecordAttempt: %v", err)
	}
	if err := ledger.Complete(context.Background(), record); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	monitor := newTestFreshnessMonitor(t)
	if err := monitor.Record([]catalogs.SourceObservationLink{
		testFreshnessObservation(types.ProvidersID, "provider-fresh", now.Add(-time.Minute), types.ObservationStatusSucceeded, types.ObservationCompletenessComplete),
		testFreshnessObservation(types.ModelsDevHTTPID, "models-degraded", now.Add(-time.Minute), types.ObservationStatusDegraded, types.ObservationCompletenessPartial),
	}); err != nil {
		t.Fatalf("Record freshness: %v", err)
	}
	operations, err := NewOperations(
		WithOperationsRunLedger(ledger),
		WithOperationsFreshness(monitor),
		WithOperationsClock(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("NewOperations: %v", err)
	}
	state, err := operations.State(context.Background(), CatalogIdentity{GenerationID: "generation-new", Sequence: 7})
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if state.Catalog.GenerationID != "generation-new" || state.Catalog.Sequence != 7 ||
		state.Freshness == nil || !state.Freshness.Ready || !state.Freshness.Degraded ||
		state.LastSync == nil || state.LastSync.ID != "run-1" || state.LastSync.SyncRunID != "sync-1" ||
		!state.Scheduler.Configured {
		t.Fatalf("operational state = %#v", state)
	}
	if len(state.DegradedSources) != 1 || state.DegradedSources[0] != types.ModelsDevHTTPID {
		t.Fatalf("degraded sources = %#v", state.DegradedSources)
	}
}

func TestOperationalStateExplicitlyReportsUnconfiguredScheduler(t *testing.T) {
	operations, err := NewOperations(WithOperationsClock(func() time.Time { return time.Unix(1, 0) }))
	if err != nil {
		t.Fatalf("NewOperations: %v", err)
	}
	state, err := operations.State(context.Background(), CatalogIdentity{GenerationID: "embedded", Sequence: 1})
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if state.Scheduler.Configured || state.Freshness != nil || state.LastSync != nil || state.DegradedSources == nil {
		t.Fatalf("unconfigured state = %#v", state)
	}
}
