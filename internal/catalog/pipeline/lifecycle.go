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

func observe(ctx context.Context, srcs []sources.Source, opts []sources.Option) ([]sources.Observation, error) {
	logger := logging.FromContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	var errs []error
	var observations []sources.Observation
	var errMutex sync.Mutex

	for _, src := range srcs {
		wg.Add(1)
		go func(src sources.Source) {
			defer wg.Done()

			if ctx.Err() != nil {
				logger.Debug().Str("source", string(src.ID())).Msg("Skipping observation - context cancelled")
				return
			}

			logger.Info().Str("source", string(src.ID())).Msg("Observing")

			observation, err := src.Observe(ctx, opts...)
			if err != nil {
				logger.Warn().Err(err).Str("source", string(src.ID())).Msg("Source observation had errors")

				wrappedErr := pkgerrors.WrapResource("observe", "source", string(src.ID()), err)
				errMutex.Lock()
				errs = append(errs, wrappedErr)
				errMutex.Unlock()

				if observation.Catalog == nil {
					logger.Warn().Str("source", string(src.ID())).Msg("No catalog returned, skipping source")
					return
				}
			}

			if observation.Catalog != nil {
				if validationErr := observation.Validate(); validationErr != nil {
					errMutex.Lock()
					errs = append(errs, pkgerrors.WrapResource("validate", "source observation", string(src.ID()), validationErr))
					errMutex.Unlock()
					return
				}
				if observation.SourceID != src.ID() {
					errMutex.Lock()
					errs = append(errs, &pkgerrors.ValidationError{
						Field:   "observation.source",
						Value:   observation.SourceID,
						Message: "must match configured source " + src.ID().String(),
					})
					errMutex.Unlock()
					return
				}
				errMutex.Lock()
				observations = append(observations, observation)
				errMutex.Unlock()
				logger.Debug().
					Str("source", string(src.ID())).
					Int("providers", len(observation.Catalog.Providers().List())).
					Int("models", len(observation.Catalog.Definitions())).
					Msg("Added source catalog to reconciliation")
			}
		}(src)
	}

	wg.Wait()
	if err := ctx.Err(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return observations, errors.Join(errs...)
	}
	return observations, nil
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
		return nil, &pkgerrors.DependencyError{
			Dependency: "source-set",
			Message:    fmt.Sprintf("required sources unavailable due to missing dependencies: %v", skippedSources),
		}
	}

	if len(availableSources) == 0 {
		return nil, &pkgerrors.ConfigError{
			Component: "sync sources",
			Message:   "no sources available; all configured sources have missing dependencies",
		}
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
		return false, dependencyError(src, missingDeps[0], "required source has a missing dependency")
	}

	for _, dep := range missingDeps {
		decision, err := dependencyDecision(ctx, src, dep, opts)
		if err != nil {
			return false, err
		}

		switch decision {
		case pkgsync.DependencyDecisionInstall:
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
					return false, dependencyError(src, dep, fmt.Sprintf("automatic installation failed: %v", err))
				}
				return true, nil
			}

		case pkgsync.DependencyDecisionSkip:
			if !src.IsOptional() {
				return false, dependencyError(src, dep, "cannot skip a required source")
			}
			return true, nil

		case pkgsync.DependencyDecisionCancel:
			return false, pkgerrors.ErrCanceled

		default:
			return false, &pkgerrors.ValidationError{
				Field:   "DependencyDecision",
				Value:   decision,
				Message: "decision must be install, skip, or cancel",
			}
		}
	}

	return false, nil
}

func dependencyDecision(ctx context.Context, src sources.Source, dep sources.Dependency, opts *pkgsync.Options) (pkgsync.DependencyDecision, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if opts.AutoInstallDeps {
		return pkgsync.DependencyDecisionInstall, nil
	}
	if opts.DependencyDecisionHandler != nil {
		return opts.DependencyDecisionHandler(ctx, src.ID(), dep, src.IsOptional())
	}
	if src.IsOptional() {
		return pkgsync.DependencyDecisionSkip, nil
	}
	return 0, dependencyError(src, dep, "required dependency is missing and no interactive decision adapter is configured")
}

func dependencyError(src sources.Source, dep sources.Dependency, message string) error {
	return &pkgerrors.DependencyError{
		Dependency: dep.Name,
		Message:    fmt.Sprintf("source %s: %s", src.ID(), message),
	}
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
