// Package starmap provides the main entry point for the Starmap AI model catalog system.
// It offers a high-level interface for managing AI model catalogs with automatic updates,
// event hooks, and provider synchronization capabilities.
//
// Starmap wraps the underlying catalog system with additional features including:
// - Automatic background synchronization with provider APIs
// - Event hooks for model changes (added, updated, removed)
// - Thread-safe catalog access with copy-on-read semantics
// - Flexible configuration through functional options
// - Support for multiple data sources and merge strategies
//
// Example usage:
//
//	// Create a starmap instance with default settings
//	sm, err := starmap.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer sm.AutoUpdatesOff()
//
//	// Register event hooks
//	sm.OnModelAdded(func(model catalogs.Model) {
//	    log.Printf("New model: %s", model.ID)
//	})
//
//	// Get catalog (returns a copy for thread safety)
//	catalog, err := sm.Catalog()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access models
//	models := catalog.Models()
//	for _, model := range models.List() {
//	    fmt.Printf("Model: %s - %s\n", model.ID, model.Name)
//	}
//
//	// Manually trigger sync
//	result, err := sm.Sync(ctx, WithProviders("openai", "anthropic"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Configure with custom options
//	sm, err = starmap.New(
//	    WithAutoUpdateInterval(30 * time.Minute),
//	    WithLocalPath("./custom-catalog"),
//	    WithAutoUpdates(true),
//	)
package starmap

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/agentstation/starmap/internal/sources/local"
	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

// Starmap manages a catalog with automatic updates and event hooks.
type Starmap interface {
	// Catalog returns a copy of the current catalog
	Catalog() (catalogs.Catalog, error)

	// AutoUpdatesOn begins automatic updates if configured
	AutoUpdatesOn() error

	// AutoUpdatesOff stops automatic updates
	AutoUpdatesOff() error

	// Update manually triggers a catalog update
	Update(ctx context.Context) error

	// Sync synchronizes the catalog with provider APIs
	Sync(ctx context.Context, opts ...SyncOption) (*Result, error)

	// OnModelAdded registers a callback for when models are added
	OnModelAdded(ModelAddedHook)

	// OnModelUpdated registers a callback for when models are updated
	OnModelUpdated(ModelUpdatedHook)

	// OnModelRemoved registers a callback for when models are removed
	OnModelRemoved(ModelRemovedHook)

	// Write saves the current catalog to disk
	Write() error
}

// starmap is the internal implementation of the Starmap interface.
type starmap struct {
	mu           sync.RWMutex
	catalog      catalogs.Catalog
	sources      []sources.Source // Configured sources for syncing
	options      *options
	updateTicker *time.Ticker
	stopCh       chan struct{}
	updateCancel context.CancelFunc // Cancel function for update goroutine

	// Event hooks
	hooks *hooks

	// HTTP client for remote server
	httpClient *http.Client
}

// New creates a new Starmap instance with the given options.
func New(opts ...Option) (Starmap, error) {

	sm := &starmap{
		options: defaultOptions(),
		stopCh:  make(chan struct{}),
		hooks:   newHooks(),
		sources: defaultSources(), // Initialize default sources
	}

	if err := sm.apply(opts...); err != nil {
		return nil, errors.WrapResource("apply", "options", "", err)
	}

	// Configure sources based on options
	sm.configureSources()

	// Use provided catalog or create default
	if sm.options.initialCatalog != nil {
		sm.catalog = *sm.options.initialCatalog
	} else {
		// Create and load default embedded catalog
		embeddedCat, err := catalogs.New(catalogs.WithEmbedded())
		if err != nil {
			return nil, errors.WrapResource("create", "embedded catalog", "", err)
		}
		sm.catalog = embeddedCat
	}

	if sm.options.remoteServerURL != nil {
		sm.httpClient = &http.Client{
			Timeout: constants.DefaultHTTPTimeout,
		}
	}

	// Start auto-updates if enabled
	if sm.options.autoUpdatesEnabled {
		if err := sm.AutoUpdatesOn(); err != nil {
			return nil, errors.WrapResource("start", "auto-updates", "", err)
		}
	}

	return sm, nil
}

