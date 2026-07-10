package catalogscheduler

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	runTimeLayout = "2006-01-02T15:04:05.000000000Z07:00"

	createRunsTable = `CREATE TABLE IF NOT EXISTS catalog_sync_runs (
run_id TEXT PRIMARY KEY,
trigger TEXT NOT NULL,
lease_owner TEXT NOT NULL,
base_generation_id TEXT NOT NULL,
started_at TEXT NOT NULL,
completed_at TEXT NOT NULL,
duration_ns INTEGER NOT NULL,
status TEXT NOT NULL,
published_generation_id TEXT NOT NULL,
sync_run_id TEXT NOT NULL,
failure_type TEXT NOT NULL
)`
	createAttemptsTable = `CREATE TABLE IF NOT EXISTS catalog_sync_attempts (
run_id TEXT NOT NULL,
attempt_number INTEGER NOT NULL,
started_at TEXT NOT NULL,
completed_at TEXT NOT NULL,
duration_ns INTEGER NOT NULL,
status TEXT NOT NULL,
retry_class TEXT NOT NULL,
retry_delay_ns INTEGER NOT NULL,
failure_type TEXT NOT NULL,
PRIMARY KEY (run_id, attempt_number)
)`
	createRunsQueryIndex = `CREATE INDEX IF NOT EXISTS catalog_sync_runs_query
ON catalog_sync_runs (started_at DESC, trigger, status)`
	createRunSourcesTable = `CREATE TABLE IF NOT EXISTS catalog_sync_run_sources (
run_id TEXT NOT NULL,
source TEXT NOT NULL,
observation_id TEXT NOT NULL,
observed_at TEXT NOT NULL,
revision_kind TEXT NOT NULL,
revision_value TEXT NOT NULL,
revision_input_name TEXT NOT NULL,
revision_input_checksum TEXT NOT NULL,
completeness TEXT NOT NULL,
status TEXT NOT NULL,
evidence_checksum TEXT NOT NULL,
PRIMARY KEY (run_id, source)
)`
)

// SQLRunLedger persists queryable run and attempt records through database/sql.
// The baseline statements use SQLite-compatible question-mark bind parameters.
type SQLRunLedger struct {
	db *sql.DB
}

// NewSQLRunLedger initializes the durable run-ledger schema.
func NewSQLRunLedger(ctx context.Context, db *sql.DB) (*SQLRunLedger, error) {
	if db == nil {
		return nil, &errors.ConfigError{Component: "catalog scheduler run ledger", Message: "SQL database is required"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, statement := range []string{createRunsTable, createAttemptsTable, createRunSourcesTable, createRunsQueryIndex} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return nil, errors.WrapResource("initialize", "catalog scheduler run ledger schema", "", err)
		}
	}
	return &SQLRunLedger{db: db}, nil
}

