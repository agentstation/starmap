package starmap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/agentstation/starmap/internal/sources/local"
	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/pkg/sync"
)

// Updater handles catalog synchronization operations.
type Updater interface {
	// Sync synchronizes the catalog with provider APIs
	Sync(ctx context.Context, opts ...sync.Option) (*Result, error)

	// Update manually triggers a catalog update
	Update(ctx context.Context) error
}

// Compile-time interface check to ensure proper implementation.
var _ Updater = (*starmap)(nil)

// Update manually triggers a catalog update.
func (s *starmap) Update(ctx context.Context) error {
	if s.options.remoteServerURL != nil {
		return s.updateFromServer(ctx)
	}

	if s.options.autoUpdateFunc != nil {
		s.mu.RLock()
		currentCatalog := s.catalog
		s.mu.RUnlock()

		newCatalog, err := s.options.autoUpdateFunc(currentCatalog)
		if err != nil {
			return err
		}
		s.setCatalog(newCatalog)
	} else {
		// Use pipeline-based update as default
		return s.updateWithPipeline(ctx)
	}

	return nil
}

// updateWithPipeline performs a pipeline-based update for all providers.
func (s *starmap) updateWithPipeline(ctx context.Context) error {
	// Use default options for auto-updates
	opts := []sync.Option{
		sync.WithDryRun(false),
		sync.WithAutoApprove(true),
	}

	// Perform a sync operation with default options
	_, err := s.Sync(ctx, opts...)

	return err
}

// updateFromServer fetches catalog updates from the remote server.
func (s *starmap) updateFromServer(ctx context.Context) error {
	if s.options.remoteServerURL == nil {
		return &errors.ConfigError{
			Component: "starmap",
			Message:   "remote server URL is not set",
		}
	}

	logger := logging.FromContext(ctx)
	logger.Debug().
		Str("url", *s.options.remoteServerURL).
		Msg("Fetching catalog from remote server")

	req, err := http.NewRequestWithContext(ctx, "GET", *s.options.remoteServerURL+"/catalog", nil)
	if err != nil {
		return errors.WrapResource("create", "request", "", err)
	}

	if s.options.remoteServerAPIKey != nil {
		req.Header.Set("Authorization", "Bearer "+*s.options.remoteServerAPIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &errors.APIError{
			Provider: "starmap-server",
			Endpoint: *s.options.remoteServerURL,
			Message:  "failed to make request",
			Err:      err,
		}
	}
	defer func() {
		// Drain and close body to allow connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		logger.Error().
			Int("status_code", resp.StatusCode).
			Str("url", *s.options.remoteServerURL).
			Msg("Remote server returned error status")
		return &errors.APIError{
			Provider:   "starmap-server",
			Endpoint:   *s.options.remoteServerURL,
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("server returned status %d", resp.StatusCode),
		}
	}

	logger.Trace().Msg("Parsing catalog response")

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.WrapIO("read", "response body", err)
	}

	// Parse remote catalog response
	type RemoteCatalogResponse struct {
		Version   string          `json:"version"`
		Catalog   json.RawMessage `json:"catalog"`
		Checksum  string          `json:"checksum,omitempty"`
		Timestamp time.Time       `json:"timestamp"`
	}

	var response RemoteCatalogResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return errors.WrapParse("json", "remote catalog response", err)
	}

	// Create a new memory catalog and populate it
	newCatalog := catalogs.Empty()

	// Parse catalog data structure
	type CatalogData struct {
		Providers []catalogs.Provider `json:"providers,omitempty"`
		Authors   []catalogs.Author   `json:"authors,omitempty"`
		Models    []catalogs.Model    `json:"models,omitempty"`
		Endpoints []catalogs.Endpoint `json:"endpoints,omitempty"`
	}

	var catalogData CatalogData
	if err := json.Unmarshal(response.Catalog, &catalogData); err != nil {
		return errors.WrapParse("json", "catalog data", err)
	}

	// Populate the catalog
	for _, provider := range catalogData.Providers {
		if err := newCatalog.SetProvider(provider); err != nil {
			logger.Warn().Err(err).Str("provider", string(provider.ID)).Msg("Failed to set provider")
		}
	}

	for _, author := range catalogData.Authors {
		if err := newCatalog.SetAuthor(author); err != nil {
			logger.Warn().Err(err).Str("author", string(author.ID)).Msg("Failed to set author")
		}
	}

	// Models are now associated with providers and authors, not set directly
	// They should already be included in the provider/author data structures

	for _, endpoint := range catalogData.Endpoints {
		if err := newCatalog.SetEndpoint(endpoint); err != nil {
			logger.Warn().Err(err).Str("endpoint", endpoint.ID).Msg("Failed to set endpoint")
		}
	}

	// Update the catalog
	s.setCatalog(newCatalog)

	logger.Info().
		Str("version", response.Version).
		Time("timestamp", response.Timestamp).
		Int("providers", len(catalogData.Providers)).
		Int("models", len(catalogData.Models)).
		Msg("Successfully updated catalog from remote server")

	return nil
}

// setCatalog updates the catalog and triggers appropriate event hooks.
func (s *starmap) setCatalog(newCatalog catalogs.Catalog) {
	s.mu.Lock()
	oldCatalog := s.catalog
	s.catalog = newCatalog
	s.mu.Unlock()

	// Trigger hooks for catalog changes
	s.hooks.triggerUpdate(oldCatalog, newCatalog)
}

// Sources returns the sources to use based on configuration.
func (s *starmap) filterSources(options *sync.Options) []sources.Source {
	// Create sources with configuration (especially SourcesDir)
	configuredSources := createSourcesWithConfig(options)

	// If specific sources are requested, filter to those
	if len(options.Sources) > 0 {
		var filtered []sources.Source
		for _, src := range configuredSources {
			if slices.Contains(options.Sources, src.ID()) {
				filtered = append(filtered, src)
			}
		}
		return filtered
	}

	// Otherwise return all configured sources
	return configuredSources
}

// createSourcesWithConfig creates sources configured with sync options.
func createSourcesWithConfig(options *sync.Options) []sources.Source {
	sources := []sources.Source{
		local.New(),
		providers.New(),
	}

	// Configure models.dev sources if SourcesDir is specified
	if options.SourcesDir != "" {
		sources = append(sources,
			modelsdev.NewGitSource(modelsdev.WithSourcesDir(options.SourcesDir)),
			modelsdev.NewHTTPSource(modelsdev.WithHTTPSourcesDir(options.SourcesDir)),
		)
	} else {
		sources = append(sources,
			modelsdev.NewGitSource(),
			modelsdev.NewHTTPSource(),
		)
	}

	return sources
}
