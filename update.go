package starmap

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	catalogevidence "github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/errors"
)

// CandidateEvidence binds one publication candidate to its immutable source
// observations and excluded model review candidates.
type CandidateEvidence struct {
	SourceObservations []catalogs.SourceObservationLink
	ReviewCandidates   []catalogevidence.ReviewCandidate
}

// Candidate is a complete immutable catalog prepared off to the side for one
// atomic publication. Evidence does not alter catalog facts.
type Candidate struct {
	catalog      *catalogs.Catalog
	evidence     CandidateEvidence
	generationID string
}

// CandidateOption configures one publication candidate above its catalog and
// its evidence.
type CandidateOption func(*Candidate) error

// WithCandidateGenerationID binds the candidate to a generation ID that the
// caller derives. A caller that composes a catalog from its own layers knows
// the identity of the result. The update then publishes that identity instead
// of a fresh one.
//
// The identity names one payload. A retained generation never changes. A later
// candidate that carries the same identity and other bytes therefore fails
// with a typed conflict from the catalog store. A caller that omits this
// option gets one fresh UUID-shaped identity per publication.
func WithCandidateGenerationID(id string) CandidateOption {
	return func(candidate *Candidate) error {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			return &errors.ValidationError{
				Field:   "candidate.generation_id",
				Message: "is required",
			}
		}
		candidate.generationID = trimmed
		return nil
	}
}

// NewCandidate validates and returns a publication candidate. Custom acquisition
// can omit evidence. Client.Update records a deterministic
// custom-update observation in that case.
func NewCandidate(
	catalog *catalogs.Catalog,
	evidence CandidateEvidence,
	opts ...CandidateOption,
) (*Candidate, error) {
	if catalog == nil {
		return nil, &errors.ValidationError{
			Field:   "candidate.catalog",
			Message: "is required",
		}
	}
	for _, observation := range evidence.SourceObservations {
		if err := observation.Validate(); err != nil {
			return nil, errors.WrapResource(
				"validate",
				"candidate source observation",
				observation.ObservationID,
				err,
			)
		}
	}
	reviewCandidates := append([]catalogevidence.ReviewCandidate(nil), evidence.ReviewCandidates...)
	slices.SortFunc(reviewCandidates, catalogevidence.CompareReviewCandidates)
	if len(reviewCandidates) > 0 {
		if err := catalogs.ValidateReviewCandidates(reviewCandidates, evidence.SourceObservations); err != nil {
			return nil, errors.WrapResource(
				"validate",
				"candidate review candidates",
				"",
				err,
			)
		}
	}
	candidate := &Candidate{
		catalog: catalog,
		evidence: CandidateEvidence{
			SourceObservations: append([]catalogs.SourceObservationLink(nil), evidence.SourceObservations...),
			ReviewCandidates:   reviewCandidates,
		},
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(candidate); err != nil {
			return nil, err
		}
	}
	return candidate, nil
}

// UpdateFunc builds and validates a complete candidate while Client.Update
// holds the client's mutation transaction. A nil result does not publish.
// The current catalog is immutable and safe to retain.
type UpdateFunc func(context.Context, *catalogs.Catalog) (*Candidate, error)

// Publication identifies the durable generation produced by a successful
// update. Published is false when the update function returns no candidate or
// reactivates an identical retained generation.
type Publication struct {
	Published       bool
	GenerationID    string
	PayloadChecksum string
	SyncRunID       string
}

// Update serializes candidate construction, generation-store CAS, and atomic
// in-memory publication. Acquisition and scheduling remain explicit caller
// composition above Client.
func (c *Client) Update(ctx context.Context, update UpdateFunc) (Publication, error) {
	if c == nil {
		return Publication{}, &errors.ValidationError{
			Field:   "starmap.client",
			Message: "is required",
		}
	}
	if update == nil {
		return Publication{}, &errors.ValidationError{
			Field:   "starmap.update",
			Message: "update function is required",
		}
	}
	if err := c.requireWritableCatalogStore(); err != nil {
		return Publication{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	release, err := c.updates.acquire(ctx)
	if err != nil {
		return Publication{}, err
	}
	defer release()

	candidate, err := update(ctx, c.Catalog())
	if err != nil {
		return Publication{}, err
	}
	if candidate == nil {
		return Publication{}, nil
	}
	return c.commitAndPublish(ctx, candidate.catalog, candidate.evidence, candidate.generationID)
}

// Activate validates, durably commits, and atomically activates an immutable
// generation obtained by an explicit trusted distribution adapter.
func (c *Client) Activate(ctx context.Context, generation catalogs.Generation) (Publication, error) {
	if c == nil {
		return Publication{}, &errors.ValidationError{
			Field:   "starmap.client",
			Message: "is required",
		}
	}
	if err := c.requireWritableCatalogStore(); err != nil {
		return Publication{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if generation.Manifest.SchemaVersion != catalogs.CurrentCatalogSchemaVersion ||
		!generation.Manifest.ConsumerCompatibility.SupportsSchema(
			catalogs.CurrentCatalogSchemaVersion,
		) {
		return Publication{}, &errors.ValidationError{
			Field:   "catalog_generation.schema_version",
			Value:   generation.Manifest.SchemaVersion,
			Message: "is not compatible with this Starmap catalog schema",
		}
	}
	release, err := c.updates.acquire(ctx)
	if err != nil {
		return Publication{}, err
	}
	defer release()

	published, err := catalogs.DecodeCatalogPayload(generation.Payload)
	if err != nil {
		return Publication{}, errors.WrapResource(
			"decode",
			"catalog generation",
			generation.Manifest.GenerationID,
			err,
		)
	}
	return c.commitReceivedGeneration(ctx, published, generation)
}

func (c *Client) swapCatalogGeneration(
	published *catalogs.Catalog,
	generationID string,
	payloadChecksum string,
	generatedAt time.Time,
) (*catalogs.Catalog, uint64) {
	c.mu.Lock()
	oldCatalog := c.catalog
	c.catalog = published
	c.usingEmbeddedBootstrap = false
	c.generationSequence++
	sequence := c.generationSequence
	if generationID != "" {
		c.generationID = generationID
	}
	c.generationPayloadChecksum = payloadChecksum
	c.generationGeneratedAt = generatedAt
	c.mu.Unlock()
	return oldCatalog, sequence
}
