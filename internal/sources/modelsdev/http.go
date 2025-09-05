package modelsdev

import (
	"context"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

const (
	defaultHTTPOutputDir = "internal/embedded/catalog/providers"
)

// Package-level state for expensive HTTP operations.
var (
	httpOnce sync.Once
	httpAPI  *API
	httpErr  error
)

// HTTPSource enhances models with models.dev data via HTTP.
type HTTPSource struct {
	providers *catalogs.Providers
	catalog   catalogs.Catalog
}

// NewHTTPSource creates a new models.dev HTTP source.
func NewHTTPSource() *HTTPSource {
	return &HTTPSource{}
}

// Type returns the type of this source.
func (s *HTTPSource) Type() sources.Type {
	return sources.ModelsDevHTTP
}

// ensureHTTPAPI initializes models.dev data once via HTTP.
func ensureHTTPAPI(ctx context.Context, outputDir string) (*API, error) {
	httpOnce.Do(func() {
		if outputDir == "" {
			outputDir = defaultHTTPOutputDir
		}

		client := NewHTTPClient(outputDir)
		if err := client.EnsureAPI(ctx); err != nil {
			httpErr = err
			return
		}
		httpAPI, httpErr = ParseAPI(client.GetAPIPath())
	})
	return httpAPI, httpErr
}

// Setup initializes the source with dependencies.
func (s *HTTPSource) Setup(providers *catalogs.Providers) error {
	s.providers = providers
	return nil
}

// Fetch creates a catalog with models that have pricing/limits data from models.dev.
func (s *HTTPSource) Fetch(ctx context.Context, opts ...sources.Option) error {
	// Create a new catalog to build into
	var err error
	s.catalog, err = catalogs.New()
	if err != nil {
		return errors.WrapResource("create", "memory catalog", "", err)
	}

	// Note: Output directory is now handled by catalog Save() method
	outputDir := defaultHTTPOutputDir

	// Initialize models.dev data once
	api, err := ensureHTTPAPI(ctx, outputDir)
	if err != nil {
		return errors.WrapResource("initialize", "models.dev HTTP", "", err)
	}

	// Process the API data using shared logic
	added, err := processFetch(ctx, s.catalog, api, opts...)
	if err != nil {
		return err
	}

	logging.Info().
		Int("model_count", added).
		Msg("Found models with pricing/limits from models.dev HTTP")
	return nil
}

// Catalog returns the catalog of this source.
func (s *HTTPSource) Catalog() catalogs.Catalog {
	return s.catalog
}

// Cleanup releases any resources.
func (s *HTTPSource) Cleanup() error {
	// HTTPSource doesn't hold persistent resources
	return nil
}
