package operations

import (
	"fmt"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// SyncOptions configures sync operations
type SyncOptions struct {
	OverwriteExisting bool
	SkipEmpty         bool
	DryRun            bool
}

// Sync synchronizes all items from a source catalog to a target catalog.
// Items that exist in both catalogs will be updated, and items that only exist
// in the source catalog will be added to the target catalog.
// Items that only exist in the target catalog are left unchanged.
func Sync(source, target catalogs.Catalog, opts ...SyncOption) error {
	options := &SyncOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if options.DryRun {
		return SyncDryRun(source, target, options)
	}

	// Sync all providers using batch upsert for efficiency
	if providersMap := source.Providers().Map(); len(providersMap) > 0 {
		if !options.SkipEmpty || len(providersMap) > 0 {
			if err := target.Providers().SetBatch(providersMap); err != nil {
				return fmt.Errorf("failed to sync providers: %w", err)
			}
		}
	}

	// Sync all authors using batch upsert for efficiency
	if authorsMap := source.Authors().Map(); len(authorsMap) > 0 {
		if !options.SkipEmpty || len(authorsMap) > 0 {
			if err := target.Authors().SetBatch(authorsMap); err != nil {
				return fmt.Errorf("failed to sync authors: %w", err)
			}
		}
	}

	// Sync all models using batch upsert for efficiency
	if modelsMap := source.Models().Map(); len(modelsMap) > 0 {
		if !options.SkipEmpty || len(modelsMap) > 0 {
			if err := target.Models().SetBatch(modelsMap); err != nil {
				return fmt.Errorf("failed to sync models: %w", err)
			}
		}
	}

	// Sync all endpoints using batch upsert for efficiency
	if endpointsMap := source.Endpoints().Map(); len(endpointsMap) > 0 {
		if !options.SkipEmpty || len(endpointsMap) > 0 {
			if err := target.Endpoints().SetBatch(endpointsMap); err != nil {
				return fmt.Errorf("failed to sync endpoints: %w", err)
			}
		}
	}

	return nil
}

// SyncDryRun performs a dry run sync operation, returning what would be changed
func SyncDryRun(source, target catalogs.Catalog, opts *SyncOptions) error {
	// This would analyze the differences and report what would be changed
	// For now, just return nil as this is a placeholder
	fmt.Println("Dry run mode - would sync:")
	fmt.Printf("  Providers: %d\n", source.Providers().Len())
	fmt.Printf("  Authors: %d\n", source.Authors().Len())
	fmt.Printf("  Models: %d\n", source.Models().Len())
	fmt.Printf("  Endpoints: %d\n", source.Endpoints().Len())
	return nil
}

// SyncOption configures sync behavior
type SyncOption func(*SyncOptions)

// WithOverwrite enables overwriting existing items
func WithOverwrite(overwrite bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.OverwriteExisting = overwrite
	}
}

// WithSkipEmpty skips syncing empty collections
func WithSkipEmpty(skip bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.SkipEmpty = skip
	}
}

// WithDryRun enables dry run mode
func WithDryRun(dryRun bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.DryRun = dryRun
	}
}
