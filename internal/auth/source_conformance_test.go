package auth

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestCredentialSourceConformance(t *testing.T) {
	vectors := []struct {
		id  string
		run func(*testing.T)
	}{
		{id: "static", run: conformanceStatic},
		{id: "default_chain", run: conformanceDefaultChain},
		{id: "version", run: conformanceVersion},
		{id: "expiry", run: conformanceExpiry},
		{id: "lease", run: conformanceLease},
		{id: "cancellation", run: conformanceCancellation},
		{id: "concurrency", run: conformanceConcurrency},
		{id: "denial", run: conformanceDenial},
		{id: "redaction", run: conformanceRedaction},
		{id: "rotation_in_place", run: conformanceRotationInPlace},
		{id: "rotation_atomic_replace", run: conformanceRotationAtomicReplace},
		{id: "rotation_symlink_swap", run: conformanceRotationSymlinkSwap},
		{id: "rotation_mounted_replace", run: conformanceRotationMountedReplace},
		{id: "rotation_agent_rerender", run: conformanceRotationAgentRerender},
	}
	for _, vector := range vectors {
		t.Run(vector.id, vector.run)
	}
}

func TestDefaultChainResolutionIsSingleFlightAndCachedUntilRefresh(t *testing.T) {
	var calls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	chain := cloudChainFunc(func(
		context.Context,
		catalogs.ProviderCredentialProfile,
		map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
	) (sourceMaterial, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return sourceMaterial{
			values:  map[string]string{"access-token": "token", "project": "project"},
			version: "chain-version",
			lease: &sources.ProviderCredentialLease{
				Renewable: true, RefreshAfter: time.Now().Add(time.Hour),
			},
		}, nil
	})
	provider := defaultChainCredentialProvider()
	resolver := newResolver(mapEnvironment(nil), withCloudChain(
		catalogs.ProviderAuthenticationGoogleDefault,
		chain,
	))

	const workers = 32
	errs := make(chan error, workers)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := resolver.ResolveCatalog(context.Background(), &provider)
			errs <- err
		}()
	}
	<-entered
	close(release)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("ResolveCatalog: %v", err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("default-chain calls = %d, want 1", got)
	}
	if _, err := resolver.ResolveCatalog(context.Background(), &provider); err != nil {
		t.Fatalf("cached ResolveCatalog: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("cached default-chain calls = %d, want 1", got)
	}
}

