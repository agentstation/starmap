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
