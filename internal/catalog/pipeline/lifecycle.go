package pipeline

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

func fetch(ctx context.Context, srcs []sources.Source, opts []sources.Option) error {
	logger := logging.FromContext(ctx)

	var wg sync.WaitGroup
	var errs []error
	var errMutex sync.Mutex

	for _, src := range srcs {
		wg.Add(1)
		go func(src sources.Source) {
			defer wg.Done()

			if ctx.Err() != nil {
				logger.Debug().Str("source", string(src.ID())).Msg("Skipping fetch - context cancelled")
				return
			}

			logger.Info().Str("source", string(src.ID())).Msg("Fetching")

			if err := src.Fetch(ctx, opts...); err != nil {
				logger.Warn().Err(err).Str("source", string(src.ID())).Msg("Source fetch had errors")

				wrappedErr := pkgerrors.WrapResource("fetch", "source", string(src.ID()), err)
				errMutex.Lock()
				errs = append(errs, wrappedErr)
				errMutex.Unlock()

				if src.Catalog() == nil {
					logger.Warn().Str("source", string(src.ID())).Msg("No catalog returned, skipping source")
					return
				}
			}

			if src.Catalog() != nil {
				logger.Debug().
					Str("source", string(src.ID())).
					Int("providers", len(src.Catalog().Providers().List())).
					Int("models", len(src.Catalog().Models().List())).
					Msg("Added source catalog to reconciliation")
			}
		}(src)
	}

	wg.Wait()

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func resolveDependencies(ctx context.Context, srcs []sources.Source, opts *pkgsync.Options) ([]sources.Source, error) {
	logger := logging.FromContext(ctx)

	missingDepsMap := make(map[sources.ID][]sources.Dependency)

	for _, src := range srcs {
		sourceDeps := src.Dependencies()
		if len(sourceDeps) == 0 {
			continue
		}

		statuses := deps.CheckAll(ctx, src)
		missing := deps.GetMissingDeps(sourceDeps, statuses)
		if len(missing) > 0 {
			missingDepsMap[src.ID()] = missing
		}
	}

	if len(missingDepsMap) == 0 {
		logger.Debug().Msg("All source dependencies satisfied")
		return srcs, nil
	}

	logger.Info().
		Int("sources_with_missing_deps", len(missingDepsMap)).
		Msg("Some sources have missing dependencies")

	availableSources := make([]sources.Source, 0, len(srcs))
	skippedSources := make([]sources.ID, 0)

	for _, src := range srcs {
		missingDeps, hasMissing := missingDepsMap[src.ID()]
		if !hasMissing {
			availableSources = append(availableSources, src)
			continue
		}

		shouldSkip, err := handleMissingDeps(ctx, src, missingDeps, opts)
		if err != nil {
			return nil, err
		}

		if shouldSkip {
			skippedSources = append(skippedSources, src.ID())
			logger.Info().
				Str("source", string(src.ID())).
				Msg("Skipping source due to missing dependencies")
			continue
		}

		availableSources = append(availableSources, src)
	}

	if opts.RequireAllSources && len(skippedSources) > 0 {
		return nil, fmt.Errorf("required sources unavailable due to missing dependencies: %v", skippedSources)
	}

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

func handleMissingDeps(ctx context.Context, src sources.Source, missingDeps []sources.Dependency, opts *pkgsync.Options) (bool, error) {
	logger := logging.FromContext(ctx)

	if opts.SkipDepPrompts {
		if src.IsOptional() {
			logger.Info().
				Str("source", string(src.ID())).
				Msg("Skipping optional source (--skip-dep-prompts)")
			return true, nil
		}
		return false, fmt.Errorf("required source %s has missing dependencies (use --skip-dep-prompts=false to install)", src.ID())
	}

	for _, dep := range missingDeps {
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

				if !src.IsOptional() {
					return false, fmt.Errorf("failed to install required dependency %s for source %s: %w", dep.Name, src.ID(), err)
				}
				return true, nil
			}

			continue
		}

		result := deps.PromptForMissingDep(dep, string(src.ID()))

		switch result {
		case deps.PromptInstall:
			if err := deps.AutoInstall(ctx, dep); err != nil {
				logger.Warn().
					Err(err).
					Str("dependency", dep.Name).
					Msg("Installation failed")

				if !src.IsOptional() {
					return false, fmt.Errorf("failed to install required dependency %s: %w", dep.Name, err)
				}
				if deps.ConfirmSkipSource(string(src.ID()), missingDeps) {
					return true, nil
				}
				return false, fmt.Errorf("cannot continue without %s", dep.Name)
			}

		case deps.PromptSkip:
			if !src.IsOptional() {
				return false, fmt.Errorf("cannot skip required source %s (missing: %s)", src.ID(), dep.Name)
			}
			return true, nil

		case deps.PromptCancel:
			return false, fmt.Errorf("operation cancelled by user")
		}
	}

	return false, nil
}

func cleanup(ctx context.Context, srcs []sources.Source) error {
	if err := ctx.Err(); err != nil {
		logging.Warn().Err(err).Msg("Cleanup skipped - context already cancelled")
		return err
	}

	var wg sync.WaitGroup
	var errs []error
	var errMutex sync.Mutex

	for _, src := range srcs {
		wg.Add(1)
		go func(src sources.Source) {
			defer wg.Done()

			if ctx.Err() != nil {
				return
			}

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

	wg.Wait()

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
