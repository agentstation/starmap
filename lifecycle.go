package starmap

import (
	"context"
	"errors"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

// setup initializes all sources with provider configurations concurrently
func setup(srcs []sources.Source, providers *catalogs.Providers) error {
	var wg sync.WaitGroup
	var errs []error
	var errMutex sync.Mutex

	// setup all sources concurrently
	for _, src := range srcs {
		wg.Add(1)
		go func(src sources.Source) {
			defer wg.Done()

			if err := src.Setup(providers); err != nil {
				wrappedErr := pkgerrors.WrapResource("setup", "source", string(src.Type()), err)
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

// fetch fetches catalogs from all configured sources
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

			logger.Info().Str("source", string(src.Type())).Msg("Fetching")

			// Fetch catalog from source
			if err := src.Fetch(ctx, opts...); err != nil {
				logger.Warn().Err(err).Str("source", string(src.Type())).Msg("Source fetch had errors")

				// Don't skip if we still got a catalog (partial success)
				if src.Catalog() == nil {
					logger.Warn().Str("source", string(src.Type())).Msg("No catalog returned, skipping source")
					// Collect the error even for failed fetches
					wrappedErr := pkgerrors.WrapResource("fetch", "source", string(src.Type()), err)
					errMutex.Lock()
					errs = append(errs, wrappedErr)
					errMutex.Unlock()
					return
				}

				// Even if we got a partial catalog, still record the error
				wrappedErr := pkgerrors.WrapResource("fetch", "source", string(src.Type()), err)
				errMutex.Lock()
				errs = append(errs, wrappedErr)
				errMutex.Unlock()
			}

			// Debug: log provider count from this source
			if src.Catalog() != nil {
				logger.Debug().
					Str("source", string(src.Type())).
					Int("providers", len(src.Catalog().Providers().List())).
					Int("models", len(src.Catalog().GetAllModels())).
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

// cleanup cleans up all sources concurrently, logging and collecting any errors
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
					Str("source", string(src.Type())).
					Msg("Cleanup failed")

				wrappedErr := pkgerrors.WrapResource("cleanup", "source", string(src.Type()), err)
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
