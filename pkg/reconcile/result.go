package reconcile

import (
	"fmt"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Result represents the outcome of a reconciliation operation
type Result struct {
	// Success indicates if reconciliation completed successfully
	Success bool

	// Catalog is the reconciled catalog (if successful)
	Catalog catalogs.Catalog

	// Changeset contains all changes that were detected
	Changeset *Changeset

	// AppliedChanges contains changes that were actually applied
	AppliedChanges *Changeset

	// Errors contains any errors that occurred
	Errors []error

	// Warnings contains non-critical issues
	Warnings []string

	// Metadata about the reconciliation
	Metadata ResultMetadata

	// Provenance information for audit trail
	Provenance ProvenanceMap
}

// ResultMetadata contains metadata about the reconciliation process
type ResultMetadata struct {
	// StartTime when reconciliation started
	StartTime time.Time

	// EndTime when reconciliation completed
	EndTime time.Time

	// Duration of the reconciliation
	Duration time.Duration

	// Sources that were reconciled
	Sources []SourceName

	// Strategy used for reconciliation
	Strategy Strategy

	// DryRun indicates if this was a dry-run
	DryRun bool

	// Statistics about the reconciliation
	Stats ResultStatistics
}

// ResultStatistics contains statistics about the reconciliation
type ResultStatistics struct {
	// Number of resources processed
	ModelsProcessed    int
	ProvidersProcessed int
	AuthorsProcessed   int

	// Number of conflicts resolved
	ConflictsResolved int

	// Number of resources skipped
	ResourcesSkipped int

	// Performance metrics
	MergeTimeMs   int64
	DiffTimeMs    int64
	ApplyTimeMs   int64
	TotalTimeMs   int64
}

// IsSuccess returns true if the reconciliation was successful
func (r *Result) IsSuccess() bool {
	return r.Success && len(r.Errors) == 0
}

// HasErrors returns true if there were errors
func (r *Result) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if there were warnings
func (r *Result) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// HasChanges returns true if any changes were detected
func (r *Result) HasChanges() bool {
	return r.Changeset != nil && r.Changeset.HasChanges()
}

// WasApplied returns true if changes were applied
func (r *Result) WasApplied() bool {
	return r.AppliedChanges != nil && r.AppliedChanges.HasChanges()
}

// Summary returns a human-readable summary of the result
func (r *Result) Summary() string {
	if !r.Success {
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

// Report generates a detailed report of the reconciliation
func (r *Result) Report() string {
	report := fmt.Sprintf(`
Reconciliation Report
=====================
Status: %s
Duration: %s
Sources: %v
Strategy: %s

`, r.statusString(), r.Metadata.Duration, r.Metadata.Sources, r.Metadata.Strategy)

	// Add statistics
	report += fmt.Sprintf(`Statistics:
-----------
Models Processed: %d
Providers Processed: %d
Authors Processed: %d
Conflicts Resolved: %d
Resources Skipped: %d

`, r.Metadata.Stats.ModelsProcessed,
		r.Metadata.Stats.ProvidersProcessed,
		r.Metadata.Stats.AuthorsProcessed,
		r.Metadata.Stats.ConflictsResolved,
		r.Metadata.Stats.ResourcesSkipped)

	// Add timing information
	report += fmt.Sprintf(`Performance:
------------
Merge Time: %dms
Diff Time: %dms
Apply Time: %dms
Total Time: %dms

`, r.Metadata.Stats.MergeTimeMs,
		r.Metadata.Stats.DiffTimeMs,
		r.Metadata.Stats.ApplyTimeMs,
		r.Metadata.Stats.TotalTimeMs)

	// Add changes summary
	if r.HasChanges() {
		report += fmt.Sprintf(`Changes Detected:
-----------------
%s

`, r.Changeset.String())
	}

	// Add applied changes if different from detected
	if r.WasApplied() && r.AppliedChanges != r.Changeset {
		report += fmt.Sprintf(`Changes Applied:
----------------
%s

`, r.AppliedChanges.String())
	}

	// Add errors
	if r.HasErrors() {
		report += fmt.Sprintf(`Errors (%d):
------------
`, len(r.Errors))
		for i, err := range r.Errors {
			report += fmt.Sprintf("%d. %v\n", i+1, err)
		}
		report += "\n"
	}

	// Add warnings
	if r.HasWarnings() {
		report += fmt.Sprintf(`Warnings (%d):
--------------
`, len(r.Warnings))
		for i, warning := range r.Warnings {
			report += fmt.Sprintf("%d. %s\n", i+1, warning)
		}
		report += "\n"
	}

	return report
}

// statusString returns a string representation of the status
func (r *Result) statusString() string {
	if !r.Success {
		return "❌ Failed"
	}
	if r.Metadata.DryRun {
		return "🔍 Dry Run"
	}
	if r.HasWarnings() {
		return "⚠️  Success with Warnings"
	}
	return "✅ Success"
}

// ResultBuilder helps construct Result objects
type ResultBuilder struct {
	result *Result
}

// NewResultBuilder creates a new ResultBuilder
func NewResultBuilder() *ResultBuilder {
	return &ResultBuilder{
		result: &Result{
			Success:    true,
			Errors:     []error{},
			Warnings:   []string{},
			Metadata:   ResultMetadata{
				StartTime: time.Now(),
				Sources:   []SourceName{},
			},
			Provenance: make(ProvenanceMap),
		},
	}
}

// WithCatalog sets the reconciled catalog
func (b *ResultBuilder) WithCatalog(catalog catalogs.Catalog) *ResultBuilder {
	b.result.Catalog = catalog
	return b
}

// WithChangeset sets the detected changes
func (b *ResultBuilder) WithChangeset(changeset *Changeset) *ResultBuilder {
	b.result.Changeset = changeset
	return b
}

// WithAppliedChanges sets the applied changes
func (b *ResultBuilder) WithAppliedChanges(changeset *Changeset) *ResultBuilder {
	b.result.AppliedChanges = changeset
	return b
}

// WithError adds an error
func (b *ResultBuilder) WithError(err error) *ResultBuilder {
	if err != nil {
		b.result.Success = false
		b.result.Errors = append(b.result.Errors, err)
	}
	return b
}

// WithWarning adds a warning
func (b *ResultBuilder) WithWarning(warning string) *ResultBuilder {
	b.result.Warnings = append(b.result.Warnings, warning)
	return b
}

// WithSources sets the sources that were reconciled
func (b *ResultBuilder) WithSources(sources ...SourceName) *ResultBuilder {
	b.result.Metadata.Sources = sources
	return b
}

// WithStrategy sets the reconciliation strategy
func (b *ResultBuilder) WithStrategy(strategy Strategy) *ResultBuilder {
	b.result.Metadata.Strategy = strategy
	return b
}

// WithDryRun marks this as a dry run
func (b *ResultBuilder) WithDryRun(dryRun bool) *ResultBuilder {
	b.result.Metadata.DryRun = dryRun
	return b
}

// WithProvenance sets the provenance map
func (b *ResultBuilder) WithProvenance(provenance ProvenanceMap) *ResultBuilder {
	b.result.Provenance = provenance
	return b
}

// WithStatistics sets the result statistics
func (b *ResultBuilder) WithStatistics(stats ResultStatistics) *ResultBuilder {
	b.result.Metadata.Stats = stats
	return b
}

// Build finalizes and returns the Result
func (b *ResultBuilder) Build() *Result {
	// Calculate duration
	b.result.Metadata.EndTime = time.Now()
	b.result.Metadata.Duration = b.result.Metadata.EndTime.Sub(b.result.Metadata.StartTime)
	b.result.Metadata.Stats.TotalTimeMs = int64(b.result.Metadata.Duration.Milliseconds())
	
	return b.result
}

// ValidationResult represents the result of validating a catalog or changeset
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationWarning
}

// ValidationError represents a validation error
type ValidationError struct {
	ResourceType ResourceType
	ResourceID   string
	Field        string
	Message      string
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	ResourceType ResourceType
	ResourceID   string
	Field        string
	Message      string
}

// IsValid returns true if validation passed
func (v *ValidationResult) IsValid() bool {
	return v.Valid && len(v.Errors) == 0
}

// HasWarnings returns true if there are warnings
func (v *ValidationResult) HasWarnings() bool {
	return len(v.Warnings) > 0
}

// String returns a string representation of the validation result
func (v *ValidationResult) String() string {
	if v.IsValid() {
		if v.HasWarnings() {
			return fmt.Sprintf("Validation passed with %d warnings", len(v.Warnings))
		}
		return "Validation passed"
	}
	return fmt.Sprintf("Validation failed with %d errors", len(v.Errors))
}