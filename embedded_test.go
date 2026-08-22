package starmap_test

import (
	"testing"

	starmap "github.com/agentstation/starmap"
)

// TestEmbeddedBuilderCarriesIdentity proves the public embedded accessor
// yields the embedded generation with catalog-carried brand identity.
func TestEmbeddedBuilderCarriesIdentity(t *testing.T) {
	t.Parallel()

	builder, err := starmap.EmbeddedBuilder()
	if err != nil {
		t.Fatalf("EmbeddedBuilder: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	provider, err := catalog.Provider("anthropic")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if provider.Description == nil || *provider.Description == "" {
		t.Fatal("anthropic description missing from embedded catalog")
	}
	if len(provider.Logo) == 0 {
		t.Fatal("anthropic logo missing from embedded catalog")
	}
	author, err := catalog.Author("phind")
	if err != nil {
		t.Fatalf("Author: %v", err)
	}
	if len(author.Logo) == 0 {
		t.Fatal("phind logo missing from embedded catalog")
	}
}
