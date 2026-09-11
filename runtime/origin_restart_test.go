package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestOriginReplicaStartupRetainsCatalogWithoutInputs(t *testing.T) {
	store := storage.NewMemory()
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	base := []Option{WithCatalogSource("embedded"), WithAcquisitionEnabled(false), WithSourcePollInterval(0)}
	leader := openTestRuntime(t, append(base, WithProviderBindings(*layer.Receipt.ProviderBinding), WithAuthorityOrigin(store, originTestConfig()))...)
	if _, err := leader.publishProviders(t.Context(), []ProviderLayer{layer}, leader.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	accepted := leader.State()
	if err := leader.Close(); err != nil {
		t.Fatal(err)
	}
	for _, configured := range []bool{true, false} {
		name := "without bindings"
		if configured {
			name = "with bindings"
		}
		t.Run(name, func(t *testing.T) {
			watched := &originFollowerStore{Store: store, AuthorityHeadReader: store}
			leases := &stubLeaseStore{}
			options := append(base, WithAuthorityOrigin(watched, originTestConfig()), WithLeaseStore(leases), withScheduleTimer(newStubScheduleTimer().after))
			if configured {
				options = append(options, WithProviderBindings(*layer.Receipt.ProviderBinding))
			}
			follower := openTestRuntime(t, options...)
			if follower.State().AuthorityHead != accepted.AuthorityHead || follower.State().PayloadChecksum != accepted.PayloadChecksum {
				t.Fatal("fresh replica replaced the accepted catalog without its retained inputs")
			}
			if leases.acquireCount() != 0 || watched.commits.Load() != 0 {
				t.Fatal("fresh replica tried to own publication before it recovered accepted inputs")
			}
		})
	}
}

func TestOriginReplicaStartupTakesLeaseWithMatchingInputs(t *testing.T) {
	store := storage.NewMemory()
	leader := openTestRuntime(t, WithCatalogSource("embedded"), WithAuthorityOrigin(store, originTestConfig()))
	accepted := leader.State()
	if err := leader.Close(); err != nil {
		t.Fatal(err)
	}
	leases := &stubLeaseStore{}
	owner := openTestRuntime(t, WithCatalogSource("embedded"), WithAuthorityOrigin(store, originTestConfig()), WithLeaseStore(leases), withScheduleTimer(newStubScheduleTimer().after))
	if owner.State().AuthorityHead != accepted.AuthorityHead || owner.lease.status() != leaseHeld || leases.acquireCount() != 1 {
		t.Fatal("matching replica failed to take ownership without replacing the accepted generation")
	}
}
