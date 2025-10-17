package starmap

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/agentstation/starmap/internal/deps"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// fetch fetches catalogs from all configured sources.
func fetch(ctx context.Context, srcs []sources.Source, opts []sources.Option) error {

	// setup logger
	logger := logging.FromContext(ctx)

	var wg sync.WaitGroup
	var errs []error
	var errMutex sync.Mutex

	// fetch catalogs from all sources concurrently
	for _, src := range srcs {
		wg.Add(1)
		go func(src sources.Source) {
			defer wg.Done()

			logger.Info().Str("source", string(src.ID())).Msg("Fetching")

			// Fetch catalog from source
			if err := src.Fetch(ctx, opts...); err != nil {
				logger.Warn().Err(err).Str("source", string(src.ID())).Msg("Source fetch had errors")

				// Don't skip if we still got a catalog (partial success)
				if src.Catalog() == nil {
					logger.Warn().Str("source", string(src.ID())).Msg("No catalog returned, skipping source")
					// Collect the error even for failed fetches
					wrappedErr := pkgerrors.WrapResource("fetch", "source", string(src.ID()), err)
					errMutex.Lock()
					errs = append(errs, wrappedErr)
					errMutex.Unlock()
					return
				}

				// Even if we got a partial catalog, still record the error
				wrappedErr := pkgerrors.WrapResource("fetch", "source", string(src.ID()), err)
				errMutex.Lock()
				errs = append(errs, wrappedErr)
				errMutex.Unlock()
			}

			// Debug: log provider count from this source
			if src.Catalog() != nil {
				logger.Debug().
					Str("source", string(src.ID())).
					Int("providers", len(src.Catalog().Providers().List())).
					Int("models", len(src.Catalog().Models().List())).
					Msg("Added source catalog to reconciliation")
			}
		}(src)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Return all errors joined together, or nil if no errors
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// resolveDependencies checks dependencies for all sources and handles missing ones.
// Returns a filtered list of sources that can be used, or an error if resolution fails.
//
// The function operates in three modes based on sync options:
//
//  1. Interactive (default): Prompts user to install missing dependencies or skip sources
//  2. Auto-install (--auto-install-deps): Automatically installs missing dependencies
//  3. Skip prompts (--skip-dep-prompts): Silently skips sources with missing dependencies
//
// If --require-all-sources is set, any missing dependency causes the sync to fail.
//
// Sources that declare IsOptional() as true can be skipped gracefully. Required sources
// must have all dependencies satisfied or the sync will fail.
func resolveDependencies(ctx context.Context, srcs []sources.Source, opts *pkgsync.Options) ([]sources.Source, error) {
	logger := logging.FromContext(ctx)

	// Check dependencies for all sources
	missingDepsMap := make(map[sources.ID][]sources.Dependency)

	for _, src := range srcs {
		sourceDeps := src.Dependencies()
		if len(sourceDeps) == 0 {
			continue // No dependencies, nothing to check
		}

		statuses := deps.CheckAll(ctx, src)

		// Find missing dependencies
		missing := deps.GetMissingDeps(sourceDeps, statuses)
		if len(missing) > 0 {
			missingDepsMap[src.ID()] = missing
		}
	}

	// If no missing dependencies, return original sources
	if len(missingDepsMap) == 0 {
		logger.Debug().Msg("All source dependencies satisfied")
		return srcs, nil
	}

	// Handle missing dependencies
	logger.Info().
		Int("sources_with_missing_deps", len(missingDepsMap)).
		Msg("Some sources have missing dependencies")

	// Process each source with missing deps
	availableSources := make([]sources.Source, 0, len(srcs))
	skippedSources := make([]sources.ID, 0)

	for _, src := range srcs {
		missingDeps, hasMissing := missingDepsMap[src.ID()]
		if !hasMissing {
			// No missing deps, keep this source
			availableSources = append(availableSources, src)
			continue
		}

		// Handle missing dependencies for this source
		shouldSkip, err := handleMissingDeps(ctx, src, missingDeps, opts)
		if err != nil {
			return nil, err
		}

		if shouldSkip {
			skippedSources = append(skippedSources, src.ID())
			logger.Info().
				Str("source", string(src.ID())).
				Msg("Skipping source due to missing dependencies")
		} else {
			// Dependencies resolved, keep the source
			availableSources = append(availableSources, src)
		}
	}

	// Check if we're requiring all sources
	if opts.RequireAllSources && len(skippedSources) > 0 {
		return nil, fmt.Errorf("required sources unavailable due to missing dependencies: %v", skippedSources)
	}

	// Warn if all sources were skipped
	if len(availableSources) == 0 {
		return nil, fmt.Errorf("no sources available - all sources have missing dependencies")
	}

	if len(skippedSources) > 0 {
		logger.Info().
			Int("available", len(availableSources)).
			Int("skipped", len(skippedSources)).
			Msg("Continuing with available sources")
	}

	return availableSources, nil
}

// handleMissingDeps handles missing dependencies for a source.
// Returns true if the source should be skipped, false if deps are resolved or error.
func handleMissingDeps(ctx context.Context, src sources.Source, missingDeps []sources.Dependency, opts *pkgsync.Options) (bool, error) {
	logger := logging.FromContext(ctx)

	// If skip-dep-prompts is set, skip optional sources immediately
	if opts.SkipDepPrompts {
		if src.IsOptional() {
			logger.Info().
				Str("source", string(src.ID())).
				Msg("Skipping optional source (--skip-dep-prompts)")
			return true, nil
		}
		// Required source with missing deps
		return false, fmt.Errorf("required source %s has missing dependencies (use --skip-dep-prompts=false to install)", src.ID())
	}

	// Try to resolve each missing dependency
	for _, dep := range missingDeps {
		// Auto-install if requested
		if opts.AutoInstallDeps {
			logger.Info().
				Str("dependency", dep.Name).
				Str("source", string(src.ID())).
				Msg("Auto-installing dependency")

			if err := deps.AutoInstall(ctx, dep); err != nil {
				logger.Warn().
					Err(err).
					Str("dependency", dep.Name).
					Msg("Auto-install failed")

				// If required source, fail
				if !src.IsOptional() {
					return false, fmt.Errorf("failed to install required dependency %s for source %s: %w", dep.Name, src.ID(), err)
				}
				// Optional source, we'll skip it
				return true, nil
			}

			// Installation succeeded, continue to next dep
			continue
		}

		// Interactive prompt for this dependency
		result := deps.PromptForMissingDep(dep, string(src.ID()))

		switch result {
		case deps.PromptInstall:
			// User wants to install
			if err := deps.AutoInstall(ctx, dep); err != nil {
				logger.Warn().
					Err(err).
					Str("dependency", dep.Name).
					Msg("Installation failed")

				// If required source, fail
				if !src.IsOptional() {
					return false, fmt.Errorf("failed to install required dependency %s: %w", dep.Name, err)
				}
				// Optional source, ask if they want to skip
				if deps.ConfirmSkipSource(string(src.ID()), missingDeps) {
					return true, nil
				}
				return false, fmt.Errorf("cannot continue without %s", dep.Name)
			}

		case deps.PromptSkip:
			// User wants to skip this dependency
			if !src.IsOptional() {
				return false, fmt.Errorf("cannot skip required source %s (missing: %s)", src.ID(), dep.Name)
			}
			return true, nil

		case deps.PromptCancel:
			// User wants to cancel entire operation
			return false, fmt.Errorf("operation cancelled by user")
		}
	}

	// All dependencies resolved
	return false, nil
}

// cleanup cleans up all sources concurrently, logging and collecting any errors.
func cleanup(srcs []sources.Source) error {
	var wg sync.WaitGroup
	var errs []error
	var errMutex sync.Mutex

	// cleanup all sources concurrently
	for _, src := range srcs {
		wg.Add(1)
		go func(src sources.Source) {
			defer wg.Done()

			if err := src.Cleanup(); err != nil {
				logging.Warn().
					Err(err).
					Str("source", string(src.ID())).
					Msg("Cleanup failed")

				wrappedErr := pkgerrors.WrapResource("cleanup", "source", string(src.ID()), err)
				errMutex.Lock()
				errs = append(errs, wrappedErr)
				errMutex.Unlock()
			}
		}(src)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Return all errors joined together, or nil if no errors
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
