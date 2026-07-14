// Package sync provides options and utilities for synchronizing the catalog with provider APIs.
package sync

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// Options controls the overall sync orchestration in Starmap.Sync().
type Options struct {
	// Orchestration control
	DryRun  bool          // Show changes without applying them
	Timeout time.Duration // Timeout for the entire sync operation

	// Source selection
	Sources    []sources.ID         // Which sources to use (empty means default providers/local/models.dev HTTP)
	ProviderID *catalogs.ProviderID // Filter for specific provider

	// Output control (used AFTER merging)
	OutputPath string // Where to save final catalog (empty means default location)

	// Source behavior control
	Fresh              bool   // Delete existing models and fetch fresh from APIs (destructive)
	CleanModelsDevRepo bool   // Remove temporary models.dev repository after update
	Reformat           bool   // Reformat providers.yaml file even without changes
	SourcesDir         string // Directory for external source data (models.dev cache/git)
	ModelsDevGitCommit string // Exact models.dev commit required by Git verification

	// Dependency control
	AutoInstallDeps   bool // Automatically install missing dependencies without prompting
	SkipDepPrompts    bool // Skip dependency prompts and continue without optional dependencies
	RequireAllSources bool // Require all sources to succeed (fail if any dependencies are missing)

	// DependencyDecisionHandler is supplied by an interactive adapter. It is nil
	// for library, server, scheduler, and other noninteractive callers.
	DependencyDecisionHandler DependencyDecisionHandler
}

// DependencyDecision describes how an interactive adapter wants to handle one
// missing source dependency.
type DependencyDecision uint8

const (
	optionFieldSources = "Sources"
	// DependencyDecisionInstall requests automatic installation.
	DependencyDecisionInstall DependencyDecision = iota + 1
	// DependencyDecisionSkip requests skipping the source.
	DependencyDecisionSkip
	// DependencyDecisionCancel cancels synchronization.
	DependencyDecisionCancel
)

// DependencyDecisionHandler lets an interactive adapter decide how to handle a
// missing dependency. Core synchronization never installs a terminal reader or
// prompts on its own.
type DependencyDecisionHandler func(context.Context, sources.ID, sources.Dependency, bool) (DependencyDecision, error)

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
		Timeout:            constants.UpdateContextTimeout,
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
func (s *Options) Validate(providers catalogs.ProvidersReader) error {
	// Validate timeout
	if s.Timeout < 0 {
		return &errors.ValidationError{
			Field:   "Timeout",
			Value:   s.Timeout,
			Message: "timeout must be non-negative",
		}
	}

	if s.AutoInstallDeps && s.SkipDepPrompts {
		return &errors.ValidationError{
			Field:   "DependencyPolicy",
			Value:   "auto-install+skip",
			Message: "automatic installation and dependency skipping are mutually exclusive",
		}
	}
	if s.DependencyDecisionHandler != nil && (s.AutoInstallDeps || s.SkipDepPrompts) {
		return &errors.ValidationError{
			Field:   "DependencyPolicy",
			Value:   "interactive+noninteractive",
			Message: "an interactive dependency decision handler cannot be combined with automatic installation or skipping",
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

	for _, sourceID := range s.Sources {
		if !sourceID.IsValid() {
			return &errors.ValidationError{
				Field:   optionFieldSources,
				Value:   sourceID,
				Message: fmt.Sprintf("source %q is not supported", sourceID),
			}
		}
		if s.Fresh && sourceID == sources.LocalCatalogID {
			return &errors.ValidationError{
				Field:   optionFieldSources,
				Value:   sourceID,
				Message: "fresh sync cannot use the existing local catalog as an input source",
			}
		}
	}
	if slices.Contains(s.Sources, sources.ModelsDevHTTPID) && slices.Contains(s.Sources, sources.ModelsDevGitID) {
		return &errors.ValidationError{
			Field:   optionFieldSources,
			Value:   s.Sources,
			Message: "models.dev HTTP and Git are alternative transports; select exactly one",
		}
	}
	usesGit := slices.Contains(s.Sources, sources.ModelsDevGitID)
	if usesGit {
		if !isExactGitCommit(s.ModelsDevGitCommit) {
			return &errors.ValidationError{
				Field: "ModelsDevGitCommit", Value: s.ModelsDevGitCommit,
				Message: "an exact 40- or 64-character hexadecimal commit is required for models.dev Git verification",
			}
		}
	} else if s.ModelsDevGitCommit != "" {
		return &errors.ValidationError{
			Field: "ModelsDevGitCommit", Value: s.ModelsDevGitCommit,
			Message: "requires models_dev_git as an explicitly selected source",
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

// WithTimeout configures the sync timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(opts *Options) {
		opts.Timeout = timeout
	}
}

// WithSources configures which sources to use.
func WithSources(types ...sources.ID) Option {
	return func(opts *Options) {
		opts.Sources = append([]sources.ID(nil), types...)
	}
}

// WithProvider configures syncing for a specific provider only.
func WithProvider(providerID catalogs.ProviderID) Option {
	return func(opts *Options) {
		opts.ProviderID = &providerID
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

// WithModelsDevGitCommit pins explicit models.dev Git verification to one commit.
func WithModelsDevGitCommit(commit string) Option {
	return func(opts *Options) {
		opts.ModelsDevGitCommit = commit
	}
}

func isExactGitCommit(commit string) bool {
	if len(commit) != 40 && len(commit) != 64 {
		return false
	}
	_, err := hex.DecodeString(commit)
	return err == nil
}

// WithAutoInstallDeps configures whether to automatically install missing dependencies.
func WithAutoInstallDeps(autoInstall bool) Option {
	return func(opts *Options) {
		opts.AutoInstallDeps = autoInstall
	}
}

// WithSkipDepPrompts skips optional sources with missing dependencies without
// consulting a configured DependencyDecisionHandler.
func WithSkipDepPrompts(skip bool) Option {
	return func(opts *Options) {
		opts.SkipDepPrompts = skip
	}
}

// WithDependencyDecisionHandler configures the interactive dependency decision
// adapter. Noninteractive callers should leave it unset.
func WithDependencyDecisionHandler(handler DependencyDecisionHandler) Option {
	return func(opts *Options) {
		opts.DependencyDecisionHandler = handler
	}
}

// WithRequireAllSources configures whether all sources are required to succeed.
func WithRequireAllSources(require bool) Option {
	return func(opts *Options) {
		opts.RequireAllSources = require
	}
}
