package runtime

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// providerInventoryOmitsRecords preserves original evidence for omitted offerings.
// A complete inventory changes observed availability without deleting visible records.
func (l *layerSet) providerInventoryOmitsRecords(ctx context.Context, incoming []ProviderLayer) (bool, error) {
	for _, layer := range incoming {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		prior, exists := l.providers[layer.evidenceKey()]
		if !exists {
			continue
		}
		previous, err := catalogs.DecodeSourceObservationPayload(prior.Payload)
		if err != nil {
			return false, err
		}
		next, err := catalogs.DecodeSourceObservationPayload(layer.Payload)
		if err != nil {
			return false, err
		}
		oldProvider, oldExists := previous.Providers().Get(layer.ProviderID)
		newProvider, newExists := next.Providers().Get(layer.ProviderID)
		if !oldExists || !newExists {
			return false, invalidProviderEvidence("provider_id", "inventory does not contain the declared provider")
		}
		for modelID := range oldProvider.Models {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			if _, exists := newProvider.Models[modelID]; !exists {
				return true, nil
			}
		}
	}
	return false, nil
}
