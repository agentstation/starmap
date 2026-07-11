package catalogscheduler

import (
	"context"
	"reflect"
	"sort"
	"strconv"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// MemoryRunLedger is the concurrency-safe reference run-ledger adapter.
type MemoryRunLedger struct {
	mu      sync.RWMutex
	records map[string]RunRecord
}

// NewMemoryRunLedger creates an empty reference ledger.
func NewMemoryRunLedger() *MemoryRunLedger {
	return &MemoryRunLedger{records: make(map[string]RunRecord)}
}

// Begin durably models insertion of a running trigger.
func (l *MemoryRunLedger) Begin(ctx context.Context, record RunRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := record.ValidateBegin(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if existing, found := l.records[record.ID]; found {
		if sameRunRecord(existing, record) {
			return nil
		}
		return runIdentityConflict(record.ID)
	}
	l.records[record.ID] = record.Copy()
	return nil
}

// RecordAttempt appends one contiguous attempt to a running record.
func (l *MemoryRunLedger) RecordAttempt(ctx context.Context, runID string, attempt AttemptRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := attempt.Validate(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	record, found := l.records[runID]
	if !found {
		return runNotFound(runID)
	}
	if attempt.Number <= len(record.Attempts) {
		if reflect.DeepEqual(record.Attempts[attempt.Number-1], attempt) {
			return nil
		}
		return attemptIdentityConflict(runID, attempt.Number)
	}
	if record.Status != RunStatusRunning || attempt.Number != len(record.Attempts)+1 {
		return &errors.ConflictError{Resource: "catalog scheduler attempt", Expected: strconv.Itoa(len(record.Attempts) + 1), Actual: strconv.Itoa(attempt.Number)}
	}
	record.Attempts = append(record.Attempts, attempt)
	l.records[runID] = record
	return nil
}

// Complete atomically makes a running record terminal.
func (l *MemoryRunLedger) Complete(ctx context.Context, record RunRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := record.ValidateComplete(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	existing, found := l.records[record.ID]
	if !found {
		return runNotFound(record.ID)
	}
	if existing.Status != RunStatusRunning {
		if sameRunRecord(existing, record) {
			return nil
		}
		return runIdentityConflict(record.ID)
	}
	if !sameRunBeginning(existing, record) || !sameAttemptRecords(existing.Attempts, record.Attempts) {
		return runIdentityConflict(record.ID)
	}
	l.records[record.ID] = record.Copy()
	return nil
}

// Get returns one caller-owned run record.
func (l *MemoryRunLedger) Get(ctx context.Context, runID string) (RunRecord, error) {
	if err := ctx.Err(); err != nil {
		return RunRecord{}, err
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	record, found := l.records[runID]
	if !found {
		return RunRecord{}, runNotFound(runID)
	}
	return record.Copy(), nil
}

// List returns newest-first caller-owned records matching query.
func (l *MemoryRunLedger) List(ctx context.Context, query RunQuery) ([]RunRecord, error) {
	limit, err := validateRunQuery(query)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.mu.RLock()
	records := make([]RunRecord, 0, len(l.records))
	for _, record := range l.records {
		if query.Trigger != "" && record.Trigger != query.Trigger {
			continue
		}
		if query.Status != "" && record.Status != query.Status {
			continue
		}
		records = append(records, record.Copy())
	}
	l.mu.RUnlock()
	sort.Slice(records, func(i, j int) bool {
		if records[i].StartedAt.Equal(records[j].StartedAt) {
			return records[i].ID > records[j].ID
		}
		return records[i].StartedAt.After(records[j].StartedAt)
	})
	if len(records) > limit {
		records = records[:limit]
	}
	return records, nil
}

func validateRunQuery(query RunQuery) (int, error) {
	if query.Trigger != "" {
		if err := query.Trigger.Validate(); err != nil {
			return 0, err
		}
	}
	if query.Status != "" {
		switch query.Status {
		case RunStatusRunning, RunStatusSucceeded, RunStatusFailed, RunStatusSkippedLeaseHeld, RunStatusSkippedInitialRun:
		default:
			return 0, &errors.ValidationError{Field: "catalog_scheduler.run_query.status", Value: query.Status, Message: "is invalid"}
		}
	}
	if query.Limit < 0 || query.Limit > 1000 {
		return 0, &errors.ValidationError{Field: "catalog_scheduler.run_query.limit", Value: query.Limit, Message: "must be between zero and 1000"}
	}
	if query.Limit == 0 {
		return 100, nil
	}
	return query.Limit, nil
}

func sameRunBeginning(left, right RunRecord) bool {
	return left.ID == right.ID && left.Trigger == right.Trigger && left.LeaseOwner == right.LeaseOwner &&
		left.BaseGenerationID == right.BaseGenerationID && left.StartedAt.Equal(right.StartedAt)
}

func sameRunRecord(left, right RunRecord) bool {
	if !sameRunBeginning(left, right) || !left.CompletedAt.Equal(right.CompletedAt) ||
		left.Duration != right.Duration || left.Status != right.Status ||
		left.PublishedGenerationID != right.PublishedGenerationID || left.SyncRunID != right.SyncRunID ||
		left.FailureType != right.FailureType {
		return false
	}
	attemptsEqual := sameAttemptRecords(left.Attempts, right.Attempts)
	sourcesEqual := sameSourceObservationLinks(left.SourceObservations, right.SourceObservations)
	return attemptsEqual && sourcesEqual
}

func sameAttemptRecords(left, right []AttemptRecord) bool {
	return len(left) == 0 && len(right) == 0 || reflect.DeepEqual(left, right)
}

func sameSourceObservationLinks(left, right []catalogs.SourceObservationLink) bool {
	if len(left) != len(right) {
		return false
	}
	indexed := make(map[catalogmeta.SourceID]catalogs.SourceObservationLink, len(left))
	for _, observation := range left {
		indexed[observation.Source] = observation
	}
	for _, observation := range right {
		if existing, found := indexed[observation.Source]; !found || !reflect.DeepEqual(existing, observation) {
			return false
		}
	}
	return true
}

func runNotFound(id string) error {
	return &errors.NotFoundError{Resource: "catalog scheduler run", ID: id}
}

func runIdentityConflict(id string) error {
	return &errors.ConflictError{Resource: "catalog scheduler run", Expected: id, Actual: id, Message: "run ID is already bound to different state"}
}

func attemptIdentityConflict(runID string, number int) error {
	return &errors.ConflictError{Resource: "catalog scheduler attempt", Expected: runID, Actual: strconv.Itoa(number), Message: "attempt number is already bound to different state"}
}
