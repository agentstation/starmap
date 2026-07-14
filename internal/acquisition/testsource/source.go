// Package testsource provides explicit protocol-fixture source construction.
// It is imported only by tests and development fixture tooling.
package testsource

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Unauthenticated resolves the only configured source after cloning it into
// an explicitly unauthenticated protocol fixture. Invocation metadata is not
// part of catalog-response protocol tests.
func Unauthenticated(t testing.TB, provider *catalogs.Provider) acquisition.Source {
	t.Helper()
	if provider == nil || provider.Catalog == nil || len(provider.Catalog.Sources) != 1 {
		t.Fatalf("protocol fixture provider must contain exactly one source")
	}
	clone := catalogs.DeepCopyProvider(*provider)
	clone.Credentials = nil
	clone.Invocation = nil
	clone.Catalog.Sources[0].Auth = catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone}
	return resolve(t, &clone)
}

// Authenticated resolves the only configured source with its real auth policy.
func Authenticated(t testing.TB, provider *catalogs.Provider) acquisition.Source {
	t.Helper()
	if provider == nil || provider.Catalog == nil || len(provider.Catalog.Sources) != 1 {
		t.Fatalf("authenticated fixture provider must contain exactly one source")
	}
	clone := catalogs.DeepCopyProvider(*provider)
	clone.Invocation = nil
	return resolve(t, &clone)
}

func resolve(t testing.TB, provider *catalogs.Provider) acquisition.Source {
	t.Helper()
	if provider == nil || provider.Catalog == nil || len(provider.Catalog.Sources) != 1 {
		t.Fatalf("fixture provider must contain exactly one source")
	}
	resolved, err := acquisition.NewResolver().Resolve(context.Background(), provider, provider.Catalog.Sources[0].ID)
	if err != nil {
		t.Fatalf("resolve fixture source: %v", err)
	}
	return resolved
}
