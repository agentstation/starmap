package starmap

import (
	"context"
	stderrors "errors"
	"time"

	"github.com/google/uuid"

	bootstraploader "github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// CurrentGeneration returns the exact immutable generation currently published
// by this client. The embedded bootstrap is returned before durable mutation.
func (c *Client) CurrentGeneration(ctx context.Context) (catalogstore.Generation, error) {
	if c == nil {
		return catalogstore.Generation{}, &errors.ValidationError{Field: "starmap.client", Message: "is required"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.RLock()
	id := c.generationID
	embedded := c.usingEmbeddedBootstrap
	c.mu.RUnlock()
	if id != "" {
		if err := c.requireWritableCatalogStore(); err != nil {
			return catalogstore.Generation{}, err
		}
		return c.options.catalogStore.Get(ctx, id)
	}
	if embedded {
		return bootstraploader.Generation()
	}
	return catalogstore.Generation{}, &errors.NotFoundError{Resource: "current catalog generation", ID: "current"}
}

// Generation returns one retained immutable generation by ID.
func (c *Client) Generation(ctx context.Context, id string) (catalogstore.Generation, error) {
	if c == nil {
		return catalogstore.Generation{}, &errors.ValidationError{Field: "starmap.client", Message: "is required"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if c.options != nil && !isNilCatalogStore(c.options.catalogStore) {
		generation, err := c.options.catalogStore.Get(ctx, id)
		if err == nil || !stderrors.Is(err, errors.ErrNotFound) {
			return generation, err
		}
	}
	c.mu.RLock()
	embeddedID := c.embeddedBootstrap.GenerationID
	c.mu.RUnlock()
	if id == embeddedID {
		return bootstraploader.Generation()
	}
	return catalogstore.Generation{}, &errors.NotFoundError{Resource: "catalog generation", ID: id}
}

const (
	generationValidatorVersion = "starmap-v1"
	generationValidationCheck  = "catalog_publication"
	customUpdateSourceID       = catalogmeta.SourceID("custom_update")
)

func (c *Client) commitAndPublish(
	ctx context.Context,
	published *catalogs.Catalog,
	observations []sources.Observation,
) (pipeline.Publication, error) {
	if err := c.requireWritableCatalogStore(); err != nil {
		return pipeline.Publication{}, err
	}
	generation, err := c.newGeneration(published, observations)
	if err != nil {
		return pipeline.Publication{}, err
	}

	c.mu.RLock()
	expectedGenerationID := c.generationID
	c.mu.RUnlock()
	if err := c.options.catalogStore.Commit(ctx, generation, expectedGenerationID); err != nil {
		return pipeline.Publication{}, errors.WrapResource(
			"commit",
			"catalog generation",
			generation.Manifest.GenerationID,
			err,
		)
	}

	oldCatalog := c.swapCatalogGeneration(published, generation.Manifest.GenerationID)
	sequence := c.CurrentCatalogState().Sequence
	event := CatalogPublishedEvent{
		GenerationID: generation.Manifest.GenerationID,
		SyncRunID:    generation.Manifest.SyncRunID,
		Sequence:     sequence,
		Catalog:      published,
	}
	c.hooks.dispatchUpdate(oldCatalog, published, event)
	return pipeline.Publication{
		GenerationID: event.GenerationID,
		SyncRunID:    event.SyncRunID,
	}, nil
}

func (c *Client) commitReceivedGeneration(ctx context.Context, published *catalogs.Catalog, generation catalogstore.Generation) error {
	if err := c.requireWritableCatalogStore(); err != nil {
		return err
	}
	if published == nil {
		return &errors.ValidationError{Field: "remote catalog", Message: "decoded catalog is required"}
	}
	if err := generation.Validate(); err != nil {
		return err
	}
	c.mu.RLock()
	expectedGenerationID := c.generationID
	c.mu.RUnlock()
	if err := c.options.catalogStore.Commit(ctx, generation, expectedGenerationID); err != nil {
		return errors.WrapResource("commit", "remote catalog generation", generation.Manifest.GenerationID, err)
	}
	// CatalogStore commits are idempotent so callers can safely retry an
	// ambiguous response. Do not turn an identical retry into a second in-memory
	// publication, sequence, or event.
	if expectedGenerationID == generation.Manifest.GenerationID {
		return nil
	}
	oldCatalog := c.swapCatalogGeneration(published, generation.Manifest.GenerationID)
	sequence := c.CurrentCatalogState().Sequence
	event := CatalogPublishedEvent{
		GenerationID: generation.Manifest.GenerationID,
		SyncRunID:    generation.Manifest.SyncRunID,
		Sequence:     sequence,
		Catalog:      published,
	}
	c.hooks.dispatchUpdate(oldCatalog, published, event)
	return nil
}

func (c *Client) newGeneration(published *catalogs.Catalog, sourceObservations []sources.Observation) (catalogstore.Generation, error) {
	payload, err := catalogstore.EncodeCatalogPayload(published)
	if err != nil {
		return catalogstore.Generation{}, err
	}
	descriptor := catalogs.DescribeCatalogPayload(payload)
	generationID, err := c.nextID()
	if err != nil {
		return catalogstore.Generation{}, err
	}
	syncRunID, err := c.nextID()
	if err != nil {
		return catalogstore.Generation{}, err
	}
	generatedAt := c.currentTime()
	observations := make([]catalogs.SourceObservationLink, 0, len(sourceObservations))
	completeness := catalogs.GenerationCompletenessComplete
	degraded := false
	degradationReasons := make([]string, 0)
	for _, observation := range sourceObservations {
		if err := observation.Validate(); err != nil {
			return catalogstore.Generation{}, errors.WrapResource("validate", "source observation", observation.ID, err)
		}
		observations = append(observations, observation.Link())
		if observation.Completeness == sources.ObservationCompletenessPartial {
			completeness = catalogs.GenerationCompletenessPartial
		}
		if observation.Status == sources.ObservationStatusDegraded {
			degraded = true
			degradationReasons = append(degradationReasons, "source "+observation.SourceID.String()+" observation is degraded")
		}
	}
	generation := catalogstore.Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.CurrentGenerationManifestVersion,
			SchemaVersion:   catalogs.CurrentCatalogSchemaVersion,
			GenerationID:    generationID,
			GeneratedAt:     generatedAt,
			Payload:         descriptor,
			Validation: catalogs.GenerationValidationReport{
				ValidatorVersion: generationValidatorVersion,
				ValidatedAt:      generatedAt,
				Status:           catalogs.GenerationValidationPassed,
				Checks: []catalogs.GenerationValidationCheck{{
					Name:   generationValidationCheck,
					Status: catalogs.GenerationValidationCheckPassed,
				}},
			},
			SyncRunID:          syncRunID,
			SourceObservations: observations,
			Completeness:       completeness,
			Degraded:           degraded,
			DegradationReasons: degradationReasons,
			ConsumerCompatibility: catalogs.ConsumerCompatibility{
				MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
				MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
			},
		},
		Payload: payload,
	}
	if err := generation.Validate(); err != nil {
		return catalogstore.Generation{}, err
	}
	return generation, nil
}

func (c *Client) nextID() (string, error) {
	if c.newID != nil {
		return c.newID()
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return "", errors.WrapIO("generate", "catalog generation ID", err)
	}
	return id.String(), nil
}

func (c *Client) currentTime() time.Time {
	if c.now != nil {
		return c.now().UTC()
	}
	return time.Now().UTC()
}
