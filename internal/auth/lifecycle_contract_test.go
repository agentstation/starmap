package auth

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/sources"
)

func TestCatalogRoleRequiresBoundedSourceRead(t *testing.T) {
	source := &testCredentialSource{backend: "test", resolve: func(ctx context.Context, _ Reference) (sourceMaterial, error) {
		deadline, bounded := ctx.Deadline()
		if !bounded || time.Until(deadline) > time.Minute {
			t.Error("credential read has no bounded deadline")
		}
		return sourceMaterial{}, context.DeadlineExceeded
	}}
	provider, resolver := testReferenceResolver(t, source)
	_, err := resolver.ResolveCatalog(t.Context(), &provider)
	if !stderrors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("source deadline error = %v", err)
	}
}

func TestCatalogRoleRefusesExpiredSourceMaterial(t *testing.T) {
	source := &testCredentialSource{backend: "test", resolve: func(context.Context, Reference) (sourceMaterial, error) {
		return sourceMaterial{
			values: map[string]string{"value": "valid-expired"}, version: "expired",
			expiresAt: time.Now().Add(-time.Second),
		}, nil
	}}
	provider, resolver := testReferenceResolver(t, source)
	material, err := resolver.ResolveCatalog(t.Context(), &provider)
	if err == nil {
		t.Fatal("resolver accepted expired credential material")
	}
	if _, found := material.Value("api-key"); found {
		t.Fatal("expiry failure returned credential material")
	}
}

func TestCatalogCredentialStatusReportsExpiryAsInvalid(t *testing.T) {
	source := &testCredentialSource{backend: "test", resolve: func(context.Context, Reference) (sourceMaterial, error) {
		return sourceMaterial{values: map[string]string{"value": "valid-expired"}, expiresAt: time.Now().Add(-time.Second)}, nil
	}}
	provider, resolver := testReferenceResolver(t, source)
	status := NewChecker(WithCredentialResolver(resolver)).CheckProvider(&provider, map[string]bool{string(provider.ID): true})
	if status.State != StateInvalid {
		t.Fatalf("expired credential state = %v, want invalid", status.State)
	}
}

func TestCatalogRoleRefusesResultAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	source := &testCredentialSource{backend: "test", resolve: func(context.Context, Reference) (sourceMaterial, error) {
		cancel()
		return sourceMaterial{values: map[string]string{"value": "valid-late"}, version: "late"}, nil
	}}
	provider, resolver := testReferenceResolver(t, source)
	_, err := resolver.ResolveCatalog(ctx, &provider)
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("late response error = %v, want cancellation", err)
	}
}

func TestCatalogRoleBackendFailureRefusesStaleMaterialAndRecovers(t *testing.T) {
	for _, kind := range []SourceErrorKind{SourceErrorUnavailable, SourceErrorDenied} {
		t.Run(string(kind), func(t *testing.T) {
			reads := 0
			source := &testCredentialSource{backend: "test", resolve: func(context.Context, Reference) (sourceMaterial, error) {
				reads++
				if reads == 2 {
					return sourceMaterial{}, newSourceError(kind, "test")
				}
				value, version := "valid-old", "old"
				if reads > 2 {
					value, version = "valid-new", "new"
				}
				return sourceMaterial{
					values: map[string]string{"value": value}, version: version,
					lease: &sources.ProviderCredentialLease{RefreshAfter: time.Now().Add(-time.Second)},
				}, nil
			}}
			provider, resolver := testReferenceResolver(t, source)
			if _, err := resolver.ResolveCatalog(t.Context(), &provider); err != nil {
				t.Fatal(err)
			}
			material, err := resolver.ResolveCatalog(t.Context(), &provider)
			if !isSourceError(err, kind) {
				t.Fatalf("backend failure = %v", err)
			}
			if _, exists := material.Value("api-key"); exists {
				t.Fatal("backend failure returned stale material")
			}
			material, err = resolver.ResolveCatalog(t.Context(), &provider)
			if err != nil {
				t.Fatal(err)
			}
			if value, _ := material.Value("api-key"); value != "valid-new" || reads != 3 {
				t.Fatal("backend recovery reused stale material")
			}
		})
	}
}
