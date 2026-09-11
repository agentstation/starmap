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
// catalog with its generation identity, checksum, timestamp, local sequence, and authority head.
type CatalogState struct {
	Catalog         *catalogs.Catalog
	GenerationID    string
	PayloadChecksum string
	GeneratedAt     time.Time
	Sequence        uint64
	// AuthorityHead identifies this catalog's committed authority generation.
	// Ordinary and embedded catalogs have a zero head. This field does not grant permission.
	AuthorityHead catalogs.CatalogAuthorityHead
}

// CurrentCatalogState returns one atomic catalog snapshot, including its authority head.
// It allocates no memory and reads no storage. Retained snapshots remain immutable.
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
		AuthorityHead:   c.generationAuthorityHead,
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

// PublishesDurably reports whether the client holds the explicit writable
// catalog store that durable publication needs. A client without one keeps
// every published generation in memory, and a restart loses it. The connected
// runtime reads this before it commits an effective generation.
func (c *Client) PublishesDurably() bool {
	return c.requireWritableCatalogStore() == nil
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
	generationAuthorityHead   catalogs.CatalogAuthorityHead
	generationGeneratedAt     time.Time
	generationSequence        uint64
	usingEmbeddedBootstrap    bool
	embeddedBootstrap         catalogs.BootstrapManifest
	embeddedCatalog           *catalogs.Catalog
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
// bounds reads from caller-supplied storage and must be non-nil.
// Construction never repairs or creates a workspace. Use RepairWorkspace for
// explicit repair, or open the connected runtime for application startup.
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
	return newClient(ctx, options)
}

// newClient builds the offline client from already applied options. It
// publishes the verified baseline that every client serves before its first
// durable generation.
func newClient(ctx context.Context, options *options) (*Client, error) {
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
	var generationAuthorityHead catalogs.CatalogAuthorityHead
	generationPayloadChecksum := bootstrapManifest.Payload.Checksum
	generationGeneratedAt := bootstrapManifest.GeneratedAt
	usingEmbeddedBootstrap := true
	if !isNilCatalogStore(sm.options.catalogStore) {
		loadCtx, cancel := context.WithTimeout(ctx, catalogLoadTimeout)
		stored, currentErr := sm.options.catalogStore.Current(loadCtx)
		cancel()
		switch {
		case currentErr == nil:
			if err := stored.Validate(); err != nil {
				return nil, errors.WrapResource("validate", "stored current catalog generation", stored.Manifest.GenerationID, err)
			}
			initial, err = catalogs.DecodeCatalogGeneration(stored)
			if err != nil {
				return nil, errors.WrapResource("decode", "stored current catalog generation", stored.Manifest.GenerationID, err)
			}
			generationID = stored.Manifest.GenerationID
			generationAuthorityHead = stored.Manifest.AuthorityHead
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
		err := workspace.Read(ctx, catalogPath, func(workspace.InputExpectation) error {
			human, humanErr := catalogs.NewFromPath(catalogPath)
			switch {
			case humanErr == nil:
				initial, err = human.Build()
				if err != nil {
					return errors.WrapResource("publish", "initial human catalog", catalogPath, err)
				}
				generationGeneratedAt = time.Time{}
				payload, encodeErr := catalogs.EncodeCatalogPayload(initial)
				if encodeErr != nil {
					return errors.WrapResource("encode", "initial human catalog", catalogPath, encodeErr)
				}
				generationPayloadChecksum = catalogs.DescribeCatalogPayload(payload).Checksum
				usingEmbeddedBootstrap = false
			case stderrors.Is(humanErr, os.ErrNotExist):
			default:
				return errors.WrapResource("create", "human catalog workspace", catalogPath, humanErr)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if err := constructionContextError(ctx); err != nil {
		return nil, err
	}
	sm.catalog = initial
	sm.generationID = generationID
	sm.generationAuthorityHead = generationAuthorityHead
	sm.generationPayloadChecksum = generationPayloadChecksum
	sm.generationGeneratedAt = generationGeneratedAt
	sm.generationSequence = 1
	sm.usingEmbeddedBootstrap = usingEmbeddedBootstrap
	sm.embeddedBootstrap = bootstrapManifest
	sm.embeddedCatalog = embeddedCatalog

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
