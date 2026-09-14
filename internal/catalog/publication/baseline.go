package publication

import (
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	catalogruntime "github.com/agentstation/starmap/runtime"
)

func selectPublicationBaseline(state *State, baseline catalogs.Generation) (*State, bool, error) {
	if baseline.Manifest.AuthorityHead != (catalogs.CatalogAuthorityHead{}) {
		return nil, false, admissionError("baseline", "cannot publish an authoritative enterprise catalog")
	}
	next, err := catalogs.DecodeCatalogGeneration(baseline)
	if err != nil {
		return nil, false, err
	}
	current, err := catalogs.DecodeCatalogGeneration(state.current)
	if err != nil {
		return nil, false, err
	}
	if err := current.CanonicalAliases().ValidateSuccessor(next.CanonicalAliases()); err != nil {
		return nil, false, err
	}
	digest, err := baseline.SemanticChecksum()
	if err != nil {
		return nil, false, err
	}
	for _, previous := range []catalogs.Generation{state.baseline, state.current} {
		prior, err := previous.SemanticChecksum()
		if err != nil {
			return nil, false, err
		}
		if prior == digest {
			return state, false, nil
		}
	}
	selected := *state
	selected.baseline = baseline.Copy()
	return &selected, true, nil
}

func publicationCollectionBaseline(ctx context.Context, state *State, bindings []sources.ProviderAcquisitionBinding, history []sources.Observation, runID string) (*catalogs.Catalog, error) {
	candidate, err := catalogruntime.ReplayAcquisition(ctx, state.baseline, state.publisherID, bindings, history)
	if err != nil {
		return nil, err
	}
	generation, err := candidate.Generation(runID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return catalogs.DecodeCatalogGeneration(generation)
}
