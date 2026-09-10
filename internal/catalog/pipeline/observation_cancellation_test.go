package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

type cancelingObservationSource struct {
	sources.Source
	cancel context.CancelFunc
}

func (s cancelingObservationSource) Observe(ctx context.Context, opts ...sources.Option) (sources.Observation, error) {
	observation, err := s.Source.Observe(ctx, opts...)
	s.cancel()
	return observation, err
}

func TestObserveSourceStopsAfterCollectorCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	source := cancelingObservationSource{
		Source: &lifecycleTestSource{id: sources.LocalCatalogID, catalog: asSnapshot(catalogs.NewEmpty())},
		cancel: cancel,
	}
	result := observeSource(ctx, source, nil)
	if result.observation != nil || !errors.Is(errors.Join(result.errs...), context.Canceled) {
		t.Fatalf("canceled source result = %+v", result)
	}
}
