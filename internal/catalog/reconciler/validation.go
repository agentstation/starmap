package reconciler

import (
	"fmt"

	"github.com/agentstation/starmap/pkg/sources"
)

// ValidationResult represents the result of validating a catalog or changeset.
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationWarning
}

// ValidationError represents a validation error.
type ValidationError struct {
	ResourceType sources.ResourceType
	ResourceID   string
	Field        string
	Message      string
}

// ValidationWarning represents a validation warning.
type ValidationWarning struct {
	ResourceType sources.ResourceType
	ResourceID   string
	Field        string
	Message      string
}

// IsValid returns true if validation passed.
func (v *ValidationResult) IsValid() bool {
	return v.Valid && len(v.Errors) == 0
}

// HasWarnings returns true if there are warnings.
func (v *ValidationResult) HasWarnings() bool {
	return len(v.Warnings) > 0
}

// String returns a string representation of the validation result.
func (v *ValidationResult) String() string {
	if v.IsValid() {
		if v.HasWarnings() {
			return fmt.Sprintf("Validation passed with %d warnings", len(v.Warnings))
		}
		return "Validation passed"
	}
	return fmt.Sprintf("Validation failed with %d errors", len(v.Errors))
}