// Begin inserts an idempotent running trigger.
func (l *SQLRunLedger) Begin(ctx context.Context, record RunRecord) error {
	if err := record.ValidateBegin(); err != nil {
		return err
	}
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.WrapResource("begin", "catalog scheduler run transaction", record.ID, err)
	}
	defer func() { _ = tx.Rollback() }()
	existing, found, err := sqlRun(ctx, tx, record.ID)
	if err != nil {
		return err
	}
	if found {
		if sameRunRecord(existing, record) {
			return tx.Commit()
		}
		return runIdentityConflict(record.ID)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO catalog_sync_runs (
run_id, trigger, lease_owner, base_generation_id, started_at, completed_at,
duration_ns, status, published_generation_id, sync_run_id, failure_type
) VALUES (?, ?, ?, ?, ?, '', 0, ?, '', '', '')`,
		record.ID, record.Trigger, record.LeaseOwner, record.BaseGenerationID,
		formatRunTime(record.StartedAt), record.Status,
	); err != nil {
		return errors.WrapResource("insert", "catalog scheduler run", record.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return errors.WrapResource("commit", "catalog scheduler run", record.ID, err)
	}
	return nil
}

// RecordAttempt appends an idempotent contiguous attempt to a running record.
func (l *SQLRunLedger) RecordAttempt(ctx context.Context, runID string, attempt AttemptRecord) error {
	if err := attempt.Validate(); err != nil {
		return err
	}
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.WrapResource("begin", "catalog scheduler attempt transaction", runID, err)
	}
	defer func() { _ = tx.Rollback() }()
	record, found, err := sqlRun(ctx, tx, runID)
	if err != nil {
		return err
	}
	if !found {
		return runNotFound(runID)
	}
	for _, existing := range record.Attempts {
		if existing.Number == attempt.Number {
			if reflect.DeepEqual(existing, attempt) {
				return tx.Commit()
			}
			return attemptIdentityConflict(runID, attempt.Number)
		}
	}
	if record.Status != RunStatusRunning || attempt.Number != len(record.Attempts)+1 {
		return &errors.ConflictError{Resource: "catalog scheduler attempt", Expected: strconv.Itoa(len(record.Attempts) + 1), Actual: strconv.Itoa(attempt.Number)}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO catalog_sync_attempts (
run_id, attempt_number, started_at, completed_at, duration_ns, status,
retry_class, retry_delay_ns, failure_type
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, attempt.Number, formatRunTime(attempt.StartedAt), formatRunTime(attempt.CompletedAt),
		int64(attempt.Duration), attempt.Status, attempt.RetryClass, int64(attempt.RetryDelay), attempt.FailureType,
	); err != nil {
		return errors.WrapResource("insert", "catalog scheduler attempt", runID, err)
	}
	if err := tx.Commit(); err != nil {
		return errors.WrapResource("commit", "catalog scheduler attempt", runID, err)
	}
	return nil
}

// Complete atomically makes a running record terminal.
func (l *SQLRunLedger) Complete(ctx context.Context, record RunRecord) error {
	if err := record.ValidateComplete(); err != nil {
		return err
	}
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.WrapResource("begin", "catalog scheduler completion transaction", record.ID, err)
	}
	defer func() { _ = tx.Rollback() }()
	existing, found, err := sqlRun(ctx, tx, record.ID)
	if err != nil {
		return err
	}
	if !found {
		return runNotFound(record.ID)
	}
	if existing.Status != RunStatusRunning {
		if sameRunRecord(existing, record) {
			return tx.Commit()
		}
		return runIdentityConflict(record.ID)
	}
	if !sameRunBeginning(existing, record) || !sameAttemptRecords(existing.Attempts, record.Attempts) {
		return runIdentityConflict(record.ID)
	}
	for _, observation := range record.SourceObservations {
		if _, err := tx.ExecContext(ctx, `INSERT INTO catalog_sync_run_sources (
run_id, source, observation_id, observed_at, revision_kind, revision_value,
revision_input_name, revision_input_checksum, completeness, status, evidence_checksum
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			record.ID, observation.Source, observation.ObservationID, formatRunTime(observation.ObservedAt),
			observation.Revision.Kind, observation.Revision.Value, observation.Revision.InputName,
			observation.Revision.InputChecksum, observation.Completeness, observation.Status,
			observation.EvidenceChecksum,
		); err != nil {
			return errors.WrapResource("insert", "catalog scheduler run source observation", record.ID, err)
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE catalog_sync_runs SET
completed_at = ?, duration_ns = ?, status = ?, published_generation_id = ?,
sync_run_id = ?, failure_type = ? WHERE run_id = ? AND status = ?`,
		formatRunTime(record.CompletedAt), int64(record.Duration), record.Status,
		record.PublishedGenerationID, record.SyncRunID, record.FailureType,
		record.ID, RunStatusRunning,
	)
	if err != nil {
		return errors.WrapResource("update", "catalog scheduler run", record.ID, err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return errors.WrapResource("verify", "catalog scheduler run update", record.ID, err)
	}
	if updated != 1 {
		return &errors.ConflictError{Resource: "catalog scheduler run completion", Expected: "1 running record", Actual: fmt.Sprintf("%d records", updated)}
	}
	if err := tx.Commit(); err != nil {
		return errors.WrapResource("commit", "catalog scheduler run", record.ID, err)
	}
	return nil
}

// Get returns one complete run and its ordered attempts.
func (l *SQLRunLedger) Get(ctx context.Context, runID string) (RunRecord, error) {
	record, found, err := sqlRun(ctx, l.db, runID)
	if err != nil {
		return RunRecord{}, err
	}
	if !found {
		return RunRecord{}, runNotFound(runID)
	}
	return record, nil
}

// List returns newest-first complete records matching query.
func (l *SQLRunLedger) List(ctx context.Context, query RunQuery) ([]RunRecord, error) {
	limit, err := validateRunQuery(query)
	if err != nil {
		return nil, err
	}
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 3)
	if query.Trigger != "" {
		clauses = append(clauses, "trigger = ?")
		args = append(args, query.Trigger)
	}
	if query.Status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, query.Status)
	}
	statement := selectRuns
	if len(clauses) > 0 {
		statement += " WHERE " + strings.Join(clauses, " AND ")
	}
	statement += " ORDER BY started_at DESC, run_id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := l.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, errors.WrapResource("list", "catalog scheduler runs", "", err)
	}
	records := make([]RunRecord, 0)
	for rows.Next() {
		record, scanErr := scanRun(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Close(); err != nil {
		return nil, errors.WrapResource("close", "catalog scheduler run rows", "", err)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.WrapResource("list", "catalog scheduler runs", "", err)
	}
	for index := range records {
		attempts, err := sqlAttempts(ctx, l.db, records[index].ID)
		if err != nil {
			return nil, err
		}
		records[index].Attempts = attempts
		observations, err := sqlRunSources(ctx, l.db, records[index].ID)
		if err != nil {
			return nil, err
		}
		records[index].SourceObservations = observations
	}
	return records, nil
}

const selectRuns = `SELECT run_id, trigger, lease_owner, base_generation_id,
started_at, completed_at, duration_ns, status, published_generation_id,
sync_run_id, failure_type FROM catalog_sync_runs`

type sqlQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type sqlScanner interface {
	Scan(...any) error
}

func sqlRun(ctx context.Context, queryer sqlQueryer, runID string) (RunRecord, bool, error) {
	record, err := scanRun(queryer.QueryRowContext(ctx, selectRuns+" WHERE run_id = ?", runID))
	if err == sql.ErrNoRows {
		return RunRecord{}, false, nil
	}
	if err != nil {
		return RunRecord{}, false, err
	}
	attempts, err := sqlAttempts(ctx, queryer, runID)
	if err != nil {
		return RunRecord{}, false, err
	}
	record.Attempts = attempts
	observations, err := sqlRunSources(ctx, queryer, runID)
	if err != nil {
		return RunRecord{}, false, err
	}
	record.SourceObservations = observations
	return record, true, nil
}

func scanRun(scanner sqlScanner) (RunRecord, error) {
	var record RunRecord
	var startedAt, completedAt string
	var duration int64
	if err := scanner.Scan(
		&record.ID, &record.Trigger, &record.LeaseOwner, &record.BaseGenerationID,
		&startedAt, &completedAt, &duration, &record.Status,
		&record.PublishedGenerationID, &record.SyncRunID, &record.FailureType,
	); err != nil {
		return RunRecord{}, err
	}
	parsed, err := parseRunTime("started_at", startedAt)
	if err != nil {
		return RunRecord{}, err
	}
	record.StartedAt = parsed
	if completedAt != "" {
		parsed, err = parseRunTime("completed_at", completedAt)
		if err != nil {
			return RunRecord{}, err
		}
		record.CompletedAt = parsed
	}
	record.Duration = time.Duration(duration)
	return record, nil
}

func sqlAttempts(ctx context.Context, queryer sqlQueryer, runID string) ([]AttemptRecord, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT attempt_number, started_at, completed_at,
duration_ns, status, retry_class, retry_delay_ns, failure_type
FROM catalog_sync_attempts WHERE run_id = ? ORDER BY attempt_number`, runID)
	if err != nil {
		return nil, errors.WrapResource("list", "catalog scheduler attempts", runID, err)
	}
	defer func() { _ = rows.Close() }()
	var attempts []AttemptRecord
	for rows.Next() {
		var attempt AttemptRecord
		var startedAt, completedAt string
		var duration, retryDelay int64
		if err := rows.Scan(&attempt.Number, &startedAt, &completedAt, &duration, &attempt.Status,
			&attempt.RetryClass, &retryDelay, &attempt.FailureType); err != nil {
			return nil, errors.WrapResource("scan", "catalog scheduler attempt", runID, err)
		}
		attempt.StartedAt, err = parseRunTime("attempt.started_at", startedAt)
		if err != nil {
			return nil, err
		}
		attempt.CompletedAt, err = parseRunTime("attempt.completed_at", completedAt)
		if err != nil {
			return nil, err
		}
		attempt.Duration = time.Duration(duration)
		attempt.RetryDelay = time.Duration(retryDelay)
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.WrapResource("list", "catalog scheduler attempts", runID, err)
	}
	return attempts, nil
}