func TestExpiredDefaultChainMaterialIsNotCachedPastExpiry(t *testing.T) {
	var calls atomic.Int32
	chain := cloudChainFunc(func(
		context.Context,
		catalogs.ProviderCredentialProfile,
		map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
	) (sourceMaterial, error) {
		calls.Add(1)
		return sourceMaterial{
			values:    map[string]string{"access-token": "token", "project": "project"},
			version:   "chain-version",
			expiresAt: time.Now().Add(-time.Minute),
			lease: &sources.ProviderCredentialLease{
				Renewable: true, RefreshAfter: time.Now().Add(time.Hour),
			},
		}, nil
	})
	provider := defaultChainCredentialProvider()
	resolver := newResolver(mapEnvironment(nil), withCloudChain(
		catalogs.ProviderAuthenticationGoogleDefault,
		chain,
	))
	for range 2 {
		if _, err := resolver.ResolveCatalog(context.Background(), &provider); err != nil {
			t.Fatalf("ResolveCatalog: %v", err)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expired default-chain calls = %d, want 2", got)
	}
}

func conformanceStatic(t *testing.T) {
	provider := ambientCredentialProvider()
	resolver := newResolver(mapEnvironment(map[string]string{"OPENAI_API_KEY": "valid-static"}))
	material, err := resolver.ResolveCatalog(context.Background(), &provider)
	if err != nil {
		t.Fatalf("ResolveCatalog: %v", err)
	}
	if value, _ := material.Value("api-key"); value != "valid-static" {
		t.Fatalf("api-key = %q", value)
	}
	if material.Version() == "" {
		t.Fatal("static material has no opaque version")
	}
}

func conformanceDefaultChain(t *testing.T) {
	provider := defaultChainCredentialProvider()
	chain := &fakeCloudChain{material: sourceMaterial{
		values:  map[string]string{"access-token": "token", "project": "project"},
		version: "chain-version",
	}}
	resolver := newResolver(mapEnvironment(nil), withCloudChain(
		catalogs.ProviderAuthenticationGoogleDefault,
		chain,
	))
	material, err := resolver.ResolveCatalog(context.Background(), &provider)
	if err != nil {
		t.Fatalf("ResolveCatalog: %v", err)
	}
	if value, _ := material.Value("access-token"); value != "token" {
		t.Fatalf("access-token = %q", value)
	}
}

func conformanceVersion(t *testing.T) {
	provider := ambientCredentialProvider()
	values := map[string]string{"OPENAI_API_KEY": "valid-one"}
	resolver := newResolver(mapEnvironment(values))
	first, err := resolver.ResolveCatalog(context.Background(), &provider)
	if err != nil {
		t.Fatalf("first ResolveCatalog: %v", err)
	}
	values["OPENAI_API_KEY"] = "valid-two"
	second, err := resolver.ResolveCatalog(context.Background(), &provider)
	if err != nil {
		t.Fatalf("second ResolveCatalog: %v", err)
	}
	if first.Version() == "" || first.Version() == second.Version() {
		t.Fatalf("versions = %q and %q", first.Version(), second.Version())
	}
}

func conformanceExpiry(t *testing.T) {
	expiresAt := time.Now().UTC().Add(time.Hour)
	material := resolveTestSource(t, sourceMaterial{
		values:    map[string]string{"value": "valid-expiring"},
		version:   "expiring",
		expiresAt: expiresAt,
	})
	got, found := material.ExpiresAt()
	if !found || !got.Equal(expiresAt) {
		t.Fatalf("expiry = %v, %t", got, found)
	}
}

func conformanceLease(t *testing.T) {
	refreshAfter := time.Now().UTC().Add(time.Minute)
	material := resolveTestSource(t, sourceMaterial{
		values:  map[string]string{"value": "valid-leased"},
		version: "leased",
		lease: &sources.ProviderCredentialLease{
			Renewable:    true,
			RefreshAfter: refreshAfter,
		},
	})
	lease, found := material.Lease()
	if !found || !lease.Renewable || !lease.RefreshAfter.Equal(refreshAfter) {
		t.Fatalf("lease = %#v, %t", lease, found)
	}
}

func conformanceCancellation(t *testing.T) {
	source := &testCredentialSource{backend: "test", resolve: func(ctx context.Context, _ Reference) (sourceMaterial, error) {
		<-ctx.Done()
		return sourceMaterial{}, ctx.Err()
	}}
	provider, resolver := testReferenceResolver(t, source)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	_, err := resolver.ResolveCatalog(ctx, &provider)
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("ResolveCatalog error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("cancellation took %s", elapsed)
	}
}

func conformanceConcurrency(t *testing.T) {
	var calls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	source := &testCredentialSource{backend: "test", resolve: func(context.Context, Reference) (sourceMaterial, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return sourceMaterial{
			values: map[string]string{"value": "valid-concurrent"}, version: "one",
			lease: &sources.ProviderCredentialLease{RefreshAfter: time.Now().Add(time.Hour)},
		}, nil
	}}
	provider, resolver := testReferenceResolver(t, source)

	const workers = 32
	errs := make(chan error, workers)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := resolver.ResolveCatalog(context.Background(), &provider)
			errs <- err
		}()
	}
	<-entered
	close(release)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("ResolveCatalog: %v", err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("source calls = %d, want 1", got)
	}
}

func conformanceDenial(t *testing.T) {
	source := &testCredentialSource{backend: "test", resolve: func(context.Context, Reference) (sourceMaterial, error) {
		return sourceMaterial{}, newSourceError(SourceErrorDenied, "test")
	}}
	provider, resolver := testReferenceResolver(t, source)
	_, err := resolver.ResolveCatalog(context.Background(), &provider)
	if !isSourceError(err, SourceErrorDenied) {
		t.Fatalf("ResolveCatalog error = %v", err)
	}
}

