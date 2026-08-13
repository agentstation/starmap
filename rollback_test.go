package starmap

import (
	"bytes"
	"context"
	stderrors "errors"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs"
	catalogprojection "github.com/agentstation/starmap/pkg/catalogs/projection"
	"github.com/agentstation/starmap/pkg/catalogstore"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestRollbackRestoresExactRetainedGenerationAndWorkspace(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/catalog"
	store := newRollbackTestStore()
	client := newRollbackClient(t, store, path)

	first := publishRollbackFixture(t, client, path, "First Name", sources.EmbeddedCatalogID)
	second := publishRollbackFixture(t, client, path, "Second Name", sources.ProvidersID)
	assertCatalogPayload(t, client.Catalog(), second.generation.Payload)
	beforeSequence := client.CurrentCatalogState().Sequence
	events := make(chan CatalogPublishedEvent, 2)
	client.OnCatalogPublished(func(event CatalogPublishedEvent) error {
		events <- event
		return nil
	})

	result, err := client.Rollback(context.Background(), first.generation.Manifest.GenerationID)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if result.FromGenerationID != second.generation.Manifest.GenerationID ||
		result.GenerationID != first.generation.Manifest.GenerationID ||
		result.PayloadChecksum != first.generation.Manifest.Payload.Checksum {
		t.Fatalf("rollback result = %#v", result)
	}
	if result.Sequence != beforeSequence+1 {
		t.Fatalf("rollback sequence = %d, want %d", result.Sequence, beforeSequence+1)
	}
	if result.Projection == nil ||
		result.Projection.Status != pkgsync.ProjectionStatusApplied ||
		result.Projection.GenerationID != first.generation.Manifest.GenerationID ||
		result.Projection.WorkspaceChecksum != first.workspaceChecksum {
		t.Fatalf(
			"rollback projection = %#v, want prior workspace checksum %q",
			result.Projection,
			first.workspaceChecksum,
		)
	}

	current, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if current.Manifest.GenerationID != first.generation.Manifest.GenerationID ||
		!bytes.Equal(current.Payload, first.generation.Payload) {
		t.Fatalf(
			"current generation = %q, want exact %q",
			current.Manifest.GenerationID,
			first.generation.Manifest.GenerationID,
		)
	}
	if retained, err := store.Get(context.Background(), second.generation.Manifest.GenerationID); err != nil {
		t.Fatalf("Get retained second generation: %v", err)
	} else if !bytes.Equal(retained.Payload, second.generation.Payload) {
		t.Fatal("rollback changed the retained later generation")
	}
	assertCatalogPayload(t, client.Catalog(), first.generation.Payload)
	assertWorkspacePayload(t, path, first.workspacePayload)

	select {
	case event := <-events:
		if event.GenerationID != first.generation.Manifest.GenerationID || event.Sequence != result.Sequence {
			t.Fatalf("rollback event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("rollback publication event was not delivered")
	}

	repeated, err := client.Rollback(context.Background(), first.generation.Manifest.GenerationID)
	if err != nil {
		t.Fatalf("repeat Rollback: %v", err)
	}
	if repeated.Sequence != result.Sequence {
		t.Fatalf("repeat sequence = %d, want unchanged %d", repeated.Sequence, result.Sequence)
	}
	select {
	case event := <-events:
		t.Fatalf("idempotent rollback emitted a second event: %#v", event)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRollbackCommitFailureLeavesCatalogAndWorkspaceUnchanged(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/catalog"
	store := newRollbackTestStore()
	client := newRollbackClient(t, store, path)
	first := publishRollbackFixture(t, client, path, "First Name", sources.EmbeddedCatalogID)
	second := publishRollbackFixture(t, client, path, "Second Name", sources.ProvidersID)
	assertCatalogPayload(t, client.Catalog(), second.generation.Payload)
	store.failNext(stderrors.New("injected rollback commit failure"))

	if _, err := client.Rollback(context.Background(), first.generation.Manifest.GenerationID); err == nil {
		t.Fatal("Rollback succeeded after injected commit failure")
	}
	current, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if current.Manifest.GenerationID != second.generation.Manifest.GenerationID {
		t.Fatalf(
			"current generation = %q, want %q",
			current.Manifest.GenerationID,
			second.generation.Manifest.GenerationID,
		)
	}
	assertCatalogPayload(t, client.Catalog(), second.generation.Payload)
	assertWorkspacePayload(t, path, second.workspacePayload)
}

func TestRollbackProjectionConflictKeepsCommittedGenerationAndHumanEdit(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/catalog"
	store := newRollbackTestStore()
	client := newRollbackClient(t, store, path)
	first := publishRollbackFixture(t, client, path, "First Name", sources.EmbeddedCatalogID)
	_ = publishRollbackFixture(t, client, path, "Second Name", sources.ProvidersID)
	store.afterNextCommit(func() {
		editRollbackWorkspaceName(t, path, "Human Edit During Rollback")
	})

	result, err := client.Rollback(context.Background(), first.generation.Manifest.GenerationID)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if result.Projection == nil ||
		result.Projection.Status != pkgsync.ProjectionStatusPendingRepair ||
		result.Projection.IssueCode != pkgsync.ProjectionIssueWorkspaceFailed {
		t.Fatalf("projection = %#v, want pending repair", result.Projection)
	}
	current, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if current.Manifest.GenerationID != first.generation.Manifest.GenerationID {
		t.Fatalf(
			"durable generation = %q, want committed rollback %q",
			current.Manifest.GenerationID,
			first.generation.Manifest.GenerationID,
		)
	}
	assertCatalogPayload(t, client.Catalog(), first.generation.Payload)
	assertRollbackWorkspaceName(t, path, "Human Edit During Rollback")
}

type rollbackTestStore struct {
	*catalogstore.Memory
	mu          sync.Mutex
	nextFailure error
	afterCommit func()
}

func newRollbackTestStore() *rollbackTestStore {
	return &rollbackTestStore{Memory: catalogstore.NewMemory()}
}

func newRollbackClient(t testing.TB, store catalogstore.Store, path string) *Client {
	t.Helper()
	opts, err := defaults().apply(WithCatalogStore(store), WithCatalogPath(path))
	if err != nil {
		t.Fatalf("apply options: %v", err)
	}
	return newWritableStoreTestClient(t, opts)
}

func (s *rollbackTestStore) Commit(
	ctx context.Context,
	generation catalogstore.Generation,
	expected string,
) error {
	s.mu.Lock()
	failure := s.nextFailure
	after := s.afterCommit
	s.nextFailure = nil
	s.afterCommit = nil
	s.mu.Unlock()
	if failure != nil {
		return &pkgerrors.IOError{Operation: "commit", Path: "rollback-test-store", Err: failure}
	}
	if err := s.Memory.Commit(ctx, generation, expected); err != nil {
		return err
	}
	if after != nil {
		after()
	}
	return nil
}

func (s *rollbackTestStore) failNext(err error) {
	s.mu.Lock()
	s.nextFailure = err
	s.mu.Unlock()
}

func (s *rollbackTestStore) afterNextCommit(fn func()) {
	s.mu.Lock()
	s.afterCommit = fn
	s.mu.Unlock()
}

func publishRollbackFixture(
	t testing.TB,
	client *Client,
	path, name string,
	source sources.ID,
) rollbackFixture {
	t.Helper()
	builder := rollbackFixtureCatalog(t, name, source)
	input, err := observeBoundWorkspaceInput(path)
	if err != nil {
		t.Fatalf("observeBoundWorkspaceInput: %v", err)
	}
	published := mustTestCatalog(t, builder)
	observation := persistenceObservation(t, builder)
	publication, err := client.Update(context.Background(), func(
		context.Context,
		*catalogs.Catalog,
	) (*Candidate, error) {
		return NewCandidate(published, CandidateEvidence{SourceObservations: []catalogs.SourceObservationLink{observation.Link()}})
	})
	if err != nil {
		t.Fatalf("publish rollback fixture: %v", err)
	}
	projection := projectRollbackCatalog(
		context.Background(),
		published,
		path,
		workspace.Identity{
			GenerationID:    publication.GenerationID,
			PayloadChecksum: publication.PayloadChecksum,
		},
		input,
	)
	if projection.Status != catalogprojection.ProjectionStatusApplied {
		t.Fatalf("fixture projection = %#v, want applied", projection)
	}
	generation, err := client.CurrentGeneration(context.Background())
	if err != nil {
		t.Fatalf("CurrentGeneration: %v", err)
	}
	return rollbackFixture{
		generation:        generation,
		workspaceChecksum: projection.WorkspaceChecksum,
		workspacePayload:  loadWorkspacePayload(t, path),
	}
}

type rollbackFixture struct {
	generation        catalogstore.Generation
	workspaceChecksum string
	workspacePayload  []byte
}

func rollbackFixtureCatalog(t testing.TB, name string, source sources.ID) *catalogs.Builder {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{
		ID:   "provider-a",
		Name: "Provider A",
		Models: map[string]*catalogs.Model{
			"model-a": {ID: "model-a", Name: name},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	seedTestModelDefinitions(t, builder)
	builder.SetProvenance(provenance.Map{
		"model:" + provenance.ModelResourceID("provider-a", "model-a") + ":Name": {{
			Source: source,
			Field:  "Name",
			Value:  name,
			Reason: "rollback fixture",
		}},
	})
	return builder
}

func assertCatalogPayload(t testing.TB, catalog *catalogs.Catalog, want []byte) {
	t.Helper()
	got, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf(
			"catalog payload checksum = %q, want retained %q",
			catalogs.DescribeCatalogPayload(got).Checksum,
			catalogs.DescribeCatalogPayload(want).Checksum,
		)
	}
}

func assertWorkspacePayload(t testing.TB, path string, want []byte) {
	t.Helper()
	got := loadWorkspacePayload(t, path)
	if !bytes.Equal(got, want) {
		t.Fatalf(
			"workspace checksum = %q, want prior projection %q",
			catalogs.DescribeCatalogPayload(got).Checksum,
			catalogs.DescribeCatalogPayload(want).Checksum,
		)
	}
}

func loadWorkspacePayload(t testing.TB, path string) []byte {
	t.Helper()
	builder, err := catalogs.NewFromPath(path)
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build workspace: %v", err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload workspace: %v", err)
	}
	return payload
}

func editRollbackWorkspaceName(t testing.TB, path, name string) {
	t.Helper()
	builder, err := catalogs.NewFromPath(path)
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	provider, err := builder.Provider("provider-a")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	provider.Models["model-a"].Name = name
	if err := builder.SetProvider(provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	if err := builder.SaveTo(path); err != nil {
		t.Fatalf("Save human edit: %v", err)
	}
}

func assertRollbackWorkspaceName(t testing.TB, path, want string) {
	t.Helper()
	builder, err := catalogs.NewFromPath(path)
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	provider, err := builder.Provider("provider-a")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if got := provider.Models["model-a"].Name; got != want {
		t.Fatalf("workspace model name = %q, want %q", got, want)
	}
}
