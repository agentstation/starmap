// Package errors provides custom error types for the starmap system.
// These errors enable better error handling, programmatic error checking,
// and improved debugging throughout the application.
package errors

import (
	"errors"
	"fmt"
)

// New returns an error that formats as the given text.
// It's an alias for the standard library errors.New for convenience.
var New = errors.New

// Common sentinel errors for the starmap system
var (
	// ErrNotFound indicates that a requested resource was not found
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists indicates that a resource already exists
	ErrAlreadyExists = errors.New("already exists")

	// ErrInvalidInput indicates that provided input was invalid
	ErrInvalidInput = errors.New("invalid input")

	// ErrAPIKeyRequired indicates that an API key is required but not provided
	ErrAPIKeyRequired = errors.New("API key required")

	// ErrAPIKeyInvalid indicates that the provided API key is invalid
	ErrAPIKeyInvalid = errors.New("API key invalid")

	// ErrProviderUnavailable indicates that a provider is temporarily unavailable
	ErrProviderUnavailable = errors.New("provider unavailable")

	// ErrRateLimited indicates that the API rate limit has been exceeded
	ErrRateLimited = errors.New("rate limited")

	// ErrTimeout indicates that an operation timed out
	ErrTimeout = errors.New("operation timed out")

	// ErrCanceled indicates that an operation was canceled
	ErrCanceled = errors.New("operation canceled")

	// ErrNotImplemented indicates that a feature is not yet implemented
	ErrNotImplemented = errors.New("not implemented")

	// ErrReadOnly indicates an attempt to modify a read-only resource
	ErrReadOnly = errors.New("read only")
)

// NotFoundError represents an error when a resource is not found
type NotFoundError struct {
	Resource string
	ID       string
}

// Error implements the error interface
func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s with ID %s not found", e.Resource, e.ID)
}

// Is implements errors.Is support
func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

// NewNotFoundError creates a new NotFoundError
func NewNotFoundError(resource, id string) *NotFoundError {
	return &NotFoundError{Resource: resource, ID: id}
}

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation failed for field %s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation failed: %s", e.Message)
}

// Is implements errors.Is support
func (e *ValidationError) Is(target error) bool {
	return target == ErrInvalidInput
}

// NewValidationError creates a new ValidationError
func NewValidationError(field string, value interface{}, message string) *ValidationError {
	return &ValidationError{Field: field, Value: value, Message: message}
}

