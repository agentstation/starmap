package auth

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestCatalogPolicyMigrationPersistsSelectionAcrossRestart(t *testing.T) {
	for _, test := range []struct {
		name                  string
		initial               EnvironmentPolicy
		conventional, product string
		conflict              bool
	}{
		{name: "fresh installation", initial: EnvironmentPolicyCurrent, conventional: "valid-old", product: "valid-new"},
		{name: "upgrade with different material", initial: EnvironmentPolicyLegacy, conventional: "valid-old", product: "valid-new", conflict: true},
		{name: "upgrade with identical material", initial: EnvironmentPolicyLegacy, conventional: "valid-same", product: "valid-same"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "policy")
			store := openTestPolicyStore(t, path, test.initial)
			values := map[string]string{"OPENAI_API_KEY": test.conventional, "STARMAP_OPENAI_API_KEY": test.product}
			provider := ambientCredentialProvider()
			resolve := func(store PolicyStore) error {
				resolver, err := NewMigrationResolver(newResolver(mapEnvironment(values)), store)
				if err != nil {
					return err
				}
				_, err = resolver.ResolveCatalog(t.Context(), &provider)
				return err
			}
			err := resolve(store)
			if errors.IsConflict(err) != test.conflict || (!test.conflict && err != nil) {
				t.Fatalf("migration error = %v, want conflict %v", err, test.conflict)
			}
			if test.conflict {
				if strings.Contains(err.Error(), test.conventional) || strings.Contains(err.Error(), test.product) ||
					!strings.Contains(err.Error(), "OPENAI_API_KEY") || !strings.Contains(err.Error(), "STARMAP_OPENAI_API_KEY") {
					t.Fatal("conflict must name variables without their values")
				}
				delete(values, "OPENAI_API_KEY")
				if err := resolve(store); err != nil {
					t.Fatal(err)
				}
			}
			reopened := openTestPolicyStore(t, path, EnvironmentPolicyLegacy)
			policy, err := reopened.Policy(t.Context(), provider.ID)
			if err != nil || policy != EnvironmentPolicyCurrent {
				t.Fatalf("reopened policy = %s, %v", policy, err)
			}
			values["OPENAI_API_KEY"] = "valid-different-after-migration"
			if err := resolve(reopened); err != nil {
				t.Fatalf("accepted policy reverted on configuration change: %v", err)
			}
		})
	}
}

