package permission

import (
	"context"
	"reflect"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// PublisherConfig fixes the authority and policy that own a publication store.
type PublisherConfig struct {
	AuthorityID string
	PolicyID    string
}

// Publisher enforces authority publication order through the store's atomic compare-and-swap.
// Every writer for this authority must use the same publication contract.
// Direct underlying writes, deleted state, and restored older backups require separate recovery qualification.
// The caller owns the store and authorizes the publisher. Read methods retain the underlying storage contract.
type Publisher struct {
	store  storage.Store
	config PublisherConfig
}

// NewPublisher selects a store and fixed authority identity without I/O or background activity.
func NewPublisher(store storage.Store, config PublisherConfig) (*Publisher, error) {
	if nilPublicationStore(store) {
		return nil, publicationError("catalog store is required")
	}
	if err := catalogs.ValidateCatalogAuthorityIdentity(config.AuthorityID, config.PolicyID); err != nil {
		return nil, err
	}
	return &Publisher{store: store, config: config}, nil
}

// Current reads the selected generation from the caller's store.
func (p *Publisher) Current(ctx context.Context) (catalogs.Generation, error) {
	if err := p.ready(ctx); err != nil {
		return catalogs.Generation{}, err
	}
	return p.store.Current(ctx)
}

// Get reads one immutable generation from the caller's store.
func (p *Publisher) Get(ctx context.Context, id string) (catalogs.Generation, error) {
	if err := p.ready(ctx); err != nil {
		return catalogs.Generation{}, err
	}
	return p.store.Get(ctx, id)
}

// Commit publishes a supported authority generation after validating its durable predecessor.
// It permits the first authority in an empty store and exact idempotent retries.
// An existing ordinary catalog requires explicit Bootstrap. A different authority requires a separate transition.
func (p *Publisher) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
	return p.commit(ctx, generation, expected, false)
}

// Bootstrap explicitly adopts an ordinary catalog store for this authority.
// Expected identifies the exact predecessor. An empty value requires an empty store.
// It cannot replace an established authority, except for an exact retry of its current generation.
func (p *Publisher) Bootstrap(ctx context.Context, generation catalogs.Generation, expected string) error {
	return p.commit(ctx, generation, expected, true)
}

func (p *Publisher) commit(ctx context.Context, generation catalogs.Generation, expected string, bootstrap bool) error {
	if err := p.ready(ctx); err != nil {
		return err
	}
	if err := generation.Validate(); err != nil {
		return err
	}
	next := generation.Manifest.AuthorityHead
	if err := p.validateHead(next); err != nil {
		return err
	}
	if !next.SupportsPermissions() {
		return publicationError("candidate requires unsupported permission semantics")
	}
	current, err := p.store.Current(ctx)
	if err != nil {
		// The empty expectation lets the atomic store reject an unreadable existing pointer.
		if errors.IsNotFound(err) && expected == "" {
			return p.store.Commit(ctx, generation, "")
		}
		return err
	}
	if err := p.validatePredecessor(current, generation, expected, bootstrap); err != nil {
		return err
	}
	return p.store.Commit(ctx, generation, expected)
}

func (p *Publisher) validatePredecessor(current, next catalogs.Generation, expected string, bootstrap bool) error {
	currentID := current.Manifest.GenerationID
	if currentID != expected && currentID != next.Manifest.GenerationID {
		return &errors.ConflictError{Resource: "catalog authority publication", Expected: expected, Actual: currentID}
	}
	head := current.Manifest.AuthorityHead
	if head == (catalogs.CatalogAuthorityHead{}) {
		if !bootstrap {
			return publicationError("an existing ordinary catalog requires explicit bootstrap")
		}
		return nil
	}
	if err := p.validateHead(head); err != nil {
		return err
	}
	if !head.SupportsPermissions() {
		return publicationError("current authority requires unsupported permission semantics")
	}
	if bootstrap && currentID != next.Manifest.GenerationID {
		return publicationError("bootstrap cannot replace an established authority")
	}
	return head.ValidateSuccessor(next.Manifest.AuthorityHead)
}

// CurrentAuthorityHead observes independent current metadata when the underlying store supports that guarantee.
// It validates the selected authority identity but retains unknown positive permission versions for refusal diagnostics.
func (p *Publisher) CurrentAuthorityHead(ctx context.Context) (catalogs.CatalogAuthorityHead, error) {
	if err := p.ready(ctx); err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	reader, ok := p.store.(storage.AuthorityHeadReader)
	if !ok {
		return catalogs.CatalogAuthorityHead{}, publicationError("store has no current authority observation capability")
	}
	head, err := reader.CurrentAuthorityHead(ctx)
	if err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	if err := p.validateHead(head); err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	return head, nil
}

func (p *Publisher) validateHead(head catalogs.CatalogAuthorityHead) error {
	if err := head.Validate(); err != nil {
		return err
	}
	if head.AuthorityID != p.config.AuthorityID || head.PolicyID != p.config.PolicyID {
		return publicationError("generation does not match the configured authority and policy")
	}
	return nil
}

func (p *Publisher) ready(ctx context.Context) error {
	if p == nil || p.store == nil || ctx == nil {
		return publicationError("a constructed publisher and context are required")
	}
	return ctx.Err()
}

func publicationError(message string) error {
	return &errors.ConfigError{Component: "authority publisher", Message: message}
}

func nilPublicationStore(store storage.Store) bool {
	value := reflect.ValueOf(store)
	if !value.IsValid() {
		return true
	}
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

var (
	_ storage.Store               = (*Publisher)(nil)
	_ storage.AuthorityHeadReader = (*Publisher)(nil)
)
