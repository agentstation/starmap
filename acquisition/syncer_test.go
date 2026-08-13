package acquisition

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type testProviderClient struct{}

func (testProviderClient) ListModels(
	context.Context,
	sources.ProviderCredentialMaterial,
) ([]catalogs.Model, error) {
	return nil, nil
}

func TestSyncDryRunNeedsNoWritableStore(t *testing.T) {
	t.Parallel()

	client, err := starmap.New()
	if err != nil {
		t.Fatalf("New client: %v", err)
	}
	syncer, err := New(client)
	if err != nil {
		t.Fatalf("New syncer: %v", err)
	}
	result, err := syncer.Sync(
		context.Background(),
		pkgsync.WithDryRun(true),
		pkgsync.WithSources(sources.EmbeddedCatalogID),
	)
	if err != nil {
		t.Fatalf("dry-run Sync: %v", err)
	}
	if !result.DryRun {
		t.Fatalf("DryRun = false, want true")
	}
	if result.GenerationID != "" || result.Projection != nil {
		t.Fatalf("dry-run publication = %#v, want none", result)
	}
}

func TestSyncRequiresWritableStoreBeforeProviderAcquisition(t *testing.T) {
	t.Parallel()

	client, err := starmap.New()
	if err != nil {
		t.Fatalf("New client: %v", err)
	}
	var calls atomic.Int32
	syncer, err := New(client, WithProviderClientFactory(func(
		*catalogs.Provider,
	) (sources.ProviderClient, error) {
		calls.Add(1)
		return testProviderClient{}, nil
	}))
	if err != nil {
		t.Fatalf("New syncer: %v", err)
	}
	_, err = syncer.Sync(
		context.Background(),
		pkgsync.WithSources(sources.ProvidersID),
		pkgsync.WithProvider("openai"),
		pkgsync.WithReformat(true),
	)
	var configErr *pkgerrors.ConfigError
	if !stderrors.As(err, &configErr) || configErr.Component != "catalog store" {
		t.Fatalf("Sync error = %T %v, want catalog-store ConfigError", err, err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("provider factory calls = %d, want zero", got)
	}
}

func TestSyncPublishesStoreOnlyWithoutWorkspaceProjection(t *testing.T) {
	t.Parallel()

	store := catalogstore.NewMemory()
	client, err := starmap.New(starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatalf("New client: %v", err)
	}
	syncer, err := New(client)
	if err != nil {
		t.Fatalf("New syncer: %v", err)
	}
	result, err := syncer.Sync(
		context.Background(),
		pkgsync.WithSources(sources.EmbeddedCatalogID),
		pkgsync.WithReformat(true),
	)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if result.GenerationID == "" || result.Projection != nil {
		t.Fatalf("store-only result = %#v", result)
	}
	current, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if current.Manifest.GenerationID != result.GenerationID ||
		client.CurrentGenerationID() != result.GenerationID {
		t.Fatalf(
			"published generations = store %q client %q result %q",
			current.Manifest.GenerationID,
			client.CurrentGenerationID(),
			result.GenerationID,
		)
	}
}

func TestSyncRejectsWorkspaceDifferentFromClientConfiguration(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	configured := filepath.Join(root, "configured")
	client, err := starmap.New(starmap.WithCatalogPath(configured))
	if err != nil {
		t.Fatalf("New client: %v", err)
	}
	syncer, err := New(client)
	if err != nil {
		t.Fatalf("New syncer: %v", err)
	}
	_, err = syncer.Sync(
		context.Background(),
		pkgsync.WithDryRun(true),
		pkgsync.WithCatalogPath(filepath.Join(root, "different")),
		pkgsync.WithSources(sources.EmbeddedCatalogID),
	)
	var configErr *pkgerrors.ConfigError
	if !stderrors.As(err, &configErr) || configErr.Component != "acquisition" {
		t.Fatalf("Sync error = %T %v, want acquisition ConfigError", err, err)
	}
}

func TestSyncerProviderFactoriesAreInstanceLocal(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	client, err := starmap.New()
	if err != nil {
		t.Fatalf("New client: %v", err)
	}
	var firstCalls atomic.Int32
	var secondCalls atomic.Int32
	first, err := New(client, WithProviderClientFactory(func(
		provider *catalogs.Provider,
	) (sources.ProviderClient, error) {
		if provider.ID != "openai" {
			t.Fatalf("first factory provider = %q, want openai", provider.ID)
		}
		firstCalls.Add(1)
		return testProviderClient{}, nil
	}))
	if err != nil {
		t.Fatalf("New first syncer: %v", err)
	}
	second, err := New(client, WithProviderClientFactory(func(
		provider *catalogs.Provider,
	) (sources.ProviderClient, error) {
		if provider.ID != "openai" {
			t.Fatalf("second factory provider = %q, want openai", provider.ID)
		}
		secondCalls.Add(1)
		return testProviderClient{}, nil
	}))
	if err != nil {
		t.Fatalf("New second syncer: %v", err)
	}
	opts := []pkgsync.Option{
		pkgsync.WithDryRun(true),
		pkgsync.WithSources(sources.ProvidersID),
		pkgsync.WithProvider("openai"),
	}
	if _, err := first.Sync(context.Background(), opts...); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	if _, err := second.Sync(context.Background(), opts...); err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if _, err := first.Sync(context.Background(), opts...); err != nil {
		t.Fatalf("repeat first Sync: %v", err)
	}
	if got := firstCalls.Load(); got != 2 {
		t.Fatalf("first factory calls = %d, want 2", got)
	}
	if got := secondCalls.Load(); got != 1 {
		t.Fatalf("second factory calls = %d, want 1", got)
	}
}

func TestSyncerUsesInjectedCredentialResolver(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	client, err := starmap.New()
	if err != nil {
		t.Fatalf("New client: %v", err)
	}
	var resolverCalls atomic.Int32
	resolver := sources.ProviderCredentialResolverFunc(func(
		_ context.Context,
		provider *catalogs.Provider,
	) (sources.ProviderCredentialMaterial, error) {
		resolverCalls.Add(1)
		if provider == nil || provider.ID != "openai" || provider.Credentials == nil {
			t.Fatalf("resolver provider = %#v", provider)
		}
		return sources.NewProviderCredentialMaterial(
			provider.Credentials.Profiles[0],
			map[catalogs.ProviderCredentialFieldID]string{"api-key": "injected-key"},
			sources.ProviderCredentialMetadata{Version: "test"},
		), nil
	})
	syncer, err := New(
		client,
		WithCredentialResolver(resolver),
		WithProviderClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
			return testProviderClient{}, nil
		}),
	)
	if err != nil {
		t.Fatalf("New syncer: %v", err)
	}
	if _, err := syncer.Sync(
		context.Background(),
		pkgsync.WithDryRun(true),
		pkgsync.WithSources(sources.ProvidersID),
		pkgsync.WithProvider("openai"),
	); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if got := resolverCalls.Load(); got != 1 {
		t.Fatalf("resolver calls = %d, want 1", got)
	}
}

