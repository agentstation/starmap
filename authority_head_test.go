package starmap

import (
	"context"
	"io/fs"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

type authorityHeadReader interface {
	CurrentAuthorityHead() catalogs.CatalogAuthorityHead
}

type authorityReadCountingStore struct {
	*storage.Memory
	reads  atomic.Int64
	reject atomic.Bool
}

func (s *authorityReadCountingStore) Current(ctx context.Context) (catalogs.Generation, error) {
	s.reads.Add(1)
	return s.Memory.Current(ctx)
}

func (s *authorityReadCountingStore) Get(ctx context.Context, id string) (catalogs.Generation, error) {
	s.reads.Add(1)
	return s.Memory.Get(ctx, id)
}

func (s *authorityReadCountingStore) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
	if s.reject.Load() {
		return fs.ErrPermission
	}
	return s.Memory.Commit(ctx, generation, expected)
}

func TestCurrentAuthorityHeadReadsPublishedStateWithoutStorage(t *testing.T) {
	store := &authorityReadCountingStore{Memory: storage.NewMemory()}
	client, err := New(WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	reader, ok := any(client).(authorityHeadReader)
	if !ok {
		t.Fatal("the client cannot read its authority head without the catalog payload")
	}
	if any((*Client)(nil)).(authorityHeadReader).CurrentAuthorityHead() != (catalogs.CatalogAuthorityHead{}) {
		t.Fatal("nil client returned an authority head")
	}
	if reader.CurrentAuthorityHead() != (catalogs.CatalogAuthorityHead{}) {
		t.Fatal("the embedded catalog invented an authority head")
	}
	if client.CurrentCatalogState().AuthorityHead != (catalogs.CatalogAuthorityHead{}) || (*Client)(nil).CurrentCatalogState().AuthorityHead != (catalogs.CatalogAuthorityHead{}) {
		t.Fatal("an absent authority produced a snapshot head")
	}
	generation, err := client.CurrentGeneration(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	generation.Manifest.GenerationID = "authority-one"
	generation.Manifest.ManifestVersion = catalogs.AuthorityGenerationManifestVersion
	generation.Manifest.AuthorityHead = catalogs.CatalogAuthorityHead{
		AuthorityID: "enterprise", PolicyID: "production", Sequence: 1,
		GenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum,
		RequiredPermissionRevision: "sha256:" + strings.Repeat("a", 64),
		PermissionSchemaVersion:    catalogs.CatalogPermissionSchemaVersion,
	}
	if _, err := client.Activate(t.Context(), generation); err != nil {
		t.Fatal(err)
	}
	before := store.reads.Load()
	allocations := testing.AllocsPerRun(100, func() {
		if reader.CurrentAuthorityHead() != generation.Manifest.AuthorityHead {
			t.Fatal("the published authority head differs from its generation")
		}
		state := client.CurrentCatalogState()
		if state.AuthorityHead != generation.Manifest.AuthorityHead || state.AuthorityHead.GenerationID != state.GenerationID || state.AuthorityHead.PayloadChecksum != state.PayloadChecksum {
			t.Fatal("the catalog snapshot lost its committed authority head")
		}
	})
	if store.reads.Load() != before || allocations != 0 {
		t.Fatalf("head read: storage reads=%d allocations=%v", store.reads.Load()-before, allocations)
	}
	store.reject.Store(true)
	if _, err := client.Update(t.Context(), func(_ context.Context, current *catalogs.Catalog) (*Candidate, error) {
		return NewCandidate(current, CandidateEvidence{})
	}); err == nil || reader.CurrentAuthorityHead() != generation.Manifest.AuthorityHead {
		t.Fatal("failed publication replaced the committed authority head")
	}
	if client.CurrentCatalogState().AuthorityHead != generation.Manifest.AuthorityHead {
		t.Fatal("failed publication replaced the snapshot authority head")
	}
	store.reject.Store(false)
	if _, err := client.Update(t.Context(), func(_ context.Context, current *catalogs.Catalog) (*Candidate, error) {
		return NewCandidate(current, CandidateEvidence{})
	}); err != nil {
		t.Fatal(err)
	}
	if reader.CurrentAuthorityHead() != (catalogs.CatalogAuthorityHead{}) {
		t.Fatal("ordinary publication retained an earlier authority head")
	}
	if client.CurrentCatalogState().AuthorityHead != (catalogs.CatalogAuthorityHead{}) {
		t.Fatal("ordinary snapshot retained an earlier authority head")
	}
	if _, err := client.Rollback(t.Context(), generation.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	if reader.CurrentAuthorityHead() != generation.Manifest.AuthorityHead {
		t.Fatal("rollback lost the retained authority head")
	}
	if client.CurrentCatalogState().AuthorityHead != generation.Manifest.AuthorityHead {
		t.Fatal("rollback lost the snapshot authority head")
	}
	reopened, err := New(WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	if any(reopened).(authorityHeadReader).CurrentAuthorityHead() != generation.Manifest.AuthorityHead {
		t.Fatal("construction lost the stored authority head")
	}
	if reopened.CurrentCatalogState().AuthorityHead != generation.Manifest.AuthorityHead {
		t.Fatal("construction lost the snapshot authority head")
	}
}

func TestCatalogSnapshotAuthorityRemainsBoundDuringPublication(t *testing.T) {
	client, err := New(WithCatalogStore(storage.NewMemory()))
	if err != nil {
		t.Fatal(err)
	}
	var generations []catalogs.Generation
	for i, name := range []string{"first", "second"} {
		builder := catalogs.NewEmpty()
		if err := builder.SetAuthor(catalogs.Author{ID: catalogs.AuthorID(name), Name: name}); err != nil {
			t.Fatal(err)
		}
		catalog, err := builder.Build()
		if err != nil {
			t.Fatal(err)
		}
		candidate, err := NewCandidate(catalog, CandidateEvidence{}, WithCandidateGenerationID(name))
		if err != nil {
			t.Fatal(err)
		}
		generation, err := client.PrepareGeneration(t.Context(), candidate)
		if err != nil {
			t.Fatal(err)
		}
		generation.Manifest.ManifestVersion = catalogs.AuthorityGenerationManifestVersion
		generation.Manifest.AuthorityHead = catalogs.CatalogAuthorityHead{
			AuthorityID: "enterprise", PolicyID: "production", Sequence: uint64(i + 1),
			GenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum,
			RequiredPermissionRevision: "sha256:" + strings.Repeat("a", 64),
			PermissionSchemaVersion:    catalogs.CatalogPermissionSchemaVersion,
		}
		generations = append(generations, generation)
	}
	if _, err := client.Activate(t.Context(), generations[0]); err != nil {
		t.Fatal(err)
	}
	retained := client.CurrentCatalogState()
	var workers sync.WaitGroup
	start := make(chan struct{})
	workers.Go(func() {
		<-start
		for i := range 32 {
			if _, err := client.Activate(t.Context(), generations[i%len(generations)]); err != nil {
				t.Error(err)
				return
			}
		}
	})
	for range 4 {
		workers.Go(func() {
			<-start
			for range 128 {
				state := client.CurrentCatalogState()
				payload, err := catalogs.EncodeCatalogPayload(state.Catalog)
				if err != nil {
					t.Error(err)
					return
				}
				if state.AuthorityHead.GenerationID != state.GenerationID || state.AuthorityHead.PayloadChecksum != state.PayloadChecksum || catalogs.DescribeCatalogPayload(payload).Checksum != state.PayloadChecksum {
					t.Error("concurrent snapshot mixed catalog and authority publications")
					return
				}
			}
		})
	}
	close(start)
	workers.Wait()
	if retained.AuthorityHead != generations[0].Manifest.AuthorityHead || retained.GenerationID != generations[0].Manifest.GenerationID {
		t.Fatal("later publication changed a retained snapshot")
	}
}
