package starmap_test

import (
	"os"
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

func TestEmbeddedGenerationIsVerifiedPassiveAndIndependent(t *testing.T) {
	directory := t.TempDir()
	t.Chdir(directory)
	t.Setenv("HOME", directory)
	t.Setenv("STARMAP_CATALOG_SOURCE", "starmap")
	t.Setenv("STARMAP_CATALOG_SOURCE_URL", "http://127.0.0.1:1")
	original, err := starmap.EmbeddedGeneration()
	if err != nil {
		t.Fatal(err)
	}
	if err := original.Validate(); err != nil {
		t.Fatal(err)
	}
	if original.Manifest.GenerationID == "" || len(original.Payload) == 0 {
		t.Fatal("embedded generation has no identity or payload")
	}
	checksum := original.Manifest.Payload.Checksum
	original.Payload[0] ^= 0xff
	original.Manifest.Validation.Checks[0].Name = "caller-mutation"
	original.Manifest.SourceObservations[0].ObservationID = "caller-mutation"
	next, err := starmap.EmbeddedGeneration()
	if err != nil {
		t.Fatal(err)
	}
	if err := next.Validate(); err != nil {
		t.Fatal(err)
	}
	if next.Manifest.Payload.Checksum != checksum || next.Manifest.Validation.Checks[0].Name == "caller-mutation" || next.Manifest.SourceObservations[0].ObservationID == "caller-mutation" {
		t.Fatal("a caller changed the embedded generation returned to another caller")
	}
	files, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatal("passive embedded access created application files")
	}
}