func TestWithCredentialResolverRejectsNil(t *testing.T) {
	client, err := starmap.New()
	if err != nil {
		t.Fatalf("New client: %v", err)
	}
	if _, err := New(client, WithCredentialResolver(nil)); err == nil {
		t.Fatal("New accepted a nil credential resolver")
	}
}

type deadlineRecordingStore struct {
	*catalogstore.Memory
	sawDeadline atomic.Bool
	err         error
}

func (s *deadlineRecordingStore) Commit(
	ctx context.Context,
	_ catalogs.Generation,
	_ string,
) error {
	_, ok := ctx.Deadline()
	s.sawDeadline.Store(ok)
	return s.err
}

func TestSyncTimeoutContextReachesDurableCommit(t *testing.T) {
	t.Parallel()

	injected := stderrors.New("injected commit failure")
	store := &deadlineRecordingStore{
		Memory: catalogstore.NewMemory(),
		err:    injected,
	}
	client, err := starmap.New(starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatalf("New client: %v", err)
	}
	syncer, err := New(client)
	if err != nil {
		t.Fatalf("New syncer: %v", err)
	}
	_, err = syncer.Sync(
		context.Background(),
		pkgsync.WithTimeout(constants.UpdateContextTimeout),
		pkgsync.WithSources(sources.EmbeddedCatalogID),
		pkgsync.WithReformat(true),
	)
	if !stderrors.Is(err, injected) {
		t.Fatalf("Sync error = %v, want injected commit failure", err)
	}
	if !store.sawDeadline.Load() {
		t.Fatal("configured sync timeout did not reach durable commit")
	}
}

func TestProjectCommittedCatalogReportsAppliedAndPendingRepair(t *testing.T) {
	t.Parallel()

	catalog := acquisitionTestCatalog(t)
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	publication := starmap.Publication{
		Published:       true,
		GenerationID:    "generation-test",
		PayloadChecksum: catalogs.DescribeCatalogPayload(payload).Checksum,
	}

	path := filepath.Join(t.TempDir(), "catalog")
	applied := projectCommittedCatalog(
		context.Background(),
		catalog,
		path,
		publication,
		workspace.InputExpectation{},
	)
	if applied.Status != pkgsync.ProjectionStatusApplied ||
		applied.WorkspaceChecksum == "" ||
		applied.IssueCode != "" {
		t.Fatalf("applied projection = %#v", applied)
	}

	blockingFile := filepath.Join(t.TempDir(), "catalog-file")
	if err := os.WriteFile(blockingFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create blocking file: %v", err)
	}
	pending := projectCommittedCatalog(
		context.Background(),
		catalog,
		blockingFile,
		publication,
		workspace.InputExpectation{},
	)
	if pending.Status != pkgsync.ProjectionStatusPendingRepair ||
		pending.IssueCode != pkgsync.ProjectionIssueWorkspaceFailed {
		t.Fatalf("pending projection = %#v", pending)
	}
}

func TestNewRejectsNilComposition(t *testing.T) {
	t.Parallel()

	if syncer, err := New(nil); syncer != nil || err == nil {
		t.Fatalf("New(nil) = %#v, %v", syncer, err)
	}
	client, err := starmap.New()
	if err != nil {
		t.Fatalf("New client: %v", err)
	}
	if syncer, err := New(client, nil); syncer != nil || err == nil {
		t.Fatalf("New(client, nil) = %#v, %v", syncer, err)
	}
	if syncer, err := New(client, WithProviderClientFactory(nil)); syncer != nil || err == nil {
		t.Fatalf("New(client, nil factory) = %#v, %v", syncer, err)
	}
}

func acquisitionTestCatalog(t testing.TB) *catalogs.Catalog {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetAuthor(catalogs.Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetAuthorModel("author", catalogs.Model{
		ID:      "model",
		Name:    "Model",
		Authors: []catalogs.Author{{ID: "author", Name: "Author"}},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	if err := builder.SetProvider(catalogs.Provider{
		ID: "provider", Name: "Provider",
		Models: map[string]*catalogs.Model{
			"provider-model": {
				ID:       "provider-model",
				ModelRef: "author/model",
				Name:     "Model",
			},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return catalog
}
