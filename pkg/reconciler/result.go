package reconciler

import (
	"fmt"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// Result represents the outcome of a reconciliation operation.
type Result struct {
	// Core data
	Catalog        catalogs.Catalog
	Changeset      *differ.Changeset
	AppliedChanges *differ.Changeset

	// Tracking maps
	ProviderAPICounts map[catalogs.ProviderID]int
	ModelProviderMap  map[string]catalogs.ProviderID

	// Metadata
	Metadata ResultMetadata

	// Provenance tracking
	Provenance provenance.Map

	// Issues
	Errors   []error
	Warnings []string
}

// ResultMetadata contains metadata about the reconciliation process.
type ResultMetadata struct {
	// StartTime when reconciliation started
	StartTime time.Time

	// EndTime when reconciliation completed
	EndTime time.Time

	// Duration of the reconciliation
	Duration time.Duration

	// Sources that were reconciled
	Sources []sources.ID

	// Strategy used for reconciliation
	Strategy Strategy

	// DryRun indicates if this was a dry-run
	DryRun bool

	// Statistics about the reconciliation
	Stats ResultStatistics
}

// ResultStatistics contains statistics about the reconciliation.
type ResultStatistics struct {
	ModelsProcessed    int
	ProvidersProcessed int
	ConflictsResolved  int
	ResourcesSkipped   int
	TotalTimeMs        int64
}

// IsSuccess returns true if the reconciliation was successful.
func (r *Result) IsSuccess() bool {
	return len(r.Errors) == 0
}

// HasChanges returns true if any changes were detected.
func (r *Result) HasChanges() bool {
	return r.Changeset != nil && r.Changeset.HasChanges()
}

// WasApplied returns true if changes were applied.
func (r *Result) WasApplied() bool {
	return r.AppliedChanges != nil && r.AppliedChanges.HasChanges()
}

// Summary returns a human-readable summary of the result.
func (r *Result) Summary() string {
	if !r.IsSuccess() {
		return fmt.Sprintf("Reconciliation failed with %d errors", len(r.Errors))
	}

	if r.Metadata.DryRun {
		if r.HasChanges() {
			return fmt.Sprintf("Dry run completed. %s", r.Changeset.String())
		}
		return "Dry run completed. No changes detected."
	}

	if r.WasApplied() {
		return fmt.Sprintf("Reconciliation successful. %s", r.AppliedChanges.String())
	}

	if r.HasChanges() {
		return "Reconciliation completed. Changes detected but not applied."
	}

	return "Reconciliation completed. No changes detected."
}

// NewResult creates a new result with defaults.
func NewResult() *Result {
	return &Result{
		ProviderAPICounts: make(map[catalogs.ProviderID]int),
		ModelProviderMap:  make(map[string]catalogs.ProviderID),
		Provenance:        make(provenance.Map),
		Errors:            []error{},
		Warnings:          []string{},
		Metadata: ResultMetadata{
			StartTime: time.Now(),
			Sources:   []sources.ID{},
		},
	}
}

// Finalize calculates duration and marks completion.
func (r *Result) Finalize() {
	r.Metadata.EndTime = time.Now()
	r.Metadata.Duration = r.Metadata.EndTime.Sub(r.Metadata.StartTime)
	r.Metadata.Stats.TotalTimeMs = r.Metadata.Duration.Milliseconds()
}
