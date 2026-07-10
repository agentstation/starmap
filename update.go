package starmap

import (
	"context"
	"net/http"

	"github.com/agentstation/starmap/pkg/catalogremote"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/pkg/sync"
)

// Update manually triggers a catalog update.
func (c *Client) Update(ctx context.Context) error {
	if err := c.requireWritableCatalogStore(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if c.options.remoteServerOnly {
		release, err := c.updates.acquire(ctx)
		if err != nil {
			return err
		}
		defer release()
		return c.updateFromServer(ctx)
	}

	if c.options.updateFunc != nil {
		release, err := c.updates.acquire(ctx)
		if err != nil {
			return err
		}
		defer release()

		currentCatalog, err := c.catalogCopy()
		if err != nil {
			return err
		}

		newCatalog, err := c.options.updateFunc(ctx, currentCatalog)
		if err != nil {
			return err
		}
		published, err := snapshotBuilder(newCatalog)
		if err != nil {
			return err
		}
		observation, err := c.catalogObservation(customUpdateSourceID, published, sources.Revision{Kind: sources.RevisionKindContentDigest})
		if err != nil {
			return err
		}
		if _, err := c.commitAndPublish(ctx, published, []sources.Observation{observation}); err != nil {
			return err
		}
	} else {
		// Use pipeline-based update as default
		return c.updateWithPipeline(ctx)
	}

	return nil
}

// updateWithPipeline performs a pipeline-based update for all providers.
func (c *Client) updateWithPipeline(ctx context.Context) error {
	// Use default options for an explicit pipeline update.
	opts := []sync.Option{
		sync.WithDryRun(false),
	}

	// Perform a sync operation with default options
	_, err := c.Sync(ctx, opts...)

	return err
}

// updateFromServer fetches catalog updates from the remote server.
func (c *Client) updateFromServer(ctx context.Context) error {
	if c.options.remoteServerURL == nil {
		return &errors.ConfigError{
			Component: "starmap",
			Message:   "remote server URL is not set",
		}
	}

	logger := logging.FromContext(ctx)
	logger.Debug().Str("url", *c.options.remoteServerURL).Msg("Fetching remote catalog generation")
	var httpClient *http.Client
	if c.options.remoteServerAPIKey != nil {
		httpClient = &http.Client{Transport: authorizationTransport{
			base: http.DefaultTransport, token: *c.options.remoteServerAPIKey,
		}, Timeout: constants.DefaultHTTPTimeout}
	}
	remote, err := catalogremote.NewClient(*c.options.remoteServerURL, httpClient, catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		return err
	}
	generation, err := remote.FetchCurrent(ctx)
	if err != nil {
		return err
	}
	published, err := catalogstore.DecodeCatalogPayload(generation.Payload)
	if err != nil {
		return errors.WrapResource("decode", "remote catalog generation", generation.Manifest.GenerationID, err)
	}
	if err := c.commitReceivedGeneration(ctx, published, generation); err != nil {
		return err
	}
	logger.Info().Str("generation_id", generation.Manifest.GenerationID).
		Str("sync_run_id", generation.Manifest.SyncRunID).
		Msg("Successfully updated catalog from remote generation")
	return nil
}

type authorizationTransport struct {
	base  http.RoundTripper
	token string
}

func (t authorizationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(clone)
}

func (c *Client) catalogObservation(sourceID sources.ID, catalog *catalogs.Catalog, revision sources.Revision) (sources.Observation, error) {
	return sources.NewObservation(sourceID, catalog, sources.ObservationMetadata{
		ObservedAt:   c.currentTime(),
		Revision:     revision,
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
	})
}

func (c *Client) swapCatalogGeneration(published *catalogs.Catalog, generationID string) *catalogs.Catalog {
	c.mu.Lock()
	oldCatalog := c.catalog
	c.catalog = published
	c.usingEmbeddedBootstrap = false
	c.generationSequence++
	if generationID != "" {
		c.generationID = generationID
	}
	c.mu.Unlock()
	return oldCatalog
}
