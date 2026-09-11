package runtime

import (
	"context"
	"io/fs"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

type originAdoptionStore struct {
	*storage.Memory
	selected atomic.Pointer[catalogs.Generation]
	failRead atomic.Bool
	writes   atomic.Int64
}

func (s *originAdoptionStore) Current(ctx context.Context) (catalogs.Generation, error) {
	if s.failRead.Load() {
		return catalogs.Generation{}, fs.ErrPermission
	}
	if selected := s.selected.Load(); selected != nil {
		return selected.Copy(), nil
	}
	return s.Memory.Current(ctx)
}

func (s *originAdoptionStore) CurrentAuthorityHead(ctx context.Context) (catalogs.CatalogAuthorityHead, error) {
	if selected := s.selected.Load(); selected != nil {
		return selected.Manifest.AuthorityHead, nil
	}
	return s.Memory.CurrentAuthorityHead(ctx)
}

func (s *originAdoptionStore) Commit(context.Context, catalogs.Generation, string) error {
	s.writes.Add(1)
	return fs.ErrPermission
}

func TestOriginFollowerRejectsInvalidSharedCatalog(t *testing.T) {
	for _, mode := range []string{"authority", "policy", "replay", "permission schema", "catalog schema", "read failure", "corrupt payload"} {
		t.Run(mode, func(t *testing.T) {
			source, _ := authorityRuntimeFixture(t)
			first := source.replies[0].Generation.Copy()
			first.Manifest.AuthorityHead.Sequence = 2
			store := &originAdoptionStore{Memory: storage.NewMemory()}
			if err := store.Memory.Commit(t.Context(), first, ""); err != nil {
				t.Fatal(err)
			}
			leases := &stubLeaseStore{refuseAll: true}
			follower := openTestRuntime(t, WithCatalogSource("embedded"), WithAuthorityOrigin(store, originTestConfig()), WithLeaseStore(leases), withScheduleTimer(newStubScheduleTimer().after))
			before := follower.State()
			next := first.Copy()
			next.Manifest.GenerationID += "-next"
			next.Manifest.AuthorityHead.GenerationID = next.Manifest.GenerationID
			next.Manifest.AuthorityHead.Sequence = 3
			switch mode {
			case "authority":
				next.Manifest.AuthorityHead.AuthorityID = "other"
			case "policy":
				next.Manifest.AuthorityHead.PolicyID = "other"
			case "replay":
				next.Manifest.AuthorityHead.Sequence = 1
			case "permission schema":
				next.Manifest.AuthorityHead.PermissionSchemaVersion++
			case "catalog schema":
				next.Manifest.SchemaVersion++
				next.Manifest.ConsumerCompatibility.MinSchemaVersion = next.Manifest.SchemaVersion
				next.Manifest.ConsumerCompatibility.MaxSchemaVersion = next.Manifest.SchemaVersion
			case "read failure":
				store.failRead.Store(true)
			case "corrupt payload":
				next.Payload[0] = 0
			}
			store.selected.Store(&next)
			if err := follower.refreshOriginAccepted(t.Context()); err == nil {
				t.Fatal("follower activated an invalid shared generation")
			}
			if follower.State() != before || follower.client.CurrentCatalogState().AuthorityHead != before.AuthorityHead {
				t.Fatal("failed adoption changed the retained catalog")
			}
			if store.writes.Load() != 0 || leases.acquireCount() != 1 {
				t.Fatal("failed adoption wrote storage or acquired the lease")
			}
		})
	}
}
