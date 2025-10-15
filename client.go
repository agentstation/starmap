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
	"sync"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// Compile-time interface check to ensure proper implementation.
var _ Catalog = (*client)(nil)

// Catalog provides copy-on-read access to the catalog.
type Catalog interface {
	Catalog() (catalogs.Catalog, error)
}

// Catalog returns a copy of the current catalog.
func (c *client) Catalog() (catalogs.Catalog, error) {
	c.mu.RLock()
	cat, err := c.catalog.Copy()
	c.mu.RUnlock()
	return cat, err
}

// Client manages a catalog with automatic updates and event hooks.
type Client interface {

	// Catalog provides copy-on-read access to the catalog
	Catalog

	// Updater handles catalog update and sync operations
	Updater

	// Persistence handles catalog persistence operations
	Persistence

	// AutoUpdater provides access to automatic update controls
	AutoUpdater

	// Hooks provides access to event callback registration
	Hooks
}

// client is the internal implementation of the Client interface.
type client struct {

	// options are the configured options for the client
	options *options

	// catalog is the working up to date catalog
	mu      sync.RWMutex
	catalog catalogs.Catalog // working up to date catalog
	local   catalogs.Catalog // local catalog

	// auto update state
	autoUpdatesEnabled bool
	autoUpdateInterval time.Duration
	autoUpdateFunc     AutoUpdateFunc
	updateTicker       *time.Ticker       // update ticker to trigger auto-updates
	stopCh             chan struct{}      // stop channel to stop auto-updates
	updateCancel       context.CancelFunc // Cancel function for update goroutine
	hooks              *hooks             // Event hooks for catalog changes/updates
}

// New creates a new Client instance with the given options.
func New(opts ...Option) (Client, error) {

	// start with a new empty catalog to build on
	catalog, err := catalogs.New()
	if err != nil {
		return nil, errors.WrapResource("create", "catalog", "", err)
	}

	// create the client instance
	sm := &client{
		// options
		options: defaults().apply(opts...),

		// catalogs
		catalog: catalog,
		local:   nil,

		// auto update state
		autoUpdatesEnabled: true,
		autoUpdateInterval: constants.DefaultUpdateInterval,
		autoUpdateFunc:     nil,
		updateTicker:       time.NewTicker(constants.DefaultUpdateInterval),
		stopCh:             make(chan struct{}),
		updateCancel:       nil,

		// hooks
		hooks: newHooks(),
	}

	// create the local catalog either from path or embedded
	log := logging.Debug()
	log.Msg("Creating local catalog (embedded or file-based)")
	if sm.local, err = catalogs.NewLocal(sm.options.localPath); err != nil {
		return nil, errors.WrapResource("create", "local catalog", sm.options.localPath, err)
	}

	// Get counts for logging
	localProviders := sm.local.Providers().List()
	localModels := sm.local.Models().List()
	log.Int("providers", len(localProviders)).
		Int("models", len(localModels)).
		Msg("Local catalog loaded")

	// Replace empty main catalog with local catalog immediately
	// This provides embedded data on startup instead of waiting for auto-update
	// Use ReplaceWith since sm.catalog is always empty at this point
	log.Msg("Replacing main catalog with local catalog")
	if err = sm.catalog.ReplaceWith(sm.local); err != nil {
		return nil, errors.WrapResource("replace", "main catalog with local catalog", "", err)
	}

	// Verify merge
	mainProviders := sm.catalog.Providers().List()
	mainModels := sm.catalog.Models().List()
	log.Int("providers", len(mainProviders)).
		Int("models", len(mainModels)).
		Msg("Main catalog after merge")

	// set the auto update state
	sm.autoUpdatesEnabled = sm.options.autoUpdatesEnabled
	sm.autoUpdateInterval = sm.options.autoUpdateInterval
	sm.autoUpdateFunc = sm.options.autoUpdateFunc

	// start auto-updates if enabled
	if sm.autoUpdatesEnabled {
		if err := sm.AutoUpdatesOn(); err != nil {
			return nil, errors.WrapResource("start", "auto-updates", "", err)
		}
	}

	return sm, nil
}
