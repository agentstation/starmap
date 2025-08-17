package starmap

import (
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/agentstation/starmap/internal/catalogs/embedded"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Starmap manages a catalog with automatic updates and event hooks
type Starmap interface {
	// Catalog returns a copy of the current catalog
	Catalog() (catalogs.Catalog, error)

	// Start begins automatic updates if configured
	Start() error

	// Stop stops automatic updates
	Stop() error

	// Update manually triggers a catalog update
	Update() error

	// OnModelAdded registers a callback for when models are added
	OnModelAdded(func(model catalogs.Model))

	// OnModelUpdated registers a callback for when models are updated
	OnModelUpdated(func(old, new catalogs.Model))

	// OnModelRemoved registers a callback for when models are removed
	OnModelRemoved(func(model catalogs.Model))
}

// starmap is the internal implementation of the Starmap interface
type starmap struct {
	mu           sync.RWMutex
	catalog      catalogs.Catalog
	config       *config
	updateTicker *time.Ticker
	stopCh       chan struct{}

	// Event hooks
	onModelAdded   []func(catalogs.Model)
	onModelUpdated []func(catalogs.Model, catalogs.Model)
	onModelRemoved []func(catalogs.Model)

	// HTTP client for remote server
	httpClient *http.Client
}

// config holds the configuration for a Starmap instance
type config struct {
	// Remote server configuration
	serverURL     string
	apiKey        string
	useServerOnly bool // If true, don't hit provider APIs directly

	// Update configuration
	updateInterval time.Duration
	updateFunc     func(catalogs.Catalog) (catalogs.Catalog, error)

	// Initial catalog
	initialCatalog catalogs.Catalog
}

// Option is a function that configures a Starmap instance
type Option func(*config) error

// WithServerURL configures the remote server URL for catalog updates
func WithServerURL(url string) Option {
	return func(c *config) error {
		c.serverURL = url
		return nil
	}
}

// WithAPIKey configures the API key for remote server authentication
func WithAPIKey(key string) Option {
	return func(c *config) error {
		c.apiKey = key
		return nil
	}
}

// WithServerOnly configures whether to only use the remote server and not hit provider APIs
func WithServerOnly(enabled bool) Option {
	return func(c *config) error {
		c.useServerOnly = enabled
		return nil
	}
}

// WithUpdateInterval configures how often to automatically update the catalog
func WithUpdateInterval(interval time.Duration) Option {
	return func(c *config) error {
		c.updateInterval = interval
		return nil
	}
}

// WithUpdateFunc configures a custom function for updating the catalog
func WithUpdateFunc(fn func(catalogs.Catalog) (catalogs.Catalog, error)) Option {
	return func(c *config) error {
		c.updateFunc = fn
		return nil
	}
}

// WithInitialCatalog configures the initial catalog to use
func WithInitialCatalog(catalog catalogs.Catalog) Option {
	return func(c *config) error {
		c.initialCatalog = catalog
		return nil
	}
}

// New creates a new Starmap instance with the given options
func New(opts ...Option) (Starmap, error) {
	cfg := &config{
		updateInterval: 1 * time.Hour, // Default to hourly updates
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("applying option: %w", err)
		}
	}

	// Default to embedded catalog if none provided
	if cfg.initialCatalog == nil {
		catalog := embedded.NewCatalog()
		if err := catalog.Load(); err != nil {
			return nil, fmt.Errorf("loading default catalog: %w", err)
		}
		cfg.initialCatalog = catalog
	}

	sm := &starmap{
		catalog: cfg.initialCatalog,
		config:  cfg,
		stopCh:  make(chan struct{}),
	}

	if cfg.serverURL != "" {
		sm.httpClient = &http.Client{
			Timeout: 30 * time.Second,
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

// Start begins automatic updates if configured
func (s *starmap) Start() error {
	if s.config.updateInterval <= 0 {
		return fmt.Errorf("update interval must be positive")
	}

	s.updateTicker = time.NewTicker(s.config.updateInterval)

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

// Stop stops automatic updates
func (s *starmap) Stop() error {
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
	if s.config.serverURL != "" {
		return s.updateFromServer()
	}

	if s.config.updateFunc != nil {
		s.mu.RLock()
		currentCatalog := s.catalog
		s.mu.RUnlock()

		newCatalog, err := s.config.updateFunc(currentCatalog)
		if err != nil {
			return err
		}
		s.setCatalog(newCatalog)
	}

	return nil
}

// updateFromServer fetches catalog updates from the remote server
func (s *starmap) updateFromServer() error {
	// TODO: Implement remote server catalog fetching
	req, err := http.NewRequest("GET", s.config.serverURL+"/catalog", nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if s.config.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.apiKey)
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

// OnModelAdded registers a callback for when models are added
func (s *starmap) OnModelAdded(fn func(model catalogs.Model)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onModelAdded = append(s.onModelAdded, fn)
}

// OnModelUpdated registers a callback for when models are updated
func (s *starmap) OnModelUpdated(fn func(old, new catalogs.Model)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onModelUpdated = append(s.onModelUpdated, fn)
}

// OnModelRemoved registers a callback for when models are removed
func (s *starmap) OnModelRemoved(fn func(model catalogs.Model)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onModelRemoved = append(s.onModelRemoved, fn)
}

// setCatalog updates the catalog and triggers appropriate event hooks
func (s *starmap) setCatalog(newCatalog catalogs.Catalog) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get old and new models for comparison
	oldModels := s.catalog.Models()
	newModels := newCatalog.Models()

	// Create maps for efficient lookup
	oldModelMap := make(map[string]catalogs.Model)
	for _, model := range oldModels.List() {
		oldModelMap[model.ID] = *model
	}

	newModelMap := make(map[string]catalogs.Model)
	for _, model := range newModels.List() {
		newModelMap[model.ID] = *model
	}

	// Detect changes and trigger hooks
	for _, newModel := range newModels.List() {
		if oldModel, exists := oldModelMap[newModel.ID]; exists {
			// Check if model was updated
			if !reflect.DeepEqual(oldModel, *newModel) {
				for _, hook := range s.onModelUpdated {
					hook(oldModel, *newModel)
				}
			}
		} else {
			// Model was added
			for _, hook := range s.onModelAdded {
				hook(*newModel)
			}
		}
	}

	// Check for removed models
	for _, oldModel := range oldModels.List() {
		if _, exists := newModelMap[oldModel.ID]; !exists {
			for _, hook := range s.onModelRemoved {
				hook(*oldModel)
			}
		}
	}

	s.catalog = newCatalog
}