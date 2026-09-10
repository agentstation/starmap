package runtime

import (
	"context"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

type authorityTestSource struct {
	*stubSource
	permissionMu   sync.Mutex
	receipt        catalogs.CatalogPermissionEnvelope
	permissionErr  error
	permissionHook func(context.Context)
}

func (s *authorityTestSource) ReadPermission(ctx context.Context) (catalogs.CatalogPermissionEnvelope, error) {
	s.permissionMu.Lock()
	receipt, err, hook := s.receipt, s.permissionErr, s.permissionHook
	s.permissionMu.Unlock()
	if hook != nil {
		hook(ctx)
	}
	return receipt, err
}

func authorityRuntimeFixture(t *testing.T) (*authorityTestSource, []Option) {
	t.Helper()
	receipt := authorityPermissionFixture()
	generation := aliasGeneration(t, receipt.Head.GenerationID)
	generation.Manifest.ManifestVersion = catalogs.AuthorityGenerationManifestVersion
	receipt.Head.PayloadChecksum = generation.Manifest.Payload.Checksum
	generation.Manifest.AuthorityHead = receipt.Head
	s := &authorityTestSource{stubSource: newStubSource("internal-authority"), receipt: receipt}
	s.replies = []SourceRead{aliasRead(generation)}
	return s, []Option{
		WithCatalogSource("starmap"), WithSourceURL("https://authority.example"),
		WithSourceStartupPolicy("require_authority"), WithSourceAuthority("enterprise", "production"),
		WithSource(s), WithClock(func() time.Time { return receipt.IssuedAt.Add(time.Second) }),
		WithPermissionClockUncertainty(func() (time.Duration, bool) { return time.Second, true }),
		WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())),
	}
}

func TestAuthorityRuntimeColdDiagnosticsDoNotAuthorizeBaseline(t *testing.T) {
	s, options := authorityRuntimeFixture(t)
	s.permissionErr = fs.ErrNotExist
	r := openTestRuntime(t, options...)
	if r.Catalog() == nil || !r.Status().CatalogAvailable {
		t.Fatal("cold authority lost metadata diagnostics")
	}
	if r.AllowsNewAttempt() || r.Status().Usable || r.Status().AuthorityReady {
		t.Fatal("cold authority authorized the embedded baseline")
	}
}

func TestAuthorityRuntimeDoesNotAcquireLocally(t *testing.T) {
	_, opts := authorityRuntimeFixture(t)
	acquirer := &stubAcquirer{}
	r := openTestRuntime(t, append(opts, WithAcquirer(acquirer))...)
	_, err := r.Sync(t.Context())
	if err == nil || acquirer.callCount() != 0 {
		t.Fatalf("internal authority reached local acquisition: error=%v calls=%d", err, acquirer.callCount())
	}
}

