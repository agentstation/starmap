package starmap

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/agentstation/starmap/internal/sources/local"
	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// Starmap manages a catalog with automatic updates and event hooks
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
	Sync(ctx context.Context, opts ...SyncOption) (*SyncResult, error)

	// OnModelAdded registers a callback for when models are added
	OnModelAdded(ModelAddedHook)

	// OnModelUpdated registers a callback for when models are updated
	OnModelUpdated(ModelUpdatedHook)

	// OnModelRemoved registers a callback for when models are removed
	OnModelRemoved(ModelRemovedHook)

	// Write saves the current catalog to disk
	Write() error
}

// starmap is the internal implementation of the Starmap interface
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

// New creates a new Starmap instance with the given options
func New(opts ...Option) (Starmap, error) {

	sm := &starmap{
		options: defaultOptions(),
		stopCh:  make(chan struct{}),
		hooks:   newHooks(),
		sources: defaultSources(), // Initialize default sources
	}

	if err := sm.apply(opts...); err != nil {
		return nil, fmt.Errorf("applying options: %w", err)
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
			return nil, fmt.Errorf("creating embedded catalog: %w", err)
		}
		sm.catalog = embeddedCat
	}

	if sm.options.remoteServerURL != nil {
		sm.httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	// Start auto-updates if enabled
	if sm.options.autoUpdatesEnabled {
		if err := sm.AutoUpdatesOn(); err != nil {
			return nil, fmt.Errorf("starting auto-updates: %w", err)
		}
	}

	return sm, nil
}

// Catalog returns a copy of the current catalog
func (s *starmap) Catalog() (catalogs.Catalog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.catalog.Copy()
}

// AutoUpdatesOn begins automatic updates if configured
func (s *starmap) AutoUpdatesOn() error {
	if s.options.autoUpdateInterval <= 0 {
		return fmt.Errorf("update interval must be positive")
	}

	s.updateTicker = time.NewTicker(s.options.autoUpdateInterval)
	
	// Create a cancellable context for the update goroutine
	ctx, cancel := context.WithCancel(context.Background())
	s.updateCancel = cancel

	go func() {
		for {
			select {
			case <-s.updateTicker.C:
				// Create a timeout context for each update (5 minutes default)
				updateCtx, updateCancel := context.WithTimeout(ctx, 5*time.Minute)
				if err := s.Update(updateCtx); err != nil {
					// Log error but continue
					// TODO: Add proper logging
				}
				updateCancel()
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			}
		}
	}()

	return nil
}

// AutoUpdatesOff stops automatic updates
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

// Update manually triggers a catalog update
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

// updateWithPipeline performs a pipeline-based update for all providers
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

// updateFromServer fetches catalog updates from the remote server
func (s *starmap) updateFromServer(ctx context.Context) error {
	if s.options.remoteServerURL == nil {
		return fmt.Errorf("remote server URL is not set")
	}

	// TODO: Implement remote server catalog fetching
	req, err := http.NewRequestWithContext(ctx, "GET", *s.options.remoteServerURL+"/catalog", nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if s.options.remoteServerAPIKey != nil {
		req.Header.Set("Authorization", "Bearer "+*s.options.remoteServerAPIKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	// TODO: Parse response and update catalog
	// For now, this is a stub implementation

	return nil
}

// setCatalog updates the catalog and triggers appropriate event hooks
func (s *starmap) setCatalog(newCatalog catalogs.Catalog) {
	s.mu.Lock()
	oldCatalog := s.catalog
	s.catalog = newCatalog
	s.mu.Unlock()

	// Trigger hooks for catalog changes
	s.hooks.triggerCatalogUpdate(oldCatalog, newCatalog)
}

// Write saves the current catalog to disk using the catalog's native save functionality
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
	return fmt.Errorf("catalog type does not support direct saving")
}

// defaultSources creates the default set of sources
func defaultSources() []sources.Source {
	return []sources.Source{
		local.New(),
		providers.New(),
		modelsdev.NewGitSource(),
		modelsdev.NewHTTPSource(),
	}
}

// configureSources configures sources based on options
func (s *starmap) configureSources() {
	// If localPath is configured, update the local source
	if s.options.localPath != "" {
		for i, src := range s.sources {
			// Check if this is the local source by name
			if src.Name() == sources.LocalCatalog {
				s.sources[i] = local.New(local.WithCatalogPath(s.options.localPath))
				break
			}
		}
	}
}

// sourcesWithOptions returns the sources to use based on configuration
func (s *starmap) sourcesWithOptions(options *SyncOptions) []sources.Source {
	// If specific sources are requested, filter to those
	if len(options.Sources) > 0 {
		var filtered []sources.Source
		for _, src := range s.sources {
			for _, requestedName := range options.Sources {
				if src.Name() == requestedName {
					filtered = append(filtered, src)
					break
				}
			}
		}
		return filtered
	}

	// Otherwise return all sources
	return s.sources
}
