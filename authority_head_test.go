package starmap

import (
	"context"
	"io/fs"
	"strings"
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
	store.reject.Store(false)
	if _, err := client.Update(t.Context(), func(_ context.Context, current *catalogs.Catalog) (*Candidate, error) {
		return NewCandidate(current, CandidateEvidence{})
	}); err != nil {
		t.Fatal(err)
	}
	if reader.CurrentAuthorityHead() != (catalogs.CatalogAuthorityHead{}) {
		t.Fatal("ordinary publication retained an earlier authority head")
	}
	if _, err := client.Rollback(t.Context(), generation.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	if reader.CurrentAuthorityHead() != generation.Manifest.AuthorityHead {
		t.Fatal("rollback lost the retained authority head")
	}
	reopened, err := New(WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	if any(reopened).(authorityHeadReader).CurrentAuthorityHead() != generation.Manifest.AuthorityHead {
		t.Fatal("construction lost the stored authority head")
	}
}
