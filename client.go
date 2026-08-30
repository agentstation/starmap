package starmap

import (
	"context"
	stderrors "errors"
	"os"
	"sync"
	"time"

	bootstraploader "github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// Catalog returns the current immutable canonical catalog. It returns nil when
// called on a nil Client. After New or NewContext succeeds, Catalog is
// non-failing, non-nil, O(1), allocation-free, and safe to retain across
// goroutines.
func (c *Client) Catalog() *catalogs.Catalog {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	catalog := c.catalog
	c.mu.RUnlock()
	return catalog
}

// CatalogState holds one atomic snapshot. It pairs the current immutable
// catalog with its generation identity, checksum, timestamp, and local sequence.
type CatalogState struct {
	Catalog         *catalogs.Catalog
	GenerationID    string
	PayloadChecksum string
	GeneratedAt     time.Time
	Sequence        uint64
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
	return CatalogState{
		Catalog:         c.catalog,
		GenerationID:    id,
		PayloadChecksum: c.generationPayloadChecksum,
		GeneratedAt:     c.generationGeneratedAt,
		Sequence:        c.generationSequence,
	}
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

// WorkspacePath returns the configured human-editable YAML workspace. An
// empty path means this client has no filesystem projection.
func (c *Client) WorkspacePath() string {
	if c == nil || c.options == nil {
		return ""
	}
	return c.options.catalogPath
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

// Client manages an immutable canonical catalog, explicit publication,
// persistence, and event hooks. It owns no provider acquisition, scheduling
// goroutine, or cadence.
type Client struct {

	// options are the configured options for the client
	options *options

	// catalog is the atomically published immutable generation.
	mu                        sync.RWMutex
	catalog                   *catalogs.Catalog
	updates                   updateCoordinator
	generationID              string
	generationPayloadChecksum string
	generationGeneratedAt     time.Time
	generationSequence        uint64
	usingEmbeddedBootstrap    bool
	embeddedBootstrap         catalogs.BootstrapManifest
	now                       func() time.Time
	newID                     func() (string, error)

	hooks *hooks // Event hooks for catalog changes/updates
}

// New creates a Client using a background context. Call NewContext when the
// caller must cancel storage I/O during client setup.
func New(opts ...Option) (*Client, error) {
	return NewContext(context.Background(), opts...)
}

// NewContext creates a Client with the given options. The caller-owned context
// bounds durable generation loading and workspace repair and must be non-nil.
// When a durable generation and a marker-backed unchanged YAML workspace are
// both configured, construction repairs a stale or interrupted projection by
// digest. It never overwrites an unrecognized semantic workspace change.
func NewContext(ctx context.Context, opts ...Option) (*Client, error) {
	if ctx == nil {
		return nil, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := constructionContextError(ctx); err != nil {
		return nil, err
	}

	// apply options
	options, err := defaults().apply(opts...)
	if err != nil {
		return nil, err
	}
	if err := validateCatalogLayout(options.catalogStore, options.catalogPath); err != nil {
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
	catalogPath := sm.options.catalogPath
	embeddedCatalog, bootstrapManifest, err := bootstraploader.Embedded()
	if err != nil {
		return nil, err
	}
	if err := constructionContextError(ctx); err != nil {
		return nil, err
	}
	initial := embeddedCatalog
	generationID := ""
	generationPayloadChecksum := bootstrapManifest.Payload.Checksum
	generationGeneratedAt := bootstrapManifest.GeneratedAt
	usingEmbeddedBootstrap := true
	var durableCurrent *catalogs.Generation
	if !isNilCatalogStore(sm.options.catalogStore) {
		loadCtx, cancel := context.WithTimeout(ctx, catalogLoadTimeout)
		stored, currentErr := sm.options.catalogStore.Current(loadCtx)
		cancel()
		switch {
		case currentErr == nil:
			if err := stored.Validate(); err != nil {
				return nil, errors.WrapResource("validate", "stored current catalog generation", stored.Manifest.GenerationID, err)
			}
			initial, err = catalogs.DecodeCatalogPayload(stored.Payload)
			if err != nil {
				return nil, errors.WrapResource("decode", "stored current catalog generation", stored.Manifest.GenerationID, err)
			}
			durableCurrent = &stored
			generationID = stored.Manifest.GenerationID
			generationPayloadChecksum = stored.Manifest.Payload.Checksum
			generationGeneratedAt = stored.Manifest.GeneratedAt
			usingEmbeddedBootstrap = false
		case stderrors.Is(currentErr, errors.ErrNotFound):
			// A newly configured store has no durable generation yet. The verified
			// embedded/local baseline remains active until the first commit. Local
			// YAML is deliberately consulted only in this empty-store case: once a
			// durable current exists it is the authoritative restart source.
		default:
			return nil, errors.WrapResource("load", "stored current catalog generation", "current", currentErr)
		}
	}
	if generationID == "" && catalogPath != "" {
		human, humanErr := catalogs.NewFromPath(catalogPath)
		switch {
		case humanErr == nil:
			initial, err = human.Build()
			if err != nil {
				return nil, errors.WrapResource("publish", "initial human catalog", catalogPath, err)
			}
			generationGeneratedAt = time.Time{}
			payload, encodeErr := catalogs.EncodeCatalogPayload(initial)
			if encodeErr != nil {
				return nil, errors.WrapResource("encode", "initial human catalog", catalogPath, encodeErr)
			}
			generationPayloadChecksum = catalogs.DescribeCatalogPayload(payload).Checksum
			usingEmbeddedBootstrap = false
		case stderrors.Is(humanErr, os.ErrNotExist):
		default:
			return nil, errors.WrapResource("create", "human catalog workspace", catalogPath, humanErr)
		}
	}
	if err := constructionContextError(ctx); err != nil {
		return nil, err
	}
	sm.catalog = initial
	sm.generationID = generationID
	sm.generationPayloadChecksum = generationPayloadChecksum
	sm.generationGeneratedAt = generationGeneratedAt
	sm.generationSequence = 1
	sm.usingEmbeddedBootstrap = usingEmbeddedBootstrap
	sm.embeddedBootstrap = bootstrapManifest

	if durableCurrent != nil && catalogPath != "" {
		repairCtx, cancel := context.WithTimeout(ctx, catalogProjectionTimeout)
		repair, repairErr := workspace.Repair(
			repairCtx,
			catalogPath,
			initial,
			workspace.Identity{
				GenerationID:    durableCurrent.Manifest.GenerationID,
				PayloadChecksum: durableCurrent.Manifest.Payload.Checksum,
			},
		)
		cancel()
		if err := constructionContextError(ctx); err != nil {
			return nil, err
		}
		switch {
		case repairErr != nil:
			logging.Warn().
				Err(repairErr).
				Str("generation_id", durableCurrent.Manifest.GenerationID).
				Str("workspace", catalogPath).
				Msg("Durable catalog is active; YAML workspace repair remains pending")
		case repair.Status == workspace.RepairStatusSkippedDirty:
			logging.Warn().
				Str("generation_id", durableCurrent.Manifest.GenerationID).
				Str("workspace", catalogPath).
				Str("issue_code", repair.IssueCode).
				Msg("Durable catalog is active; YAML workspace has semantic human changes and was not overwritten")
		case repair.Status == workspace.RepairStatusRepaired:
			logging.Info().
				Str("generation_id", durableCurrent.Manifest.GenerationID).
				Str("workspace", catalogPath).
				Msg("Repaired YAML workspace from durable catalog generation")
		}
	}

	// Get counts for logging
	localProviders := initial.Providers().List()
	localModels := initial.Definitions()
	log.Int("providers", len(localProviders)).
		Int("models", len(localModels)).
		Msg("Local catalog loaded")

	log.Msg("Published initial catalog generation")

	return sm, nil
}

func constructionContextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.WrapResource("construct", "starmap client", "", err)
	}
	return nil
}