func TestCatalogPolicyMigrationComparesAllFields(t *testing.T) {
	store := openTestPolicyStore(t, filepath.Join(t.TempDir(), "policy"), EnvironmentPolicyLegacy)
	provider := defaultChainCredentialProvider()
	provider.Credentials.Fields[0].Environment = []string{"CLOUD_TOKEN"}
	provider.Credentials.Fields[1].Environment = []string{"CLOUD_PROJECT"}
	base := newResolver(mapEnvironment(map[string]string{
		"CLOUD_TOKEN": "same-token", "STARMAP_CLOUD_ACCESS_TOKEN": "same-token",
		"CLOUD_PROJECT": "old-project", "STARMAP_CLOUD_PROJECT": "new-project",
	}))
	resolver, err := NewMigrationResolver(base, store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ResolveCatalog(t.Context(), &provider); !errors.IsConflict(err) {
		t.Fatalf("multi-field identity change = %v, want conflict", err)
	}
	policy, err := store.Policy(t.Context(), provider.ID)
	if err != nil || policy != EnvironmentPolicyLegacy {
		t.Fatal("conflicting provider advanced its policy")
	}
	other := ambientCredentialProvider()
	otherResolver, err := NewMigrationResolver(newResolver(mapEnvironment(map[string]string{"OPENAI_API_KEY": "valid-other"})), store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := otherResolver.ResolveCatalog(t.Context(), &other); err != nil {
		t.Fatalf("unaffected provider failed: %v", err)
	}
}

func TestCatalogPolicyMigrationSharesExplicitSecretSnapshot(t *testing.T) {
	store := openTestPolicyStore(t, filepath.Join(t.TempDir(), "policy"), EnvironmentPolicyLegacy)
	reads := 0
	source, resource := rotatingProfileSecret(t, referenceBackendAWSStore, &reads)
	provider := defaultChainCredentialProvider()
	base := newResolver(func(string) (string, bool) { return "", false }, withCredentialSource(source), WithReferencePolicies(map[CredentialFieldKey]ReferencePolicy{
		{ProviderID: provider.ID, FieldID: "access-token"}: {Reference: mustReference(t, resource+"#token")},
		{ProviderID: provider.ID, FieldID: "project"}:      {Reference: mustReference(t, resource+"#project")},
	}))
	resolver, err := NewMigrationResolver(base, store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ResolveCatalog(t.Context(), &provider); err != nil {
		t.Fatal(err)
	}
	if reads != 1 {
		t.Fatalf("policy comparison read %d secret versions", reads)
	}
}

func TestCatalogPolicyMigrationNeverFallsBackAfterReferenceError(t *testing.T) {
	for _, kind := range []SourceErrorKind{SourceErrorDenied, SourceErrorInvalid, SourceErrorUnavailable} {
		t.Run(string(kind), func(t *testing.T) {
			store := openTestPolicyStore(t, filepath.Join(t.TempDir(), "policy"), EnvironmentPolicyLegacy)
			provider := ambientCredentialProvider()
			source := &testCredentialSource{backend: "test", resolve: func(context.Context, Reference) (sourceMaterial, error) {
				return sourceMaterial{}, newSourceError(kind, "test")
			}}
			base := newResolver(func(string) (string, bool) { t.Fatal("reference error read an ambient credential"); return "", false }, withCredentialSource(source), WithReferencePolicies(map[CredentialFieldKey]ReferencePolicy{
				{ProviderID: provider.ID, FieldID: "api-key"}: {Reference: mustReference(t, "test:private"), FallbackAmbient: true},
			}))
			resolver, err := NewMigrationResolver(base, store)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := resolver.ResolveCatalog(t.Context(), &provider); !isSourceError(err, kind) {
				t.Fatalf("reference error = %v", err)
			}
			policy, err := store.Policy(t.Context(), provider.ID)
			if err != nil || policy != EnvironmentPolicyLegacy {
				t.Fatal("failed reference advanced policy")
			}
		})
	}
}

func TestCatalogPolicyStoreRejectsCorruptionAndOwnerChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy")
	store := openTestPolicyStore(t, path, EnvironmentPolicyLegacy)
	if err := store.Accept(t.Context(), "openai"); err != nil {
		t.Fatal(err)
	}
	owner := PolicyOwner{Product: "starport", Deployment: "test", Instance: "test"}
	if _, err := OpenFilePolicyStore(t.Context(), path, owner, EnvironmentPolicyCurrent); err == nil {
		t.Fatal("another product adopted policy state")
	}
	if err := os.WriteFile(filepath.Join(path, providerPolicyName("openai")), []byte(`{"schema_version":99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Policy(t.Context(), "openai"); err == nil {
		t.Fatal("corrupt provider policy fell back to defaults")
	}
}

func TestCatalogPolicyStoreRejectsAmbiguousRecords(t *testing.T) {
	for _, test := range []struct{ name, before, after string }{
		{"duplicate policy", `"policy":`, `"policy":"starmap-catalog-v1","policy":`},
		{"case alias", `"policy":`, `"Policy":`},
		{"duplicate owner", `"product":`, `"product":"another-product","product":`},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "policy")
			store := openTestPolicyStore(t, path, EnvironmentPolicyCurrent)
			file := filepath.Join(path, policyRecordName)
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			data = bytes.Replace(data, []byte(test.before), []byte(test.after), 1)
			if err := os.WriteFile(file, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Policy(t.Context(), "openai"); err == nil {
				t.Fatal("ambiguous policy state changed selection")
			}
		})
	}
}

func openTestPolicyStore(t *testing.T, path string, initial EnvironmentPolicy) *FilePolicyStore {
	t.Helper()
	store, err := OpenFilePolicyStore(t.Context(), path, PolicyOwner{Product: "starmap", Deployment: "test", Instance: "test"}, initial)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

var _ PolicyStore = (*FilePolicyStore)(nil)

func TestCatalogPolicyStoreConcurrentAcceptance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy")
	stores := []*FilePolicyStore{openTestPolicyStore(t, path, EnvironmentPolicyLegacy), openTestPolicyStore(t, path, EnvironmentPolicyLegacy)}
	errs := make(chan error, 12)
	var group sync.WaitGroup
	for i := range 12 {
		group.Go(func() { errs <- stores[i%len(stores)].Accept(t.Context(), "openai") })
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

// A held publication lock must exhaust the writer wait, or the caller's
// deadline when it comes first.
func TestCatalogPolicyStorePublicationDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "policy")
		store := openTestPolicyStore(t, path, EnvironmentPolicyLegacy)
		lock := flock.New(filepath.Join(path, privatefiles.PublicationDirectoryName, ".owner.lock"))
		defer func() { _ = lock.Close() }()
		held, err := lock.TryLock()
		if err != nil || !held {
			t.Fatalf("hold real publication lock: held=%v, error=%v", held, err)
		}
		start := time.Now()
		err = store.Accept(t.Context(), "openai")
		if !stderrors.Is(err, context.DeadlineExceeded) || time.Since(start) != policyWriterWait {
			t.Fatalf("writer wait: elapsed=%v, error=%v", time.Since(start), err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		start = time.Now()
		err = store.Accept(ctx, "openai")
		if !stderrors.Is(err, context.DeadlineExceeded) || time.Since(start) != time.Second {
			t.Fatalf("caller deadline: elapsed=%v, error=%v", time.Since(start), err)
		}
		policy, err := store.Policy(t.Context(), "openai")
		if err != nil || policy != EnvironmentPolicyLegacy {
			t.Fatalf("failed publication changed policy: policy=%v, error=%v", policy, err)
		}
		if err := lock.Unlock(); err != nil {
			t.Fatal(err)
		}
		if err := store.Accept(t.Context(), "openai"); err != nil {
			t.Fatalf("publication after lock release: %v", err)
		}
		policy, err = store.Policy(t.Context(), "openai")
		if err != nil || policy != EnvironmentPolicyCurrent {
			t.Fatalf("accepted policy: policy=%v, error=%v", policy, err)
		}
	})
}

// A publication slower than the writer wait must complete under the caller's context.
func TestCatalogPolicyStoreSlowPublicationCompletes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := openTestPolicyStore(t, filepath.Join(t.TempDir(), "policy"), EnvironmentPolicyLegacy)
		store.directory = slowPolicyDirectory{policyDirectory: store.directory, delay: 2 * policyWriterWait}
		start := time.Now()
		if err := store.Accept(t.Context(), "openai"); err != nil || time.Since(start) != 2*policyWriterWait {
			t.Fatalf("slow publication: elapsed=%v, error=%v", time.Since(start), err)
		}
		policy, err := store.Policy(t.Context(), "openai")
		if err != nil || policy != EnvironmentPolicyCurrent {
			t.Fatalf("accepted policy: policy=%v, error=%v", policy, err)
		}
	})
}

// slowPolicyDirectory delays each publication as slow storage would.
type slowPolicyDirectory struct {
	policyDirectory
	delay time.Duration
}

func (d slowPolicyDirectory) CompareAndPublishFileContext(ctx context.Context, name string, previous, data []byte, prefix string) error {
	select {
	case <-time.After(d.delay):
	case <-ctx.Done():
		return ctx.Err()
	}
	return d.policyDirectory.CompareAndPublishFileContext(ctx, name, previous, data, prefix)
}

func TestCatalogPolicyMigrationRefusesExpiryDuringAcceptance(t *testing.T) {
	store := openTestPolicyStore(t, filepath.Join(t.TempDir(), "policy"), EnvironmentPolicyLegacy)
	var expiry time.Time
	source := &testCredentialSource{backend: "test", resolve: func(context.Context, Reference) (sourceMaterial, error) {
		expiry = time.Now().Add(time.Second)
		return sourceMaterial{values: map[string]string{"value": "valid-material"}, version: "one", expiresAt: expiry}, nil
	}}
	provider, base := testReferenceResolver(t, source)
	delayed := &delayedPolicyAcceptance{store: store, expiry: &expiry}
	resolver, err := NewMigrationResolver(base, delayed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ResolveCatalog(t.Context(), &provider); err == nil {
		t.Fatal("policy acceptance returned expired material")
	}
	if !delayed.accepted {
		t.Fatal("test did not reach policy acceptance")
	}
}

type delayedPolicyAcceptance struct {
	store    PolicyStore
	expiry   *time.Time
	accepted bool
}

func (s *delayedPolicyAcceptance) Policy(ctx context.Context, provider catalogs.ProviderID) (EnvironmentPolicy, error) {
	return s.store.Policy(ctx, provider)
}
func (s *delayedPolicyAcceptance) Accept(ctx context.Context, provider catalogs.ProviderID) error {
	select {
	case <-time.After(time.Until(*s.expiry) + time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}
	s.accepted = true
	return s.store.Accept(ctx, provider)
}
