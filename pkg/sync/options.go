// Package sync provides options and utilities for synchronizing the catalog with provider APIs.
package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agentstation/starmap/internal/utils/ptr"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// Options controls the overall sync orchestration in Starmap.Sync().
type Options struct {
	// Orchestration control
	DryRun      bool          // Show changes without applying them
	AutoApprove bool          // Skip confirmation prompts
	FailFast    bool          // Stop on first source error instead of continuing
	Timeout     time.Duration // Timeout for the entire sync operation

	// Source selection
	Sources    []sources.ID         // Which sources to use (empty means all)
	ProviderID *catalogs.ProviderID // Filter for specific provider

	// Output control (used AFTER merging)
	OutputPath string // Where to save final catalog (empty means default location)

	// Source behavior control
	Fresh              bool   // Delete existing models and fetch fresh from APIs (destructive)
	CleanModelsDevRepo bool   // Remove temporary models.dev repository after update
	Reformat           bool   // Reformat providers.yaml file even without changes
	SourcesDir         string // Directory for external source data (models.dev cache/git)

	// Dependency control
	AutoInstallDeps   bool // Automatically install missing dependencies without prompting
	SkipDepPrompts    bool // Skip dependency prompts and continue without optional dependencies
	RequireAllSources bool // Require all sources to succeed (fail if any dependencies are missing)
}

// Apply applies the given options to the sync options.
func (s *Options) Apply(opts ...Option) *Options {
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Defaults returns the default sync options.
func Defaults() *Options {
	return &Options{
		DryRun:             false,
		AutoApprove:        false,
		FailFast:           false,
		Timeout:            0,
		Sources:            nil,
		ProviderID:         nil,
		OutputPath:         "",
		Fresh:              false,
		CleanModelsDevRepo: false,
		Reformat:           false,
		AutoInstallDeps:    false,
		SkipDepPrompts:     false,
		RequireAllSources:  false,
	}
}

// Option is a function that configures sync Options.
type Option func(*Options)

// Validate checks if the sync options are valid.
func (s *Options) Validate(providers *catalogs.Providers) error {
	// Validate timeout
	if s.Timeout < 0 {
		return &errors.ValidationError{
			Field:   "Timeout",
			Value:   s.Timeout,
			Message: "timeout must be non-negative",
		}
	}

	// Validate provider ID if specified
	if s.ProviderID != nil {
		_, found := providers.Get(*s.ProviderID)
		if !found {
			return &errors.ValidationError{
				Field:   "ProviderID",
				Value:   *s.ProviderID,
				Message: fmt.Sprintf("provider '%s' not found", *s.ProviderID),
			}
		}
	}

	// Validate output path if specified
	if s.OutputPath != "" {
		// Check if parent directory exists
		dir := filepath.Dir(s.OutputPath)
		if dir != "." && dir != "/" {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return &errors.ValidationError{
					Field:   "OutputPath",
					Value:   s.OutputPath,
					Message: fmt.Sprintf("output directory '%s' does not exist", dir),
				}
			}
		}
	}

	return nil
}

// SourceOptions converts sync options to properly typed source options.
func (s *Options) SourceOptions() []sources.Option {
	var sourceOpts []sources.Option

	if s.ProviderID != nil {
		sourceOpts = append(sourceOpts, sources.WithProviderFilter(*s.ProviderID))
	}
	if s.Fresh {
		sourceOpts = append(sourceOpts, sources.WithFresh(true))
	}
	if s.CleanModelsDevRepo {
		sourceOpts = append(sourceOpts, sources.WithCleanupRepo(true))
	}
	if s.Reformat {
		sourceOpts = append(sourceOpts, sources.WithReformat(true))
	}

	return sourceOpts
}

// WithDryRun configures dry run mode.
func WithDryRun(dryRun bool) Option {
	return func(opts *Options) {
		opts.DryRun = dryRun
	}
}

// WithAutoApprove configures auto approval.
func WithAutoApprove(autoApprove bool) Option {
	return func(opts *Options) {
		opts.AutoApprove = autoApprove
	}
}

// WithFailFast configures fail-fast behavior.
func WithFailFast(failFast bool) Option {
	return func(opts *Options) {
		opts.FailFast = failFast
	}
}

// WithTimeout configures the sync timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(opts *Options) {
		opts.Timeout = timeout
	}
}

// WithSources configures which sources to use.
func WithSources(types ...sources.ID) Option {
	return func(opts *Options) {
		opts.Sources = types
	}
}

// WithProvider configures syncing for a specific provider only.
func WithProvider(providerID catalogs.ProviderID) Option {
	return func(opts *Options) {
		opts.ProviderID = ptr.To(providerID)
	}
}

// WithOutputPath configures the output path for saving.
func WithOutputPath(path string) Option {
	return func(opts *Options) {
		opts.OutputPath = path
	}
}

// WithFresh configures whether to delete existing models and fetch fresh from APIs.
func WithFresh(fresh bool) Option {
	return func(opts *Options) {
		opts.Fresh = fresh
	}
}

// WithCleanModelsDevRepo configures whether to remove temporary models.dev repository after update.
func WithCleanModelsDevRepo(cleanup bool) Option {
	return func(opts *Options) {
		opts.CleanModelsDevRepo = cleanup
	}
}

// WithReformat configures whether to reformat providers.yaml file even without changes.
func WithReformat(reformat bool) Option {
	return func(opts *Options) {
		opts.Reformat = reformat
	}
}

// WithSourcesDir configures the directory for external source data (models.dev cache/git).
func WithSourcesDir(dir string) Option {
	return func(opts *Options) {
		opts.SourcesDir = dir
	}
}

// WithAutoInstallDeps configures whether to automatically install missing dependencies.
func WithAutoInstallDeps(autoInstall bool) Option {
	return func(opts *Options) {
		opts.AutoInstallDeps = autoInstall
	}
}

// WithSkipDepPrompts configures whether to skip dependency prompts.
func WithSkipDepPrompts(skip bool) Option {
	return func(opts *Options) {
		opts.SkipDepPrompts = skip
	}
}

// WithRequireAllSources configures whether all sources are required to succeed.
func WithRequireAllSources(require bool) Option {
	return func(opts *Options) {
		opts.RequireAllSources = require
	}
}