// APIError represents an error from a provider API
type APIError struct {
	Provider   string // Provider ID as string
	StatusCode int
	Message    string
	Endpoint   string
	Err        error
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("API error from %s (status %d): %s", e.Provider, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API error from %s: %s", e.Provider, e.Message)
}

// Unwrap implements errors.Unwrap
func (e *APIError) Unwrap() error {
	return e.Err
}

// Is implements errors.Is support
func (e *APIError) Is(target error) bool {
	if e.StatusCode == 429 {
		return target == ErrRateLimited
	}
	if e.StatusCode >= 500 {
		return target == ErrProviderUnavailable
	}
	return false
}

// NewAPIError creates a new APIError
func NewAPIError(provider string, statusCode int, message string) *APIError {
	return &APIError{
		Provider:   provider,
		StatusCode: statusCode,
		Message:    message,
	}
}

// ConfigError represents a configuration error
type ConfigError struct {
	Component string
	Message   string
	Err       error
}

// DependencyError indicates a required external dependency is missing
type DependencyError struct {
	Dependency string
	Message    string
}

// Error implements the error interface
func (e *DependencyError) Error() string {
	return fmt.Sprintf("dependency %s: %s", e.Dependency, e.Message)
}

// Error implements the error interface
func (e *ConfigError) Error() string {
	if e.Component != "" {
		return fmt.Sprintf("configuration error in %s: %s", e.Component, e.Message)
	}
	return fmt.Sprintf("configuration error: %s", e.Message)
}

// Unwrap implements errors.Unwrap
func (e *ConfigError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a new ConfigError
func NewConfigError(component, message string, err error) *ConfigError {
	return &ConfigError{
		Component: component,
		Message:   message,
		Err:       err,
	}
}

// MergeError represents an error during catalog merge operations
type MergeError struct {
	Source      string
	Target      string
	ConflictIDs []string
	Err         error
}

// Error implements the error interface
func (e *MergeError) Error() string {
	if len(e.ConflictIDs) > 0 {
		return fmt.Sprintf("merge conflict between %s and %s for IDs: %v", e.Source, e.Target, e.ConflictIDs)
	}
	return fmt.Sprintf("merge error between %s and %s: %v", e.Source, e.Target, e.Err)
}

// Unwrap implements errors.Unwrap
func (e *MergeError) Unwrap() error {
	return e.Err
}

// NewMergeError creates a new MergeError
func NewMergeError(source, target string, conflictIDs []string, err error) *MergeError {
	return &MergeError{
		Source:      source,
		Target:      target,
		ConflictIDs: conflictIDs,
		Err:         err,
	}
}

// SyncError represents an error during sync operations
type SyncError struct {
	Provider string
	Models   []string
	Err      error
}

// Error implements the error interface
func (e *SyncError) Error() string {
	if len(e.Models) > 0 {
		return fmt.Sprintf("sync error for provider %s (affected models: %v): %v", e.Provider, e.Models, e.Err)
	}
	return fmt.Sprintf("sync error for provider %s: %v", e.Provider, e.Err)
}

// Unwrap implements errors.Unwrap
func (e *SyncError) Unwrap() error {
	return e.Err
}

// NewSyncError creates a new SyncError
func NewSyncError(provider string, models []string, err error) *SyncError {
	return &SyncError{
		Provider: provider,
		Models:   models,
		Err:      err,
	}
}

// Helper functions for error checking

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsAlreadyExists checks if an error is an already exists error
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}

// IsAPIKeyError checks if an error is related to API keys
func IsAPIKeyError(err error) bool {
	return errors.Is(err, ErrAPIKeyRequired) || errors.Is(err, ErrAPIKeyInvalid)
}

// IsRateLimited checks if an error is a rate limit error
func IsRateLimited(err error) bool {
	return errors.Is(err, ErrRateLimited)
}

// IsTimeout checks if an error is a timeout error
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// IsCanceled checks if an error is a cancellation error
func IsCanceled(err error) bool {
	return errors.Is(err, ErrCanceled)
}

// IsProviderUnavailable checks if an error indicates provider unavailability
func IsProviderUnavailable(err error) bool {
	return errors.Is(err, ErrProviderUnavailable)
}

// ParseError represents an error when parsing data formats
type ParseError struct {
	Format  string // "json", "yaml", "toml", etc.
	File    string
	Line    int
	Column  int
	Message string
	Err     error
}

// Error implements the error interface
func (e *ParseError) Error() string {
	if e.File != "" && e.Line > 0 {
		return fmt.Sprintf("parse error in %s at %s:%d:%d: %s", e.Format, e.File, e.Line, e.Column, e.Message)
	}
	if e.File != "" {
		return fmt.Sprintf("parse error in %s file %s: %s", e.Format, e.File, e.Message)
	}
	return fmt.Sprintf("%s parse error: %s", e.Format, e.Message)
}

// Unwrap implements errors.Unwrap
func (e *ParseError) Unwrap() error {
	return e.Err
}

// NewParseError creates a new ParseError
func NewParseError(format, file string, message string, err error) *ParseError {
	return &ParseError{
		Format:  format,
		File:    file,
		Message: message,
		Err:     err,
	}
}

// IOError represents an error during I/O operations
type IOError struct {
	Operation string // "read", "write", "create", "delete", "open", "close"
	Path      string
	Message   string
	Err       error
}

// Error implements the error interface
func (e *IOError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("IO error during %s of %s: %s", e.Operation, e.Path, e.Message)
	}
	return fmt.Sprintf("IO error during %s: %s", e.Operation, e.Message)
}

// Unwrap implements errors.Unwrap
func (e *IOError) Unwrap() error {
	return e.Err
}

// NewIOError creates a new IOError
func NewIOError(operation, path string, err error) *IOError {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return &IOError{
		Operation: operation,
		Path:      path,
		Message:   message,
		Err:       err,
	}
}

// ResourceError represents an error during resource operations
type ResourceError struct {
	Operation string // "create", "update", "delete", "fetch"
	Resource  string // "catalog", "provider", "model", "author"
	ID        string
	Message   string
	Err       error
}

// Error implements the error interface
func (e *ResourceError) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("failed to %s %s %s: %s", e.Operation, e.Resource, e.ID, e.Message)
	}
	return fmt.Sprintf("failed to %s %s: %s", e.Operation, e.Resource, e.Message)
}

