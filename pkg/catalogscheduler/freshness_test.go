package catalogscheduler

import (
	"context"
	stderrors "errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	starmaperrors "github.com/agentstation/starmap/pkg/errors"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestSchedulerFreshnessSLADrivesReadyDegradedAndCriticalAlerts(t *testing.T) {
	monitor := newTestFreshnessMonitor(t)
	now := time.Date(2026, time.July, 10, 21, 0, 0, 0, time.UTC)

	report := freshnessReport(t, monitor, now)
	if report.Ready || !report.Degraded || len(report.Alerts) != 2 ||
		freshnessBySource(t, report, catalogmeta.ProvidersID).State != FreshnessStateMissing ||
		freshnessAlertBySource(t, report, catalogmeta.ProvidersID).Severity != AlertSeverityCritical ||
		freshnessAlertBySource(t, report, catalogmeta.ModelsDevHTTPID).Severity != AlertSeverityWarning {
		t.Fatalf("missing-source report = %#v", report)
	}

	if err := monitor.Record([]catalogs.SourceObservationLink{
		testFreshnessObservation(catalogmeta.ProvidersID, "provider-fresh", now.Add(-30*time.Minute), catalogmeta.ObservationStatusSucceeded, catalogmeta.ObservationCompletenessComplete),
		testFreshnessObservation(catalogmeta.ModelsDevHTTPID, "models-degraded", now.Add(-10*time.Minute), catalogmeta.ObservationStatusDegraded, catalogmeta.ObservationCompletenessPartial),
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	report = freshnessReport(t, monitor, now)
	if !report.Ready || !report.Degraded || len(report.Alerts) != 1 ||
		freshnessBySource(t, report, catalogmeta.ProvidersID).State != FreshnessStateFresh ||
		freshnessBySource(t, report, catalogmeta.ModelsDevHTTPID).State != FreshnessStateDegraded ||
		freshnessAlertBySource(t, report, catalogmeta.ModelsDevHTTPID).Code != FreshnessAlertSourceDegraded {
		t.Fatalf("fresh/degraded report = %#v", report)
	}

	report = freshnessReport(t, monitor, now.Add(time.Hour))
	provider := freshnessBySource(t, report, catalogmeta.ProvidersID)
	if !report.Ready || provider.State != FreshnessStateDegraded ||
		freshnessAlertBySource(t, report, catalogmeta.ProvidersID).Severity != AlertSeverityWarning {
		t.Fatalf("warning report = %#v", report)
	}

	report = freshnessReport(t, monitor, now.Add(3*time.Hour))
	provider = freshnessBySource(t, report, catalogmeta.ProvidersID)
	if report.Ready || provider.State != FreshnessStateUnready ||
		freshnessAlertBySource(t, report, catalogmeta.ProvidersID).Severity != AlertSeverityCritical {
		t.Fatalf("critical report = %#v", report)
	}
}

func TestSchedulerFreshnessFutureAndOutOfOrderObservationsFailClosedWithoutRegression(t *testing.T) {
	monitor := newTestFreshnessMonitor(t)
	now := time.Date(2026, time.July, 10, 22, 0, 0, 0, time.UTC)
	future := testFreshnessObservation(catalogmeta.ProvidersID, "future", now.Add(time.Minute), catalogmeta.ObservationStatusSucceeded, catalogmeta.ObservationCompletenessComplete)
	if err := monitor.Record([]catalogs.SourceObservationLink{future}); err != nil {
		t.Fatalf("Record future: %v", err)
	}
	report := freshnessReport(t, monitor, now)
	if report.Ready || freshnessAlertBySource(t, report, catalogmeta.ProvidersID).Code != FreshnessAlertSourceFuture {
		t.Fatalf("future report = %#v", report)
	}

	older := testFreshnessObservation(catalogmeta.ProvidersID, "older", now.Add(-time.Hour), catalogmeta.ObservationStatusSucceeded, catalogmeta.ObservationCompletenessComplete)
	if err := monitor.Record([]catalogs.SourceObservationLink{older}); err != nil {
		t.Fatalf("Record older: %v", err)
	}
	if got := freshnessBySource(t, freshnessReport(t, monitor, now), catalogmeta.ProvidersID).ObservationID; got != future.ObservationID {
		t.Fatalf("out-of-order completion regressed latest observation to %q", got)
	}

	conflict := future
	conflict.ObservationID = "same-time-different-identity"
	modelsCandidate := testFreshnessObservation(catalogmeta.ModelsDevHTTPID, "must-not-partially-commit", now, catalogmeta.ObservationStatusSucceeded, catalogmeta.ObservationCompletenessComplete)
	if err := monitor.Record([]catalogs.SourceObservationLink{modelsCandidate, conflict}); !stderrors.Is(err, starmaperrors.ErrConflict) {
		t.Fatalf("same-time identity conflict = %v", err)
	}
	if got := freshnessBySource(t, freshnessReport(t, monitor, now), catalogmeta.ModelsDevHTTPID).ObservationID; got != "" {
		t.Fatalf("conflicting batch partially committed observation %q", got)
	}
}

type freshnessSyncer struct {
	result *pkgsync.Result
}

func (s freshnessSyncer) Sync(context.Context, ...pkgsync.Option) (*pkgsync.Result, error) {
	return s.result, nil
}

func TestSchedulerFreshnessRecordsNoChangeSyncObservation(t *testing.T) {
	monitor := newTestFreshnessMonitor(t)
	now := time.Date(2026, time.July, 10, 23, 0, 0, 0, time.UTC)
	result := &pkgsync.Result{SourceObservations: []catalogs.SourceObservationLink{
		testFreshnessObservation(catalogmeta.ProvidersID, "no-change-provider", now, catalogmeta.ObservationStatusSucceeded, catalogmeta.ObservationCompletenessComplete),
		testFreshnessObservation(catalogmeta.ModelsDevHTTPID, "no-change-modelsdev", now, catalogmeta.ObservationStatusSucceeded, catalogmeta.ObservationCompletenessComplete),
	}}
	runner, err := NewRunner(freshnessSyncer{result: result}, NewMemoryLease(), LeaseRequest{
		Key: DefaultLeaseKey, Owner: "freshness-replica", TTL: DefaultLeaseTTL,
	}, WithFreshnessMonitor(monitor))
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	run, err := runner.RunOnce(context.Background())
	if err != nil || run.Status != RunStatusSucceeded || run.Sync.GenerationID != "" {
		t.Fatalf("no-change RunOnce = %#v/%v", run, err)
	}
	report := freshnessReport(t, monitor, now)
	if !report.Ready || report.Degraded || len(report.Alerts) != 0 {
		t.Fatalf("no-change freshness report = %#v", report)
	}
}

func TestSchedulerFreshnessConcurrentRecordAndReport(t *testing.T) {
	monitor := newTestFreshnessMonitor(t)
	now := time.Date(2026, time.July, 10, 23, 30, 0, 0, time.UTC)
	var wait sync.WaitGroup
	for index := 0; index < 20; index++ {
		wait.Add(2)
		go func(index int) {
			defer wait.Done()
			observation := testFreshnessObservation(
				catalogmeta.ProvidersID, "provider-concurrent-"+time.Duration(index).String(),
				now.Add(time.Duration(index)*time.Nanosecond), catalogmeta.ObservationStatusSucceeded, catalogmeta.ObservationCompletenessComplete,
			)
			if err := monitor.Record([]catalogs.SourceObservationLink{observation}); err != nil && !stderrors.Is(err, starmaperrors.ErrConflict) {
				t.Errorf("Record: %v", err)
			}
		}(index)
		go func() {
			defer wait.Done()
			if _, err := monitor.Report(now.Add(time.Hour)); err != nil {
				t.Errorf("Report: %v", err)
			}
		}()
	}
	wait.Wait()
}

func TestSchedulerFreshnessRecoversNoChangeObservationsFromDurableRunLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "freshness-ledger.db")
	db := openTestSQLite(t, path)
	ledger, err := NewSQLRunLedger(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQLRunLedger: %v", err)
	}
	monitor := newTestFreshnessMonitor(t)
	now := time.Date(2026, time.July, 11, 0, 0, 0, 0, time.UTC)
	syncResult := &pkgsync.Result{SourceObservations: []catalogs.SourceObservationLink{
		testFreshnessObservation(catalogmeta.ProvidersID, "durable-provider", now, catalogmeta.ObservationStatusSucceeded, catalogmeta.ObservationCompletenessComplete),
		testFreshnessObservation(catalogmeta.ModelsDevHTTPID, "durable-modelsdev", now, catalogmeta.ObservationStatusSucceeded, catalogmeta.ObservationCompletenessComplete),
	}}
	runner, err := NewRunner(freshnessSyncer{result: syncResult}, NewMemoryLease(), LeaseRequest{
		Key: DefaultLeaseKey, Owner: "durable-freshness-replica", TTL: DefaultLeaseTTL,
	},
		WithRunLedger(ledger, fixedGenerationReader("generation-base")),
		WithFreshnessMonitor(monitor),
	)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	runner.newID = func() (string, error) { return "durable-freshness-run", nil }
	result, err := runner.RunOnce(context.Background())
	if err != nil || result.Status != RunStatusSucceeded {
		t.Fatalf("RunOnce = %#v/%v", result, err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	db = openTestSQLite(t, path)
	reopened, err := NewSQLRunLedger(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQLRunLedger reopened: %v", err)
	}
	record, err := reopened.Get(context.Background(), result.RunID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(record.SourceObservations) != 2 {
		t.Fatalf("persisted source observations = %#v", record.SourceObservations)
	}
	recovered := newTestFreshnessMonitor(t)
	if err := recovered.RecordRuns([]RunRecord{record}); err != nil {
		t.Fatalf("RecordRuns: %v", err)
	}
	report := freshnessReport(t, recovered, now.Add(time.Minute))
	if !report.Ready || report.Degraded || len(report.Alerts) != 0 {
		t.Fatalf("recovered report = %#v", report)
	}
}

func newTestFreshnessMonitor(t *testing.T) *FreshnessMonitor {
	t.Helper()
	monitor, err := NewFreshnessMonitor(FreshnessPolicy{Sources: []SourceFreshnessSLA{
		{Source: catalogmeta.ProvidersID, DegradedAfter: time.Hour, UnreadyAfter: 2 * time.Hour, Required: true},
		{Source: catalogmeta.ModelsDevHTTPID, DegradedAfter: 24 * time.Hour, UnreadyAfter: 48 * time.Hour},
	}})
	if err != nil {
		t.Fatalf("NewFreshnessMonitor: %v", err)
	}
	return monitor
}

func testFreshnessObservation(source catalogmeta.SourceID, id string, observedAt time.Time, status catalogmeta.ObservationStatus, completeness catalogmeta.ObservationCompleteness) catalogs.SourceObservationLink {
	checksum := catalogs.DescribeCatalogPayload([]byte("freshness-evidence")).Checksum
	return catalogs.SourceObservationLink{
		Source: source, ObservationID: id, ObservedAt: observedAt.UTC(),
		Revision:     catalogmeta.ObservationRevision{Kind: catalogmeta.ObservationRevisionKindContentDigest, Value: checksum},
		Completeness: completeness, Status: status, EvidenceChecksum: checksum,
	}
}

func freshnessReport(t *testing.T, monitor *FreshnessMonitor, at time.Time) FreshnessReport {
	t.Helper()
	report, err := monitor.Report(at)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	return report
}

func freshnessBySource(t *testing.T, report FreshnessReport, source catalogmeta.SourceID) SourceFreshness {
	t.Helper()
	for _, state := range report.Sources {
		if state.Source == source {
			return state
		}
	}
	t.Fatalf("source %q not found in %#v", source, report)
	return SourceFreshness{}
}

func freshnessAlertBySource(t *testing.T, report FreshnessReport, source catalogmeta.SourceID) FreshnessAlert {
	t.Helper()
	for _, alert := range report.Alerts {
		if alert.Source == source {
			return alert
		}
	}
	t.Fatalf("source alert %q not found in %#v", source, report)
	return FreshnessAlert{}
}
