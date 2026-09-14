package auth

import (
	"context"
	stderrors "errors"
	"testing"
	"time"
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