func conformanceRedaction(t *testing.T) {
	provider := ambientCredentialProvider()
	key := CredentialFieldKey{ProviderID: provider.ID, FieldID: "api-key"}
	const sensitivePath = "/private/operator/customer-a/openai-secret"
	resolver := newResolver(mapEnvironment(nil), WithReferencePolicies(map[CredentialFieldKey]ReferencePolicy{
		key: {Reference: mustReference(t, "file:"+sensitivePath)},
	}))
	_, err := resolver.ResolveCatalog(context.Background(), &provider)
	if err == nil {
		t.Fatal("ResolveCatalog succeeded")
	}
	for _, forbidden := range []string{sensitivePath, "customer-a", "openai-secret"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("error exposed %q: %v", forbidden, err)
		}
	}
}

func conformanceRotationInPlace(t *testing.T) {
	path, provider, resolver := fileRotationFixture(t, "valid-before")
	first := resolveVersion(t, resolver, provider)
	writeSecret(t, path, "valid-after")
	second := resolveVersion(t, resolver, provider)
	assertVersionChanged(t, first, second)
}

func conformanceRotationAtomicReplace(t *testing.T) {
	path, provider, resolver := fileRotationFixture(t, "valid-before")
	first := resolveVersion(t, resolver, provider)
	replacement := path + ".next"
	writeSecret(t, replacement, "valid-after")
	if err := os.Rename(replacement, path); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	second := resolveVersion(t, resolver, provider)
	assertVersionChanged(t, first, second)
}

func conformanceRotationSymlinkSwap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics require elevated Windows privileges")
	}
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "secret.1")
	secondPath := filepath.Join(dir, "secret.2")
	writeSecret(t, firstPath, "valid-before")
	writeSecret(t, secondPath, "valid-after")
	link := filepath.Join(dir, "secret")
	if err := os.Symlink(firstPath, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	provider, resolver := resolverForFile(t, link)
	first := resolveVersion(t, resolver, provider)
	nextLink := link + ".next"
	if err := os.Symlink(secondPath, nextLink); err != nil {
		t.Fatalf("Symlink replacement: %v", err)
	}
	if err := os.Rename(nextLink, link); err != nil {
		t.Fatalf("Rename symlink: %v", err)
	}
	second := resolveVersion(t, resolver, provider)
	assertVersionChanged(t, first, second)
}

func conformanceRotationMountedReplace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("projected-volume symlink semantics require elevated Windows privileges")
	}
	dir := t.TempDir()
	firstData := filepath.Join(dir, "..2026_08_10_1")
	secondData := filepath.Join(dir, "..2026_08_10_2")
	if err := os.Mkdir(firstData, 0o700); err != nil {
		t.Fatalf("Mkdir first data: %v", err)
	}
	if err := os.Mkdir(secondData, 0o700); err != nil {
		t.Fatalf("Mkdir second data: %v", err)
	}
	writeSecret(t, filepath.Join(firstData, "api-key"), "valid-before")
	writeSecret(t, filepath.Join(secondData, "api-key"), "valid-after")
	dataLink := filepath.Join(dir, "..data")
	if err := os.Symlink(firstData, dataLink); err != nil {
		t.Fatalf("Symlink data: %v", err)
	}
	secretLink := filepath.Join(dir, "api-key")
	if err := os.Symlink(filepath.Join("..data", "api-key"), secretLink); err != nil {
		t.Fatalf("Symlink secret: %v", err)
	}
	provider, resolver := resolverForFile(t, secretLink)
	first := resolveVersion(t, resolver, provider)
	nextDataLink := filepath.Join(dir, "..data.next")
	if err := os.Symlink(secondData, nextDataLink); err != nil {
		t.Fatalf("Symlink next data: %v", err)
	}
	if err := os.Rename(nextDataLink, dataLink); err != nil {
		t.Fatalf("Rename data link: %v", err)
	}
	second := resolveVersion(t, resolver, provider)
	assertVersionChanged(t, first, second)
}

