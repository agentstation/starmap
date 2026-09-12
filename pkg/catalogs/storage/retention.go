package storage

import (
	"context"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// DefaultRetentionScanEntries bounds a collection pass unless the caller overrides it.
	DefaultRetentionScanEntries = 4096
	// MaxRetentionScanEntries bounds an explicit collection scan.
	MaxRetentionScanEntries = 100000
)

// RetainingStore coordinates collection with publication and generation read leases.
// AcquireGeneration returns independent bytes and an idempotent release function.
// The lease protects stored content until release, including after current changes.
// Callers must release every successful acquisition. Ordinary Get returns bytes
// without retaining the stored generation after the call completes.
type RetainingStore interface {
	Store
	AcquireGeneration(context.Context, string) (catalogs.Generation, func() error, error)
	Collect(context.Context, RetentionRequest) (RetentionReport, error)
}

// RetentionRequest selects limits for one explicit collection pass.
// ExpectedGenerationID binds the request to current, including an empty store.
// RequiredGenerationIDs names baseline, candidate, and rollback generations.
// Every required ID must exist. The store also protects current and active leases.
// Callers must coordinate changes to their required IDs with collection.
type RetentionRequest struct {
	ExpectedGenerationID  string
	RequiredGenerationIDs []string
	MaxGenerations        int
	MaxBytes              int64
	ScanEntries           int
	DryRun                bool
}

// RetentionUsage counts generations and their manifest plus payload bytes.
// Bytes exclude filesystem overhead, journal files, and backend replication.
type RetentionUsage struct {
	Generations int
	Bytes       int64
}

// RetentionReport describes a complete collection decision and its applied changes.
// Candidates names generations in eviction order. Removed names actual deletions.
// Dry runs leave After equal to Before. Projected describes the proposed result.
// OverLimit means protected content alone exceeds at least one requested limit.
type RetentionReport struct {
	Before     RetentionUsage
	After      RetentionUsage
	Projected  RetentionUsage
	Protected  RetentionUsage
	Candidates []string
	Removed    []string
	OverLimit  bool
}

func (r RetentionRequest) scanLimit() (int, error) {
	if r.MaxGenerations <= 0 || r.MaxBytes <= 0 {
		return 0, &errors.ValidationError{Field: "catalog_retention.limits", Message: "generation and byte limits must be positive"}
	}
	limit := r.ScanEntries
	if limit == 0 {
		limit = DefaultRetentionScanEntries
	}
	if limit < 1 || limit > MaxRetentionScanEntries || len(r.RequiredGenerationIDs) > limit {
		return 0, &errors.ValidationError{Field: "catalog_retention.scan_entries", Message: "scan limit or required generation count exceeds the supported bounds"}
	}
	for _, id := range r.RequiredGenerationIDs {
		if strings.TrimSpace(id) == "" {
			return 0, &errors.ValidationError{Field: "catalog_retention.required_generation", Message: "generation ID must not be empty"}
		}
	}
	return limit, nil
}
