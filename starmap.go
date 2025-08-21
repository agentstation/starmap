package starmap

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/agentstation/starmap/internal/catalogs/embedded"
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
	Update() error

	// Sync synchronizes the catalog with provider APIs
	Sync(opts ...sources.SyncOption) (*catalogs.SyncResult, error)

	// SetSyncOptions sets the sync options used by Update() operations
	SetSyncOptions(options *sources.SyncOptions)

	// GetSyncOptions returns a copy of the current sync options
	GetSyncOptions() *sources.SyncOptions

	// OnModelAdded registers a callback for when models are added
	OnModelAdded(ModelAddedHook)

	// OnModelUpdated registers a callback for when models are updated
	OnModelUpdated(ModelUpdatedHook)

	// OnModelRemoved registers a callback for when models are removed
	OnModelRemoved(ModelRemovedHook)
}

// starmap is the internal implementation of the Starmap interface
type starmap struct {
	mu           sync.RWMutex
	catalog      catalogs.Catalog
	config       *config
	updateTicker *time.Ticker
	stopCh       chan struct{}

	// Event hooks
	hooks *hooks

	// HTTP client for remote server
	httpClient *http.Client

	// Sync options for Update() operations
	syncOptions *sources.SyncOptions
}

// New creates a new Starmap instance with the given options
func New(opts ...Option) (Starmap, error) {

	sm := &starmap{
		config: defaultConfig,
		stopCh: make(chan struct{}),
		hooks:  newHooks(),
	}

	if err := sm.options(opts...); err != nil {
		return nil, fmt.Errorf("applying options: %w", err)
	}

	// Apply sync options from config
	if sm.config.syncOptions != nil {
		sm.syncOptions = sm.config.syncOptions.Copy()
	}

	// Use provided catalog or create default
	if sm.config.initialCatalog != nil {
		sm.catalog = *sm.config.initialCatalog
	} else {
		// Create and load default embedded catalog
		embeddedCat := embedded.NewCatalog()
		// Cast to the concrete type to call Load()
		if loadable, ok := embeddedCat.(interface{ Load() error }); ok {
			if err := loadable.Load(); err != nil {
				return nil, fmt.Errorf("loading embedded catalog: %w", err)
			}
		}
		sm.catalog = embeddedCat
	}

	if sm.config.remoteServerURL != nil {
		sm.httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	// Start auto-updates if enabled
	if sm.config.autoUpdatesEnabled {
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
	if s.config.autoUpdateInterval <= 0 {
		return fmt.Errorf("update interval must be positive")
	}

	s.updateTicker = time.NewTicker(s.config.autoUpdateInterval)

	go func() {
		for {
			select {
			case <-s.updateTicker.C:
				if err := s.Update(); err != nil {
					// Log error but continue
					// TODO: Add proper logging
				}
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
	select {
	case <-s.stopCh:
		// Already closed
	default:
		close(s.stopCh)
	}
	return nil
}

// Update manually triggers a catalog update
func (s *starmap) Update() error {
	if s.config.remoteServerURL != nil {
		return s.updateFromServer()
	}

	if s.config.autoUpdateFunc != nil {
		s.mu.RLock()
		currentCatalog := s.catalog
		s.mu.RUnlock()

		newCatalog, err := s.config.autoUpdateFunc(currentCatalog)
		if err != nil {
			return err
		}
		s.setCatalog(newCatalog)
	} else {
		// Use pipeline-based update as default
		return s.updateWithPipeline()
	}

	return nil
}

// updateWithPipeline performs a pipeline-based update for all providers
func (s *starmap) updateWithPipeline() error {
	// Get the sync options (thread-safe)
	s.mu.RLock()
	syncOptions := s.syncOptions
	s.mu.RUnlock()

	// Use stored sync options or default options for auto-updates
	var opts []sources.SyncOption
	if syncOptions != nil {
		// Convert stored options back to option functions
		// This is simpler than trying to pass the struct directly
		opts = []sources.SyncOption{
			sources.SyncWithAutoApprove(syncOptions.AutoApprove),
			sources.SyncWithDryRun(syncOptions.DryRun),
		}
		if syncOptions.OutputDir != "" {
			opts = append(opts, sources.SyncWithOutputDir(syncOptions.OutputDir))
		}
		if syncOptions.Timeout > 0 {
			opts = append(opts, sources.SyncWithTimeout(syncOptions.Timeout))
		}
		// Add other options as needed
	} else {
		// Use default options for auto-updates
		opts = []sources.SyncOption{
			sources.SyncWithDryRun(false),
			sources.SyncWithAutoApprove(true),
		}
	}

	// Perform a sync operation with configured or default options
	_, err := s.Sync(opts...)

	return err
}

// updateFromServer fetches catalog updates from the remote server
func (s *starmap) updateFromServer() error {
	if s.config.remoteServerURL == nil {
		return fmt.Errorf("remote server URL is not set")
	}

	// TODO: Implement remote server catalog fetching
	req, err := http.NewRequest("GET", *s.config.remoteServerURL+"/catalog", nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if s.config.remoteServerAPIKey != nil {
		req.Header.Set("Authorization", "Bearer "+*s.config.remoteServerAPIKey)
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

// SetSyncOptions sets the sync options used by Update() operations
func (s *starmap) SetSyncOptions(options *sources.SyncOptions) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncOptions = options.Copy() // Store a copy to prevent external mutations
}

// GetSyncOptions returns a copy of the current sync options
func (s *starmap) GetSyncOptions() *sources.SyncOptions {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.syncOptions.Copy()
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