func conformanceRotationAgentRerender(t *testing.T) {
	path, provider, resolver := fileRotationFixture(t, "valid-before")
	first := resolveVersion(t, resolver, provider)
	templateOutput := filepath.Join(filepath.Dir(path), ".agent-render")
	writeSecret(t, templateOutput, "valid-after")
	if err := os.Rename(templateOutput, path); err != nil {
		t.Fatalf("agent rerender rename: %v", err)
	}
	second := resolveVersion(t, resolver, provider)
	assertVersionChanged(t, first, second)
}

type fakeCloudChain struct {
	material sourceMaterial
	err      error
}

type cloudChainFunc func(
	context.Context,
	catalogs.ProviderCredentialProfile,
	map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) (sourceMaterial, error)

func (f cloudChainFunc) resolve(
	ctx context.Context,
	profile catalogs.ProviderCredentialProfile,
	fields map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) (sourceMaterial, error) {
	return f(ctx, profile, fields)
}

func (f *fakeCloudChain) resolve(
	context.Context,
	catalogs.ProviderCredentialProfile,
	map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) (sourceMaterial, error) {
	return f.material, f.err
}

type testCredentialSource struct {
	backend ReferenceBackend
	resolve func(context.Context, Reference) (sourceMaterial, error)
}

func (s *testCredentialSource) Backend() ReferenceBackend { return s.backend }

func (s *testCredentialSource) Resolve(ctx context.Context, reference Reference) (sourceMaterial, error) {
	return s.resolve(ctx, reference)
}

func resolveTestSource(t *testing.T, material sourceMaterial) sources.ProviderCredentialMaterial {
	t.Helper()
	source := &testCredentialSource{backend: "test", resolve: func(context.Context, Reference) (sourceMaterial, error) {
		return material, nil
	}}
	provider, resolver := testReferenceResolver(t, source)
	resolved, err := resolver.ResolveCatalog(context.Background(), &provider)
	if err != nil {
		t.Fatalf("ResolveCatalog: %v", err)
	}
	return resolved
}

func testReferenceResolver(t *testing.T, source credentialSource) (catalogs.Provider, *Resolver) {
	t.Helper()
	provider := ambientCredentialProvider()
	provider.Credentials.Fields[0].Pattern = ""
	key := CredentialFieldKey{ProviderID: provider.ID, FieldID: "api-key"}
	resolver := newResolver(mapEnvironment(nil),
		withCredentialSource(source),
		WithReferencePolicies(map[CredentialFieldKey]ReferencePolicy{
			key: {Reference: mustReference(t, "test:resource")},
		}),
	)
	return provider, resolver
}

func fileRotationFixture(t *testing.T, value string) (string, catalogs.Provider, *Resolver) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "api-key")
	writeSecret(t, path, value)
	provider, resolver := resolverForFile(t, path)
	return path, provider, resolver
}

func resolverForFile(t *testing.T, path string) (catalogs.Provider, *Resolver) {
	t.Helper()
	provider := ambientCredentialProvider()
	provider.Credentials.Fields[0].Pattern = ""
	key := CredentialFieldKey{ProviderID: provider.ID, FieldID: "api-key"}
	resolver := newResolver(mapEnvironment(nil), WithReferencePolicies(map[CredentialFieldKey]ReferencePolicy{
		key: {Reference: mustReference(t, "file:"+path)},
	}))
	return provider, resolver
}

func writeSecret(t testing.TB, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	fixed := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, fixed, fixed); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
}

func resolveVersion(t testing.TB, resolver *Resolver, provider catalogs.Provider) string {
	t.Helper()
	material, err := resolver.ResolveCatalog(context.Background(), &provider)
	if err != nil {
		t.Fatalf("ResolveCatalog: %v", err)
	}
	if material.Version() == "" {
		t.Fatal("credential version is empty")
	}
	return material.Version()
}

func assertVersionChanged(t testing.TB, first, second string) {
	t.Helper()
	if first == second {
		t.Fatalf("credential version did not change: %q", first)
	}
}
