package starmap

import (
	"context"

	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/save"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/pkg/sync"
)

// Sync synchronizes the catalog with provider APIs using staged source execution.
func (c *Client) Sync(ctx context.Context, opts ...sync.Option) (*sync.Result, error) {
	options := sync.Defaults().Apply(opts...)
	outputPath := options.OutputPath
	if c.options != nil && outputPath == "" && c.options.catalogExportPath != "" && !c.options.embeddedCatalogEnabled {
		outputPath = c.options.catalogExportPath
	}
	if c.options != nil {
		if err := validateCatalogPathSeparation(c.options.catalogStore, outputPath); err != nil {
			return nil, err
		}
	}
	if !options.DryRun {
		if err := c.requireWritableCatalogStore(); err != nil {
			return nil, err
		}
	}

	release, err := c.updates.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	effective := append([]sync.Option(nil), opts...)
	if options.OutputPath == "" && c.options.catalogExportPath != "" && !c.options.embeddedCatalogEnabled {
		effective = append(effective, sync.WithOutputPath(c.options.catalogExportPath))
	}
	return pipeline.New(pipelineStore{client: c}).Sync(ctx, effective...)
}

// ============================================================================
// Helper Methods for Sync
// ============================================================================

// save applies the catalog changes if not in dry-run mode.
func (c *Client) save(ctx context.Context, result *catalogs.Builder, options *sync.Options, changeset *differ.Changeset, observations []sources.Observation) (pipeline.Publication, error) {
	published, err := snapshotBuilder(result)
	if err != nil {
		return pipeline.Publication{}, err
	}

	// Persist first so a failed save does not publish unsaved in-memory state.
	if options.OutputPath != "" {
		// Debug: check what providers have models
		providers := result.Providers().List()
		for _, p := range providers {
			modelCount := 0
			if p.Models != nil {
				modelCount = len(p.Models)
			}
			logging.Info().
				Str("provider", string(p.ID)).
				Int("models", modelCount).
				Msg("Provider model count before save")
		}

		if err := result.Save(save.WithPath(options.OutputPath)); err != nil {
			return pipeline.Publication{}, errors.WrapIO("write", options.OutputPath, err)
		}

		// Copy models.dev logos after successful save.
		providerPtrs := make([]*catalogs.Provider, len(providers))
		for i := range providers {
			providerPtrs[i] = &providers[i]
		}

		// Copy provider logos if we have providers and an output path.
		if len(providerPtrs) > 0 {
			logging.Debug().
				Int("provider_count", len(providerPtrs)).
				Str("output_path", options.OutputPath).
				Msg("Copying provider logos from models.dev")

			if logoErr := modelsdev.CopyProviderLogos(options.OutputPath, providerPtrs); logoErr != nil {
				logging.Warn().
					Err(logoErr).
					Msg("Could not copy provider logos")
				// Non-fatal error - continue without logos
			}
		}

		// Copy author logos from provider logos.
		authors := result.Authors().List()
		if len(authors) > 0 {
			logging.Debug().
				Int("author_count", len(authors)).
				Str("output_path", options.OutputPath).
				Msg("Copying author logos from models.dev provider logos")

			if logoErr := modelsdev.CopyAuthorLogos(options.OutputPath, authors, result.Providers()); logoErr != nil {
				logging.Warn().
					Err(logoErr).
					Msg("Could not copy author logos")
				// Non-fatal error - continue without logos
			}
		}
	} else {
		// Save to default location
		if err := result.Save(save.WithPath(options.OutputPath)); err != nil {
			return pipeline.Publication{}, errors.WrapIO("write", "catalog", err)
		}
	}

	publication, err := c.commitAndPublish(ctx, published, observations)
	if err != nil {
		return pipeline.Publication{}, err
	}

	logging.Info().
		Int("changes_applied", changeset.Summary.TotalChanges).
		Msg("Sync completed successfully")

	return publication, nil
}

type pipelineStore struct {
	client *Client
}

func (s pipelineStore) Catalog() (*catalogs.Catalog, error) {
	return s.client.Catalog(), nil
}

func (s pipelineStore) Apply(ctx context.Context, catalog *catalogs.Builder, options *sync.Options, changeset *differ.Changeset, observations []sources.Observation) (pipeline.Publication, error) {
	return s.client.save(ctx, catalog, options, changeset, observations)
}