// Unwrap implements errors.Unwrap
func (e *ResourceError) Unwrap() error {
	return e.Err
}

// NewResourceError creates a new ResourceError
func NewResourceError(operation, resource, id string, err error) *ResourceError {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return &ResourceError{
		Operation: operation,
		Resource:  resource,
		ID:        id,
		Message:   message,
		Err:       err,
	}
}

// AuthenticationError represents an authentication/authorization error
type AuthenticationError struct {
	Provider string
	Method   string // "api_key", "oauth", "basic", etc.
	Message  string
	Err      error
}

// Error implements the error interface
func (e *AuthenticationError) Error() string {
	if e.Provider != "" {
		return fmt.Sprintf("authentication error for %s (%s): %s", e.Provider, e.Method, e.Message)
	}
	return fmt.Sprintf("authentication error (%s): %s", e.Method, e.Message)
}

// Unwrap implements errors.Unwrap
func (e *AuthenticationError) Unwrap() error {
	return e.Err
}

// Is implements errors.Is support
func (e *AuthenticationError) Is(target error) bool {
	return target == ErrAPIKeyRequired || target == ErrAPIKeyInvalid
}

// NewAuthenticationError creates a new AuthenticationError
func NewAuthenticationError(provider, method, message string, err error) *AuthenticationError {
	return &AuthenticationError{
		Provider: provider,
		Method:   method,
		Message:  message,
		Err:      err,
	}
}

// TimeoutError represents an operation timeout
type TimeoutError struct {
	Operation string
	Duration  string
	Message   string
}

// Error implements the error interface
func (e *TimeoutError) Error() string {
	if e.Duration != "" {
		return fmt.Sprintf("operation %s timed out after %s: %s", e.Operation, e.Duration, e.Message)
	}
	return fmt.Sprintf("operation %s timed out: %s", e.Operation, e.Message)
}

// Is implements errors.Is support
func (e *TimeoutError) Is(target error) bool {
	return target == ErrTimeout
}

// NewTimeoutError creates a new TimeoutError
func NewTimeoutError(operation, duration, message string) *TimeoutError {
	return &TimeoutError{
		Operation: operation,
		Duration:  duration,
		Message:   message,
	}
}

// ProcessError represents an error from an external process or command
type ProcessError struct {
	Operation string // What operation was being performed
	Command   string // The command that was executed
	Output    string // Stdout/stderr output from the process
	ExitCode  int    // Exit code if available
	Err       error  // Underlying error
}

// Error implements the error interface
func (e *ProcessError) Error() string {
	if e.Output != "" {
		return fmt.Sprintf("process error during %s (command: %s): %v\nOutput: %s", e.Operation, e.Command, e.Err, e.Output)
	}
	return fmt.Sprintf("process error during %s (command: %s): %v", e.Operation, e.Command, e.Err)
}

// Unwrap implements errors.Unwrap
func (e *ProcessError) Unwrap() error {
	return e.Err
}

// NewProcessError creates a new ProcessError
func NewProcessError(operation, command, output string, err error) *ProcessError {
	return &ProcessError{
		Operation: operation,
		Command:   command,
		Output:    output,
		Err:       err,
	}
}

// Helper wrapping functions for common patterns

// WrapValidation wraps an error as a ValidationError
func WrapValidation(field string, err error) error {
	if err == nil {
		return nil
	}
	return &ValidationError{Field: field, Message: err.Error()}
}

// WrapIO wraps an error as an IOError
func WrapIO(operation, path string, err error) error {
	if err == nil {
		return nil
	}
	return NewIOError(operation, path, err)
}

// WrapResource wraps an error as a ResourceError
func WrapResource(operation, resource, id string, err error) error {
	if err == nil {
		return nil
	}
	return NewResourceError(operation, resource, id, err)
}

// WrapParse wraps an error as a ParseError
func WrapParse(format, file string, err error) error {
	if err == nil {
		return nil
	}
	return NewParseError(format, file, err.Error(), err)
}

// WrapAPI wraps an error as an APIError
func WrapAPI(provider string, statusCode int, err error) error {
	if err == nil {
		return nil
	}
	return &APIError{
		Provider:   provider,
		StatusCode: statusCode,
		Message:    err.Error(),
		Err:        err,
	}
}