func sqlRunSources(ctx context.Context, queryer sqlQueryer, runID string) ([]catalogs.SourceObservationLink, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT source, observation_id, observed_at,
revision_kind, revision_value, revision_input_name, revision_input_checksum,
completeness, status, evidence_checksum
FROM catalog_sync_run_sources WHERE run_id = ? ORDER BY source`, runID)
	if err != nil {
		return nil, errors.WrapResource("list", "catalog scheduler run source observations", runID, err)
	}
	defer func() { _ = rows.Close() }()
	var observations []catalogs.SourceObservationLink
	for rows.Next() {
		var observation catalogs.SourceObservationLink
		var observedAt string
		if err := rows.Scan(
			&observation.Source, &observation.ObservationID, &observedAt,
			&observation.Revision.Kind, &observation.Revision.Value,
			&observation.Revision.InputName, &observation.Revision.InputChecksum,
			&observation.Completeness, &observation.Status, &observation.EvidenceChecksum,
		); err != nil {
			return nil, errors.WrapResource("scan", "catalog scheduler run source observation", runID, err)
		}
		observation.ObservedAt, err = parseRunTime("source_observation.observed_at", observedAt)
		if err != nil {
			return nil, err
		}
		observations = append(observations, observation)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.WrapResource("list", "catalog scheduler run source observations", runID, err)
	}
	return observations, nil
}

func formatRunTime(value time.Time) string {
	return value.UTC().Format(runTimeLayout)
}

func parseRunTime(field, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, &errors.ValidationError{Field: "catalog_scheduler." + field, Value: value, Message: fmt.Sprintf("is not RFC3339: %v", err)}
	}
	return parsed, nil
}
