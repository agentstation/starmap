package runtime

import (
	"context"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/reconciler"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// reconcileProviders uses the same field authority as explicit source acquisition.
// Provider receipts remain attached to their original payloads during reconciliation.
func (l *layerSet) reconcileProviders(ctx context.Context, base *catalogs.Catalog, published time.Time, active []providerEvidenceKey) (*catalogs.Builder, starmap.CandidateEvidence, error) {
	baseSource := sources.EmbeddedCatalogID
	if l.source != nil {
		baseSource = sources.ReleaseArtifactID
	}
	observations := []sources.Observation{{SourceID: baseSource, Catalog: base}}
	evidence := starmap.CandidateEvidence{}
	changedAt := published
	for _, key := range active {
		if err := ctx.Err(); err != nil {
			return nil, evidence, err
		}
		layer := l.providers[key]
		catalog, err := catalogs.DecodeSourceObservationPayload(layer.Payload)
		if err != nil {
			return nil, evidence, errors.WrapResource("decode", "retained provider observation", string(key.providerID), err)
		}
		observation, err := layer.Receipt.Restore(catalog)
		if err != nil {
			return nil, evidence, errors.WrapResource("restore", "retained provider receipt", string(key.providerID), err)
		}
		observations = append(observations, observation)
		evidence.SourceObservations = append(evidence.SourceObservations, observation.Link())
		if observation.ObservedAt.After(changedAt) {
			changedAt = observation.ObservedAt
		}
	}
	result, err := reconciler.ReconcileObservations(ctx, base, observations, reconciler.WithChangeTime(changedAt))
	if err != nil {
		return nil, evidence, err
	}
	if err := ctx.Err(); err != nil {
		return nil, evidence, err
	}
	evidence.ReviewCandidates = result.ReviewCandidates
	return result.Catalog, evidence, nil
}
