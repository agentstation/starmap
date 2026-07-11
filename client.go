// Package starmap provides the main entry point for the Starmap AI model catalog system.
// It offers a high-level interface for managing AI model catalogs with explicit synchronization,
// event hooks, and provider synchronization capabilities.
//
// Starmap wraps the underlying catalog system with additional features including:
// - Explicit, idempotent synchronization with provider APIs
// - Event hooks for model changes (added, updated, removed)
// - Thread-safe access to an immutable canonical catalog
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
//	// Register event hooks
//	sm.OnModelAdded(func(model catalogs.Model) {
//	    log.Printf("New model: %s", model.ID)
//	})
//
//	// Get the current immutable catalog
//	catalog := sm.Catalog()
//
//	model, err := catalog.FindModel("gpt-4o")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Manually trigger a dry run (read-only; no store required)
//	result, err := sm.Sync(ctx, sync.WithProvider("openai"), sync.WithDryRun(true))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Configure mutation with an explicit writable generation store
//	store, err := catalogstore.NewFilesystem("./catalog-store")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	sm, err = starmap.New(
//	    WithCatalogStore(store),
//	    WithLocalPath("./custom-catalog"),
//	)
package starmap

import (
	"context"
	stderrors "errors"
	"sync"
	"time"

	bootstraploader "github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// Catalog returns the current immutable canonical catalog.
func (c *Client) Catalog() *catalogs.Catalog {
	c.mu.RLock()
	catalog := c.catalog
	c.mu.RUnlock()
	return catalog
}

// CatalogState atomically pairs the current immutable catalog with its logical
// generation identity for generation-scoped caches and responses.
type CatalogState struct {
	Catalog      *catalogs.Catalog
	GenerationID string
	Sequence     uint64
}

// CurrentCatalogState returns one atomic catalog/generation pair.
func (c *Client) CurrentCatalogState() CatalogState {
	if c == nil {
		return CatalogState{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	id := c.generationID
	if id == "" && c.usingEmbeddedBootstrap {
		id = c.embeddedBootstrap.GenerationID
	}
	return CatalogState{Catalog: c.catalog, GenerationID: id, Sequence: c.generationSequence}
}

// CurrentGenerationID returns the logical identity of the currently published
// catalog. Before the first durable mutation, this is the embedded bootstrap ID.
func (c *Client) CurrentGenerationID() string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.generationID != "" {
		return c.generationID
	}
	if c.usingEmbeddedBootstrap {
		return c.embeddedBootstrap.GenerationID
	}
	return ""
}

func (c *Client) catalogCopy() (*catalogs.Builder, error) {
	return catalogs.NewBuilderFrom(c.Catalog())
}

func (c *Client) requireWritableCatalogStore() error {
	if c == nil || c.options == nil || isNilCatalogStore(c.options.catalogStore) {
		return &errors.ConfigError{
			Component: "catalog store",
			Message:   "an explicit writable store is required for catalog mutation",
		}
	}
	return nil
}

func snapshotBuilder(builder *catalogs.Builder) (*catalogs.Catalog, error) {
	if builder == nil {
		return nil, &errors.ValidationError{
			Field:   "catalog",
			Message: "catalog builder cannot be nil",
		}
	}
	snapshot, err := builder.Build()
	if err != nil {
		return nil, errors.WrapResource("publish", "catalog snapshot", "", err)
	}
	return snapshot, nil
}

// Client manages an immutable canonical catalog, explicit synchronization,
// persistence, and event hooks. It owns no scheduling goroutine or cadence.
type Client struct {

	// options are the configured options for the client
	options *options

	// catalog is the atomically published immutable generation.
	mu                     sync.RWMutex
	catalog                *catalogs.Catalog
	updates                updateCoordinator
	generationID           string
	generationSequence     uint64
	usingEmbeddedBootstrap bool
	embeddedBootstrap      catalogs.BootstrapManifest
	now                    func() time.Time
	newID                  func() (string, error)

	hooks *hooks // Event hooks for catalog changes/updates
}

// New creates a new Client instance with the given options.
func New(opts ...Option) (*Client, error) {

	// apply options
	options, err := defaults().apply(opts...)
	if err != nil {
		return nil, err
	}

	// create the client instance
	sm := &Client{
		// options
		options: options,

		// hooks
		hooks: newHooks(),
	}

	// Load and verify the embedded bootstrap before any optional local overlay.
	log := logging.Debug()
	log.Msg("Creating local catalog (embedded or file-based)")
	localPath := sm.options.localPath
	if sm.options.embeddedCatalogEnabled {
		localPath = ""
	}
	embeddedBuilder, err := catalogs.NewEmbedded()
	if err != nil {
		return nil, errors.WrapResource("create", "embedded bootstrap catalog", "", err)
	}
	embeddedCatalog, err := embeddedBuilder.Build()
	if err != nil {
		return nil, errors.WrapResource("publish", "embedded bootstrap catalog", "", err)
	}
	bootstrapManifest, err := bootstraploader.Load(embeddedCatalog)
	if err != nil {
		return nil, err
	}
	initial := embeddedCatalog
	generationID := ""
	usingEmbeddedBootstrap := true
	if !isNilCatalogStore(sm.options.catalogStore) {
		loadCtx, cancel := context.WithTimeout(context.Background(), constants.DefaultTimeout)
		stored, currentErr := sm.options.catalogStore.Current(loadCtx)
		cancel()
		switch {
		case currentErr == nil:
			if err := stored.Validate(); err != nil {
				return nil, errors.WrapResource("validate", "stored current catalog generation", stored.Manifest.GenerationID, err)
			}
			initial, err = catalogstore.DecodeCatalogPayload(stored.Payload)
			if err != nil {
				return nil, errors.WrapResource("decode", "stored current catalog generation", stored.Manifest.GenerationID, err)
			}
			generationID = stored.Manifest.GenerationID
			usingEmbeddedBootstrap = false
		case stderrors.Is(currentErr, errors.ErrNotFound):
			// A newly configured store has no durable generation yet; the verified
			// embedded/local baseline remains active until the first commit. Local
			// YAML is deliberately consulted only in this empty-store case: once a
			// durable current exists it is the authoritative restart source.
		default:
			return nil, errors.WrapResource("load", "stored current catalog generation", "current", currentErr)
		}
	}
	if generationID == "" && localPath != "" {
		local, localErr := catalogs.NewLocal(localPath)
		if localErr != nil {
			return nil, errors.WrapResource("create", "local catalog", localPath, localErr)
		}
		initial, err = local.Build()
		if err != nil {
			return nil, errors.WrapResource("publish", "initial catalog", localPath, err)
		}
		usingEmbeddedBootstrap = false
	}
	sm.catalog = initial
	sm.generationID = generationID
	sm.generationSequence = 1
	sm.usingEmbeddedBootstrap = usingEmbeddedBootstrap
	sm.embeddedBootstrap = bootstrapManifest

	// Get counts for logging
	localProviders := initial.Providers().List()
	localModels := initial.Definitions()
	log.Int("providers", len(localProviders)).
		Int("models", len(localModels)).
		Msg("Local catalog loaded")

	log.Msg("Published initial catalog generation")

	return sm, nil
}
