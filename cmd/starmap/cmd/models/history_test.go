package models

import (
	"errors"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	starmaperrors "github.com/agentstation/starmap/pkg/errors"
)

func TestResolveHistoryProvider(t *testing.T) {
	t.Parallel()

	catalog := historyCatalog(t,
		catalogs.Provider{ID: "provider-a", Aliases: []catalogs.ProviderID{"provider-a-alias"}, Name: "Provider A", Models: map[string]*catalogs.Model{
			"shared": {ID: "shared", Name: "Shared A"},
			"unique": {ID: "unique", Name: "Unique"},
		}},
		catalogs.Provider{ID: "provider-b", Name: "Provider B", Models: map[string]*catalogs.Model{
			"shared": {ID: "shared", Name: "Shared B"},
		}},
	)

	providerID, err := resolveHistoryProvider(catalog, "", "unique")
	if err != nil || providerID != "provider-a" {
		t.Fatalf("unique model resolution = (%q, %v)", providerID, err)
	}

	providerID, err = resolveHistoryProvider(catalog, "provider-b", "shared")
	if err != nil || providerID != "provider-b" {
		t.Fatalf("explicit provider resolution = (%q, %v)", providerID, err)
	}

	providerID, err = resolveHistoryProvider(catalog, "provider-a-alias", "unique")
	if err != nil || providerID != "provider-a" {
		t.Fatalf("provider alias resolution = (%q, %v), want canonical provider-a", providerID, err)
	}

	if _, err = resolveHistoryProvider(catalog, "", "shared"); err == nil {
		t.Fatal("ambiguous shared model resolved without a provider")
	} else {
		var validation *starmaperrors.ValidationError
		if !errors.As(err, &validation) || validation.Field != "provider" {
			t.Fatalf("ambiguous error = %T %v, want provider ValidationError", err, err)
		}
	}

	if _, err = resolveHistoryProvider(catalog, "", "missing"); err == nil {
		t.Fatal("missing model resolved")
	} else {
		var notFound *starmaperrors.NotFoundError
		if !errors.As(err, &notFound) {
			t.Fatalf("missing error = %T %v, want NotFoundError", err, err)
		}
	}
}

func historyCatalog(t testing.TB, providers ...catalogs.Provider) *catalogs.Catalog {
	t.Helper()

	builder := catalogs.NewEmpty()
	for _, provider := range providers {
		if err := builder.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider(%s): %v", provider.ID, err)
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return catalog
}
