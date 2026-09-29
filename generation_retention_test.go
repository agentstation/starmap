package starmap

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestGenerationRetainerIsPassiveAndFailurePreventsCommit(t *testing.T) {
	for _, operation := range []string{"update", "activate", "rollback"} {
		t.Run(operation, func(t *testing.T) {
			store := storage.NewMemory()
			target := rootRemoteGeneration(t)
			if err := store.Commit(t.Context(), target, ""); err != nil {
				t.Fatal(err)
			}
			current := target.Copy()
			current.Manifest.GenerationID += "-current"
			if err := store.Commit(t.Context(), current, target.Manifest.GenerationID); err != nil {
				t.Fatal(err)
			}
			calls := 0
			client, err := New(WithCatalogStore(store), WithGenerationRetainer(func(context.Context, catalogs.Generation) error { calls++; return fs.ErrPermission }))
			if err != nil {
				t.Fatal(err)
			}
			_ = client.Catalog()
			if calls != 0 {
				t.Fatal("passive read called retention")
			}
			switch operation {
			case "update":
				_, err = client.Update(t.Context(), func(_ context.Context, c *catalogs.Catalog) (*Candidate, error) {
					return NewCandidate(c, CandidateEvidence{}, WithCandidateGenerationID("retainer-update"))
				})
			case "activate":
				_, err = client.Activate(t.Context(), target)
			case "rollback":
				_, err = client.Rollback(t.Context(), target.Manifest.GenerationID)
			}
			if !errors.Is(err, fs.ErrPermission) || calls != 1 {
				t.Fatalf("retention failure: calls=%d err=%v", calls, err)
			}
			after, err := store.Current(t.Context())
			if err != nil || after.Manifest.GenerationID != current.Manifest.GenerationID {
				t.Fatal("failed retention changed durable catalog")
			}
		})
	}
	if _, err := New(WithGenerationRetainer(nil)); err == nil {
		t.Fatal("nil retainer accepted")
	}
}

func TestGenerationRetainerRetryAndPrivateCopy(t *testing.T) {
	store := storage.NewMemory()
	target := rootRemoteGeneration(t)
	calls := 0
	client, err := New(WithCatalogStore(store), WithGenerationRetainer(func(_ context.Context, g catalogs.Generation) error {
		calls++
		g.Payload[0] = '!'
		g.Manifest.GenerationID = "changed"
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := client.Activate(t.Context(), target); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || got.Manifest.GenerationID != target.Manifest.GenerationID || got.Validate() != nil {
		t.Fatal("retainer changed publication or exact retry bypassed retention")
	}
}

type rejectingRetainerCommitStore struct {
	storage.Store
	reject bool
}

func (s *rejectingRetainerCommitStore) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
	if s.reject {
		s.reject = false
		return fs.ErrPermission
	}
	return s.Store.Commit(ctx, generation, expected)
}

func TestGenerationRetainerDoesNotProveCommitSuccess(t *testing.T) {
	store := &rejectingRetainerCommitStore{Store: storage.NewMemory()}
	current := rootRemoteGeneration(t)
	if err := store.Commit(t.Context(), current, ""); err != nil {
		t.Fatal(err)
	}
	target := current.Copy()
	target.Manifest.GenerationID += "-next"
	var retained []catalogs.Generation
	client, err := New(WithCatalogStore(store), WithGenerationRetainer(func(_ context.Context, generation catalogs.Generation) error {
		retained = append(retained, generation)
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	store.reject = true
	if _, err = client.Activate(t.Context(), target); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("commit failure: %v", err)
	}
	if len(retained) != 1 || retained[0].Manifest.GenerationID != target.Manifest.GenerationID {
		t.Fatal("failed commit lost its prior retention attempt")
	}
	actual, err := store.Current(t.Context())
	if err != nil || actual.Manifest.GenerationID != current.Manifest.GenerationID {
		t.Fatal("failed commit changed the selected generation")
	}
	if _, err = client.Activate(t.Context(), target); err != nil {
		t.Fatal(err)
	}
	actual, err = store.Current(t.Context())
	if err != nil || actual.Manifest.GenerationID != target.Manifest.GenerationID || len(retained) != 2 {
		t.Fatal("exact retry failed to retain and publish the same generation")
	}
}