// Catalog returns a copy of the current catalog.
func (s *starmap) Catalog() (catalogs.Catalog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.catalog.Copy()
}

// AutoUpdatesOn begins automatic updates if configured.
func (s *starmap) AutoUpdatesOn() error {
	if s.options.autoUpdateInterval <= 0 {
		return &errors.ValidationError{
			Field:   "autoUpdateInterval",
			Value:   s.options.autoUpdateInterval,
			Message: "update interval must be positive",
		}
	}

	s.updateTicker = time.NewTicker(s.options.autoUpdateInterval)

	// Create a cancellable context for the update goroutine
	ctx, cancel := context.WithCancel(context.Background())
	s.updateCancel = cancel

	go func(parentCtx context.Context) {
		for {
			select {
			case <-s.updateTicker.C:
				// Create a timeout context for each update (5 minutes default)
				updateCtx, updateCancel := context.WithTimeout(parentCtx, constants.UpdateContextTimeout)
				err := s.Update(updateCtx)
				updateCancel() // Always cancel to release resources

				if err != nil {
					// Check if context was canceled - if so, exit the loop
					if stderrors.Is(err, context.Canceled) || stderrors.Is(err, context.DeadlineExceeded) {
						return
					}
					// Log other errors but continue
					logging.Error().Err(err).Msg("Auto-update failed")
				}
			case <-parentCtx.Done():
				return
			case <-s.stopCh:
				return
			}
		}
	}(ctx)

	return nil
}

// AutoUpdatesOff stops automatic updates.
func (s *starmap) AutoUpdatesOff() error {
	if s.updateTicker != nil {
		s.updateTicker.Stop()
	}
	if s.updateCancel != nil {
		s.updateCancel()
		s.updateCancel = nil
	}
	select {
	case <-s.stopCh:
		// Already closed
	default:
		close(s.stopCh)
	}
	return nil
}

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
	opts := []SyncOption{
		WithDryRun(false),
		WithAutoApprove(true),
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

	resp, err := s.httpClient.Do(req)
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
	newCatalog := catalogs.NewMemory()

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
	s.hooks.triggerCatalogUpdate(oldCatalog, newCatalog)
}

// Write saves the current catalog to disk using the catalog's native save functionality.
func (s *starmap) Write() error {
	s.mu.RLock()
	catalog := s.catalog
	s.mu.RUnlock()

	// Check if the catalog supports saving (e.g., embedded catalog)
	if saveable, ok := catalog.(catalogs.Persistable); ok {
		return saveable.Save()
	}

	// For catalogs that don't support direct saving, we'll use the persistence layer
	// This could be extended later if needed
	return &errors.ConfigError{
		Component: "catalog",
		Message:   "catalog type does not support direct saving",
	}
}

// defaultSources creates the default set of sources.
func defaultSources() []sources.Source {
	return []sources.Source{
		local.New(),
		providers.New(),
		modelsdev.NewGitSource(),
		modelsdev.NewHTTPSource(),
	}
}

// createSourcesWithConfig creates sources configured with sync options.
func createSourcesWithConfig(options *SyncOptions) []sources.Source {
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

// configureSources configures sources based on options.
func (s *starmap) configureSources() {
	// If localPath is configured, update the local source
	if s.options.localPath != "" {
		for i, src := range s.sources {
			// Check if this is the local source by name
			if src.Type() == sources.LocalCatalog {
				s.sources[i] = local.New(local.WithCatalogPath(s.options.localPath))
				break
			}
		}
	}
}

// Sources returns the sources to use based on configuration.
func (s *starmap) filterSources(options *SyncOptions) []sources.Source {
	// Create sources with configuration (especially SourcesDir)
	configuredSources := createSourcesWithConfig(options)

	// If specific sources are requested, filter to those
	if len(options.Sources) > 0 {
		var filtered []sources.Source
		for _, src := range configuredSources {
			if slices.Contains(options.Sources, src.Type()) {
				filtered = append(filtered, src)
			}
		}
		return filtered
	}

	// Otherwise return all configured sources
	return configuredSources
}