func TestAuthorityRuntimeActivatesOnlyAuthorityCatalog(t *testing.T) {
	_, options := authorityRuntimeFixture(t)
	r := openTestRuntime(t, options...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !r.AllowsNewAttempt() || !r.Status().Usable || !r.Status().AuthorityReady {
		t.Fatalf("approved catalog is unusable: %+v", r.Status())
	}
	r.mu.RLock()
	head := r.permissions.enforced
	r.mu.RUnlock()
	payload, err := catalogs.EncodeCatalogPayload(r.Catalog())
	if err != nil {
		t.Fatal(err)
	}
	if catalogs.DescribeCatalogPayload(payload).Checksum != head.PayloadChecksum {
		t.Fatal("local baseline expanded the authority catalog")
	}
	local := testProviderLayer(t, "local", "not-approved", "Not approved", time.Now())
	if _, err := r.publishProviders(t.Context(), []ProviderLayer{local}, r.lease.epoch()); err == nil {
		t.Fatal("local acquisition expanded authority membership")
	}
	if n := testing.AllocsPerRun(100, func() {
		if !r.AllowsNewAttempt() {
			t.Fatal("admission changed")
		}
	}); n != 0 {
		t.Fatalf("permission check allocations = %v", n)
	}
}

func TestAuthorityRuntimeWarmOfflineStartAndExpiry(t *testing.T) {
	s, options := authorityRuntimeFixture(t)
	options = append(options, WithStateDirectory(privateRuntimeDirectory(t)))
	r := openTestRuntime(t, options...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	before := r.State()
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	s.permissionMu.Lock()
	s.permissionErr = fs.ErrNotExist
	s.permissionMu.Unlock()
	warm := openTestRuntime(t, options...)
	if !warm.AllowsNewAttempt() || warm.State().PayloadChecksum != before.PayloadChecksum {
		t.Fatal("valid retained authority failed offline startup")
	}
	if err := warm.Close(); err != nil {
		t.Fatal(err)
	}
	expired := openTestRuntime(t, append(options, WithClock(func() time.Time { return s.receipt.ValidUntil }))...)
	if expired.AllowsNewAttempt() || expired.Status().Usable {
		t.Fatal("expired receipt authorized a restart")
	}
	if expired.Catalog() == nil {
		t.Fatal("expiry removed diagnostics")
	}
}

func TestAuthorityRuntimeUnknownClockRefusesAdmission(t *testing.T) {
	_, opts := authorityRuntimeFixture(t)
	opts = append(opts, func(o *options) error { o.permissionClockUncertainty = nil; return nil })
	r := openTestRuntime(t, opts...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !r.Status().AuthorityReady || r.AllowsNewAttempt() || r.Status().PermissionValid {
		t.Fatal("unknown clock authorized catalog use")
	}
}

func TestAuthorityRuntimeWithdrawalBlocksBeforeFailedActivationAndRestart(t *testing.T) {
	s, options := authorityRuntimeFixture(t)
	options = append(options, WithStateDirectory(privateRuntimeDirectory(t)))
	store := &retentionRejectingStore{Memory: storage.NewMemory()}
	options = append(options, WithClientOptions(starmap.WithCatalogStore(store)))
	r := openTestRuntime(t, options...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	before := r.State()
	s.permissionMu.Lock()
	s.receipt.Head.Sequence++
	s.receipt.Head.GenerationID = "withdrawn-two"
	s.receipt.Head.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
	next := s.receipt
	s.permissionMu.Unlock()
	s.mu.Lock()
	generation := s.replies[0].Generation.Copy()
	generation.Manifest.GenerationID = next.Head.GenerationID
	generation.Manifest.AuthorityHead = next.Head
	s.replies = []SourceRead{aliasRead(generation)}
	s.mu.Unlock()
	store.reject.Store(true)
	if _, err := r.RefreshSource(t.Context()); err == nil {
		t.Fatal("failed catalog store accepted replacement")
	}
	if r.AllowsNewAttempt() || r.State().GenerationID != before.GenerationID {
		t.Fatal("failed replacement authorized stale policy or replaced metadata")
	}
	if r.Status().RequiredPermissionRevision != next.Head.RequiredPermissionRevision {
		t.Fatal("failed activation lost known withdrawal")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	s.permissionMu.Lock()
	s.permissionErr = fs.ErrNotExist
	s.permissionMu.Unlock()
	restarted := openTestRuntime(t, options...)
	if restarted.AllowsNewAttempt() || restarted.Status().RequiredPermissionRevision != next.Head.RequiredPermissionRevision {
		t.Fatal("restart lost known withdrawal")
	}
}

func TestAuthorityRuntimePermissionRefreshBypassesBlockedCatalogAndLease(t *testing.T) {
	s, options := authorityRuntimeFixture(t)
	r := openTestRuntime(t, options...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	s.observe = func(ctx context.Context) {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
		}
	}
	finished := make(chan error, 1)
	go func() { _, err := r.RefreshSource(t.Context()); finished <- err }()
	<-entered
	s.permissionMu.Lock()
	s.receipt.Head.Sequence++
	s.receipt.Head.PermissionSchemaVersion++
	s.permissionMu.Unlock()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := r.RefreshPermission(ctx); err != nil {
		t.Fatal(err)
	}
	if r.AllowsNewAttempt() {
		t.Fatal("blocked catalog transfer prevented withdrawal enforcement")
	}
	close(release)
	<-finished
	s.observe = nil
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	follower := openTestRuntime(t, append(options, WithLeaseStore(&stubLeaseStore{refuseAll: true}))...)
	if err := follower.RefreshPermission(ctx); err != nil {
		t.Fatalf("lease follower cannot learn permission: %v", err)
	}
	if follower.AllowsNewAttempt() {
		t.Fatal("lease follower authorized baseline")
	}
}

func TestAuthorityRuntimeFailedReceiptWriteRequiresFreshEvidenceAfterRestart(t *testing.T) {
	s, options := authorityRuntimeFixture(t)
	options = append(options, WithStateDirectory(privateRuntimeDirectory(t)))
	r := openTestRuntime(t, options...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s.permissionMu.Lock()
	s.receipt.Head.Sequence++
	s.receipt.Head.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
	s.permissionHook = func(readCtx context.Context) { cancel(); <-readCtx.Done() }
	s.permissionMu.Unlock()
	if err := r.RefreshPermission(ctx); err == nil {
		t.Fatal("canceled receipt write succeeded")
	}
	if r.AllowsNewAttempt() {
		t.Fatal("failed receipt write kept approval")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	s.permissionMu.Lock()
	s.permissionErr = fs.ErrNotExist
	s.permissionHook = nil
	s.permissionMu.Unlock()
	restarted := openTestRuntime(t, options...)
	if restarted.AllowsNewAttempt() || restarted.Catalog() == nil {
		t.Fatal("uncertain restart restored permission or lost diagnostics")
	}
}

func TestAuthorityRuntimeManifestAheadOfReceiptRetainsRequirement(t *testing.T) {
	s, opts := authorityRuntimeFixture(t)
	opts = append(opts, WithStateDirectory(privateRuntimeDirectory(t)))
	r := openTestRuntime(t, opts...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	generation := s.replies[0].Generation.Copy()
	generation.Manifest.GenerationID = "manifest-ahead"
	generation.Manifest.AuthorityHead.GenerationID = "manifest-ahead"
	generation.Manifest.AuthorityHead.Sequence++
	generation.Manifest.AuthorityHead.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
	s.replies = []SourceRead{aliasRead(generation)}
	s.mu.Unlock()
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if r.AllowsNewAttempt() || r.Status().RequiredPermissionRevision != generation.Manifest.AuthorityHead.RequiredPermissionRevision {
		t.Fatal("manifest requirement was ignored or the old receipt authorized it")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	s.permissionMu.Lock()
	s.permissionErr = fs.ErrNotExist
	s.permissionMu.Unlock()
	restarted := openTestRuntime(t, opts...)
	if restarted.AllowsNewAttempt() || restarted.Status().RequiredPermissionRevision != generation.Manifest.AuthorityHead.RequiredPermissionRevision {
		t.Fatal("restart discarded the manifest requirement")
	}
}

func TestAuthorityRuntimeCrashCheckpointCannotRestorePermission(t *testing.T) {
	s, opts := authorityRuntimeFixture(t)
	opts = append(opts, WithStateDirectory(privateRuntimeDirectory(t)))
	r := openTestRuntime(t, opts...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	active, err := r.store.directory.ReadFile(permissionCheckpointFile, maxPermissionCheckpointBytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	// Restore the exact checkpoint bytes that a crash would leave before Close.
	if err := r.store.directory.WriteFileContext(t.Context(), permissionCheckpointFile, active, ".test-"); err != nil {
		t.Fatal(err)
	}
	s.permissionMu.Lock()
	s.permissionErr = fs.ErrNotExist
	s.permissionMu.Unlock()
	restarted := openTestRuntime(t, opts...)
	if restarted.AllowsNewAttempt() || !restarted.Status().CatalogAvailable {
		t.Fatal("crash checkpoint revived permission or lost metadata")
	}
}

func TestAuthorityRuntimeMalformedReceiptRevokesCurrentUse(t *testing.T) {
	s, opts := authorityRuntimeFixture(t)
	r := openTestRuntime(t, opts...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	s.permissionMu.Lock()
	s.permissionErr = &errors.ValidationError{Field: "permission", Message: "malformed authenticated body"}
	s.permissionMu.Unlock()
	if err := r.RefreshPermission(t.Context()); err == nil {
		t.Fatal("malformed receipt succeeded")
	}
	if r.AllowsNewAttempt() {
		t.Fatal("malformed authenticated receipt preserved permission")
	}
}

func TestAuthorityRuntimeExplicitIdentityChangeCannotReuseApproval(t *testing.T) {
	_, opts := authorityRuntimeFixture(t)
	opts = append(opts, WithStateDirectory(privateRuntimeDirectory(t)))
	r := openTestRuntime(t, opts...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := openTestRuntime(t, append(opts, WithSourceAuthority("another-enterprise", "production"))...)
	if restarted.AllowsNewAttempt() || restarted.Status().AuthorityReady {
		t.Fatal("new authority reused the old approval")
	}
}

func TestAuthorityRuntimeWarmStartAlignsPublicationClient(t *testing.T) {
	s, opts := authorityRuntimeFixture(t)
	opts = append(opts, WithStateDirectory(privateRuntimeDirectory(t)))
	r := openTestRuntime(t, opts...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	s.permissionMu.Lock()
	s.permissionErr = fs.ErrNotExist
	s.permissionMu.Unlock()
	warm := openTestRuntime(t, append(opts, WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())))...)
	if !warm.AllowsNewAttempt() || warm.State().PayloadChecksum != warm.Client().CurrentCatalogState().PayloadChecksum {
		t.Fatal("warm runtime and its publication client disagree")
	}
}

func TestAuthorityRuntimeReservesClientPublication(t *testing.T) {
	_, opts := authorityRuntimeFixture(t)
	r := openTestRuntime(t, opts...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	before := r.State()
	called := false
	if _, err := r.Client().Update(t.Context(), func(context.Context, *catalogs.Catalog) (*starmap.Candidate, error) {
		called = true
		return nil, nil
	}); err == nil || called {
		t.Fatal("direct client update reached candidate work")
	}
	if _, err := r.Client().Activate(t.Context(), catalogs.Generation{}); err == nil {
		t.Fatal("direct activation bypassed authority publication")
	}
	if r.Client().CurrentCatalogState().GenerationID != before.GenerationID || !r.AllowsNewAttempt() {
		t.Fatal("rejected client mutation changed the approved generation or disabled its permission")
	}
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatalf("refresh after rejected client update: %v", err)
	}
	if !r.AllowsNewAttempt() {
		t.Fatal("authority refresh remains unusable")
	}
}

func TestAuthorityRuntimeRejectsAnotherRuntimeCapability(t *testing.T) {
	_, firstOptions := authorityRuntimeFixture(t)
	_, secondOptions := authorityRuntimeFixture(t)
	first := openTestRuntime(t, firstOptions...)
	second := openTestRuntime(t, secondOptions...)
	called := false
	_, err := second.Client().Update(first.authorityPublicationContext(t.Context()), func(context.Context, *catalogs.Catalog) (*starmap.Candidate, error) { called = true; return nil, nil })
	if err == nil || called {
		t.Fatal("one runtime's capability authorized another runtime's publication")
	}
}
