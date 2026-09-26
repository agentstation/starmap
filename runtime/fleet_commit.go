package runtime

import (
	"context"
	"encoding/json/v2"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

// fleetCommitStore adapts the root client's publication to one fleet transaction.
// A caller cannot write through this adapter without a runtime-owned commit context.
type fleetCommitStore struct {
	FleetStore
}

type fleetCommitContextKey struct{}

// fleetCommit retains one attempted publication across an ambiguous backend response.
// Its expected head and ownership grant never advance to make a retry succeed.
type fleetCommit struct {
	store    *fleetCommitStore
	grant    Lease
	expected FleetHead
	data     []byte
	checksum string

	mu              sync.Mutex
	accepted        FleetHead
	attemptChecksum string
}

func (s *fleetCommitStore) prepare(ctx context.Context, grant Lease, expected FleetHead, layers layerSet) (context.Context, *fleetCommit, error) {
	if err := grant.validateFleet(); err != nil {
		return nil, nil, err
	}
	if err := expected.Validate(); err != nil {
		return nil, nil, err
	}
	data, err := encodeFleetRecovery(ctx, layers)
	if err != nil {
		return nil, nil, err
	}
	commit := &fleetCommit{store: s, grant: grant, expected: expected, data: data, checksum: fleetRecoveryChecksum(data)}
	return context.WithValue(ctx, fleetCommitContextKey{}, commit), commit, nil
}

func (s *fleetCommitStore) Current(ctx context.Context) (catalogs.Generation, error) {
	snapshot, err := s.CurrentPublication(ctx)
	if err != nil {
		return catalogs.Generation{}, err
	}
	if err := snapshot.Validate(); err != nil {
		return catalogs.Generation{}, err
	}
	return snapshot.Publication.Generation, nil
}

func (s *fleetCommitStore) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	commit, ok := ctx.Value(fleetCommitContextKey{}).(*fleetCommit)
	if !ok || commit == nil || commit.store != s {
		return fleetConflict("a runtime-owned publication context is required")
	}
	commit.mu.Lock()
	defer commit.mu.Unlock()
	if expected != commit.expected.GenerationID {
		return fleetConflict("the client predecessor differs from the recovered publication")
	}
	publication := FleetPublication{Generation: generation, Expected: commit.expected, Grant: commit.grant,
		Recovery: FleetRecovery{GenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum,
			Checksum: commit.checksum, Data: commit.data}}
	if err := publication.Validate(); err != nil {
		return err
	}
	manifest, err := json.Marshal(generation.Manifest, json.Deterministic(true))
	if err != nil {
		return err
	}
	checksum := fleetRecoveryChecksum(manifest)
	if commit.attemptChecksum != "" && commit.attemptChecksum != checksum {
		return fleetConflict("a publication retry changed its original generation")
	}
	commit.attemptChecksum = checksum
	accepted, err := s.CommitPublication(ctx, publication)
	if err != nil {
		return err
	}
	if accepted != publication.nextHead() {
		return fleetConflict("the backend returned a different accepted publication")
	}
	commit.accepted = accepted
	return nil
}

func (c *fleetCommit) result() FleetHead {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.accepted
}

func (s *fleetCommitStore) GenerationLeaser() (storage.GenerationLeaser, bool) {
	if provider, ok := s.FleetStore.(storage.GenerationLeaseProvider); ok {
		return provider.GenerationLeaser()
	}
	leaser, ok := s.FleetStore.(storage.GenerationLeaser)
	return leaser, ok
}

func (s *fleetCommitStore) GenerationCollector() (storage.GenerationCollector, bool) {
	if provider, ok := s.FleetStore.(storage.GenerationCollectionProvider); ok {
		return provider.GenerationCollector()
	}
	collector, ok := s.FleetStore.(storage.GenerationCollector)
	return collector, ok
}

var _ storage.Store = (*fleetCommitStore)(nil)
