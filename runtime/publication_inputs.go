package runtime

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// inputChanges stages only the retained inputs that one accepted generation replaces.
type inputChanges struct {
	source    *sourceLayer
	providers []ProviderLayer
	manual    *manualBatch
	removals  *catalogs.CatalogRemovalPolicy
}

func (c inputChanges) empty() bool {
	return c.source == nil && len(c.providers) == 0 && c.manual == nil && c.removals == nil
}

func (c inputChanges) stage(ctx context.Context, store *layerStore, record inputPublication) (inputPublication, error) {
	var err error
	if c.source != nil {
		record.Source, err = store.stageInput(ctx, c.source)
		if err != nil {
			return inputPublication{}, err
		}
	}
	for _, layer := range c.providers {
		name, err := store.stageInput(ctx, layer)
		if err != nil {
			return inputPublication{}, err
		}
		record.Providers = append(record.Providers, name)
	}
	if c.manual != nil {
		record.Manual, err = store.stageManualBatch(ctx, c.manual)
		if err != nil {
			return inputPublication{}, err
		}
		c.manual.reference = record.Manual
	}
	if c.removals != nil {
		record.Removals, err = store.stageInput(ctx, removalPolicyRecord{Version: removalPolicyVersion, Policy: *c.removals})
		if err != nil {
			return inputPublication{}, err
		}
	}
	return record, nil
}

func (c inputChanges) complete(ctx context.Context, store *layerStore, record inputPublication) error {
	if c.removals != nil {
		return store.completeInputPublication(ctx, record, c.source, c.providers, c.removals)
	}
	return store.completeInputPublication(ctx, record, c.source, c.providers)
}
