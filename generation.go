package starmap

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	stderrors "errors"
	"time"

	bootstraploader "github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
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
	observations []catalogs.SourceObservationLink,
) (Publication, error) {
	if err := c.requireWritableCatalogStore(); err != nil {
		return Publication{}, err
	}
	generation, err := c.newGeneration(published, observations)
	if err != nil {
		return Publication{}, err
	}

	c.mu.RLock()
	expectedGenerationID := c.generationID
	c.mu.RUnlock()
	if err := c.options.catalogStore.Commit(ctx, generation, expectedGenerationID); err != nil {
		return Publication{}, errors.WrapResource(
			"commit",
			"catalog generation",
			generation.Manifest.GenerationID,
			err,
		)
	}

	c.publishCommittedGeneration(published, generation)
	return Publication{
		Published:       true,
		GenerationID:    generation.Manifest.GenerationID,
		PayloadChecksum: generation.Manifest.Payload.Checksum,
		SyncRunID:       generation.Manifest.SyncRunID,
	}, nil
}

func (c *Client) commitReceivedGeneration(
	ctx context.Context,
	published *catalogs.Catalog,
	generation catalogstore.Generation,
) (Publication, error) {
	if err := c.requireWritableCatalogStore(); err != nil {
		return Publication{}, err
	}
	if published == nil {
		return Publication{}, &errors.ValidationError{Field: "catalog generation", Message: "decoded catalog is required"}
	}
	if err := generation.Validate(); err != nil {
		return Publication{}, err
	}
	c.mu.RLock()
	expectedGenerationID := c.generationID
	c.mu.RUnlock()
	if err := c.options.catalogStore.Commit(ctx, generation, expectedGenerationID); err != nil {
		return Publication{}, errors.WrapResource("commit", "catalog generation", generation.Manifest.GenerationID, err)
	}
	// CatalogStore commits are idempotent so callers can safely retry an
	// ambiguous response. Do not turn an identical retry into a second in-memory
	// publication, sequence, or event.
	if expectedGenerationID == generation.Manifest.GenerationID {
		return Publication{
			GenerationID:    generation.Manifest.GenerationID,
			PayloadChecksum: generation.Manifest.Payload.Checksum,
			SyncRunID:       generation.Manifest.SyncRunID,
		}, nil
	}
	c.publishCommittedGeneration(published, generation)
	return Publication{
		Published:       true,
		GenerationID:    generation.Manifest.GenerationID,
		PayloadChecksum: generation.Manifest.Payload.Checksum,
		SyncRunID:       generation.Manifest.SyncRunID,
	}, nil
}

func (c *Client) publishCommittedGeneration(published *catalogs.Catalog, generation catalogstore.Generation) {
	oldCatalog := c.swapCatalogGeneration(published, generation.Manifest.GenerationID)
	sequence := c.CurrentCatalogState().Sequence
	event := CatalogPublishedEvent{
		GenerationID: generation.Manifest.GenerationID,
		SyncRunID:    generation.Manifest.SyncRunID,
		Sequence:     sequence,
		Catalog:      published,
	}
	c.hooks.dispatchUpdate(oldCatalog, published, event)
}

func (c *Client) newGeneration(
	published *catalogs.Catalog,
	sourceObservations []catalogs.SourceObservationLink,
) (catalogstore.Generation, error) {
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
	observations := append([]catalogs.SourceObservationLink(nil), sourceObservations...)
	if len(observations) == 0 {
		observations = append(observations, catalogs.SourceObservationLink{
			Source:        customUpdateSourceID,
			ObservationID: "observation:" + syncRunID,
			ObservedAt:    generatedAt,
			Revision: catalogmeta.ObservationRevision{
				Kind:  catalogmeta.ObservationRevisionKindContentDigest,
				Value: descriptor.Checksum,
			},
			Completeness:     catalogmeta.ObservationCompletenessComplete,
			Status:           catalogmeta.ObservationStatusSucceeded,
			EvidenceChecksum: descriptor.Checksum,
		})
	}
	completeness := catalogs.GenerationCompletenessComplete
	degraded := false
	degradationReasons := make([]string, 0)
	for _, observation := range observations {
		if err := observation.Validate(); err != nil {
			return catalogstore.Generation{}, errors.WrapResource(
				"validate",
				"source observation",
				observation.ObservationID,
				err,
			)
		}
		if observation.Completeness == catalogmeta.ObservationCompletenessPartial {
			completeness = catalogs.GenerationCompletenessPartial
		}
		if observation.Status == catalogmeta.ObservationStatusDegraded {
			degraded = true
			degradationReasons = append(
				degradationReasons,
				"source "+observation.Source.String()+" observation is degraded",
			)
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
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", errors.WrapIO("generate", "catalog generation ID", err)
	}
	// RFC 4122 version 4 / variant 1 bits preserve the established UUID-shaped
	// generation identifier without importing an acquisition-unrelated module.
	random[6] = (random[6] & 0x0f) | 0x40
	random[8] = (random[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(random[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" +
		encoded[16:20] + "-" + encoded[20:32], nil
}

func (c *Client) currentTime() time.Time {
	if c.now != nil {
		return c.now().UTC()
	}
	return time.Now().UTC()
}
