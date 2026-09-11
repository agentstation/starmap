package runtime

import (
	"context"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

type originFollowerStore struct {
	storage.Store
	storage.AuthorityHeadReader
	commits atomic.Int64
	reads   atomic.Int64
}

func (s *originFollowerStore) Current(ctx context.Context) (catalogs.Generation, error) {
	s.reads.Add(1)
	return s.Store.Current(ctx)
}

func (s *originFollowerStore) Commit(context.Context, catalogs.Generation, string) error {
	s.commits.Add(1)
	return &errors.ConflictError{Resource: "follower store", Message: "follower must not publish"}
}

func TestOriginFollowerStartsWithoutPublishing(t *testing.T) {
	for _, backend := range []string{"memory", "filesystem"} {
		t.Run(backend, func(t *testing.T) {
			var store storage.Store = storage.NewMemory()
			if backend == "filesystem" {
				var err error
				store, err = storage.NewFilesystem(filepath.Join(t.TempDir(), "catalog-store"))
				if err != nil {
					t.Fatal(err)
				}
			}
			layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
			base := []Option{WithCatalogSource("embedded"), WithAcquisitionEnabled(false), WithSourcePollInterval(0), WithProviderBindings(*layer.Receipt.ProviderBinding)}
			leader := openTestRuntime(t, append(base, WithStateDirectory(privateRuntimeDirectory(t)), WithAuthorityOrigin(store, originTestConfig()))...)
			before, err := store.Current(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			watched := &originFollowerStore{Store: store, AuthorityHeadReader: store.(storage.AuthorityHeadReader)}
			follower := openTestRuntime(t, append(base, WithStateDirectory(privateRuntimeDirectory(t)), WithAuthorityOrigin(watched, originTestConfig()), WithLeaseStore(&stubLeaseStore{refuseAll: true}))...)
			if follower.State().AuthorityHead != leader.State().AuthorityHead || follower.State().PayloadChecksum != leader.State().PayloadChecksum {
				t.Fatal("follower did not select the accepted authority catalog")
			}
			receipt, err := follower.ReadPermission(t.Context())
			if err != nil || receipt.Head != before.Manifest.AuthorityHead {
				t.Fatalf("follower receipt: %v", err)
			}
			if _, err := follower.RefreshSource(t.Context()); err == nil {
				t.Fatal("follower acquired without the publication lease")
			}
			if watched.commits.Load() != 0 {
				t.Fatal("follower tried to publish")
			}
			after, err := store.Current(t.Context())
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("follower changed shared state")
			}
			if _, err := leader.publishProviders(t.Context(), []ProviderLayer{layer}, leader.lease.epoch()); err != nil {
				t.Fatal(err)
			}
			latest, err := store.Current(t.Context())
			if err != nil || latest.Manifest.AuthorityHead.Sequence <= before.Manifest.AuthorityHead.Sequence {
				t.Fatalf("leader did not advance the authority: %v", err)
			}
			receipt, err = follower.ReadPermission(t.Context())
			if err != nil || receipt.Head != latest.Manifest.AuthorityHead {
				t.Fatalf("follower issued permission for stale state: %v", err)
			}
			if watched.commits.Load() != 0 {
				t.Fatal("permission issuance tried to publish")
			}
		})
	}
}

func TestOriginFollowerRefusesMissingOrWrongAuthority(t *testing.T) {
	for _, mode := range []string{"empty", "wrong-authority", "wrong-policy"} {
		t.Run(mode, func(t *testing.T) {
			store := storage.NewMemory()
			base := []Option{WithCatalogSource("embedded"), WithAcquisitionEnabled(false), WithSourcePollInterval(0)}
			if mode != "empty" {
				openTestRuntime(t, append(base, WithStateDirectory(privateRuntimeDirectory(t)), WithAuthorityOrigin(store, originTestConfig()))...)
			}
			config := originTestConfig()
			if mode == "wrong-authority" {
				config.AuthorityID = "other"
			}
			if mode == "wrong-policy" {
				config.PolicyID = "other"
			}
			watched := &originFollowerStore{Store: store, AuthorityHeadReader: store}
			follower, err := Open(t.Context(), append(base, WithStateDirectory(privateRuntimeDirectory(t)), WithAuthorityOrigin(watched, config), WithLeaseStore(&stubLeaseStore{refuseAll: true}))...)
			if follower != nil {
				_ = follower.Close()
			}
			if err == nil {
				t.Fatal("follower accepted an absent or different authority")
			}
			if watched.commits.Load() != 0 {
				t.Fatal("refused follower tried to publish")
			}
		})
	}
}

func TestOriginFollowerAdoptsSharedCatalogWithoutAcquisition(t *testing.T) {
	store := storage.NewMemory()
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	base := []Option{WithCatalogSource("embedded"), WithAcquisitionEnabled(false), WithSourcePollInterval(0), WithProviderBindings(*layer.Receipt.ProviderBinding)}
	leader := openTestRuntime(t, append(base, WithAuthorityOrigin(store, originTestConfig()))...)
	before := leader.State()
	watched := &originFollowerStore{Store: store, AuthorityHeadReader: store}
	leases := &stubLeaseStore{refuseAll: true}
	ticks := make(chan time.Time)
	waits := make(chan time.Duration, 1)
	follower := openTestRuntime(t, append(base, WithAuthorityOrigin(watched, originTestConfig()), WithLeaseStore(leases), withScheduleTimer(func(delay time.Duration) <-chan time.Time {
		select {
		case waits <- delay:
		default:
		}
		return ticks
	}))...)
	select {
	case delay := <-waits:
		if delay <= 0 || delay > 30*time.Second {
			t.Fatalf("accepted-store poll delay = %s", delay)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("origin follower has no accepted-store refresh schedule")
	}
	if _, err := leader.publishProviders(t.Context(), []ProviderLayer{layer}, leader.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	latest := leader.State()
	if latest.GenerationID == before.GenerationID {
		t.Fatal("leader did not publish a new catalog")
	}
	select {
	case ticks <- time.Now():
	case <-time.After(5 * time.Second):
		t.Fatal("follower did not wait for its poll")
	}
	eventually(t, 30*time.Second, "follower did not adopt the shared head", func() bool {
		return follower.State().AuthorityHead == latest.AuthorityHead
	})
	if follower.client.CurrentCatalogState().AuthorityHead != latest.AuthorityHead || follower.State().PayloadChecksum != latest.PayloadChecksum {
		t.Fatal("follower client and serving state disagree")
	}
	select {
	case update := <-follower.Updates():
		if update.AuthorityHead != latest.AuthorityHead {
			t.Fatal("follower announced a different authority")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("follower did not announce its accepted generation")
	}
	if watched.commits.Load() != 0 || leases.acquireCount() != 1 {
		t.Fatal("accepted-store refresh attempted publication or lease acquisition")
	}
	reads := watched.reads.Load()
	if err := follower.refreshOriginAccepted(t.Context()); err != nil {
		t.Fatal(err)
	}
	if watched.reads.Load() != reads {
		t.Fatal("unchanged authority caused a full catalog read")
	}
	// Missing source inputs cannot become a new publication after lease availability changes.
	leases.mu.Lock()
	leases.refuseAll = false
	leases.mu.Unlock()
	if _, err := follower.RefreshSource(t.Context()); !errors.IsConflict(err) {
		t.Fatalf("takeover without retained inputs = %v, want conflict", err)
	}
	if leases.acquireCount() != 1 || watched.commits.Load() != 0 {
		t.Fatal("a follower with missing inputs tried to take acquisition ownership")
	}
}

func TestOriginFollowerWithMatchingInputsCanTakeOwnership(t *testing.T) {
	store := storage.NewMemory()
	leader := openTestRuntime(t, WithCatalogSource("embedded"), WithAuthorityOrigin(store, originTestConfig()))
	before := leader.State()
	source := newStubSource("takeover-source")
	source.replies = []SourceRead{testSourceRead(t, "takeover-generation", testCatalogPayload(t, "takeover-provider", "model", "Takeover model"), time.Now().UTC())}
	leases := &stubLeaseStore{refuseAll: true}
	follower := openTestRuntime(t, WithSource(source), WithAuthorityOrigin(store, originTestConfig()), WithLeaseStore(leases), withScheduleTimer(newStubScheduleTimer().after))
	if err := leader.Close(); err != nil {
		t.Fatal(err)
	}
	leases.mu.Lock()
	leases.refuseAll = false
	leases.mu.Unlock()
	if _, err := follower.RefreshSource(t.Context()); err != nil {
		t.Fatalf("takeover with matching inputs: %v", err)
	}
	current, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if current.Manifest.AuthorityHead.Sequence != before.AuthorityHead.Sequence+1 || follower.State().AuthorityHead != current.Manifest.AuthorityHead {
		t.Fatal("takeover lost publication order or serving identity")
	}
	if source.readCount() != 1 || leases.acquireCount() != 2 {
		t.Fatal("eligible takeover did not acquire once and read its configured source")
	}
}
