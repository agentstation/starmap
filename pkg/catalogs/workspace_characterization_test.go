package catalogs

import "testing"

// TestF001CharacterizationEmbeddedBuilderCarriesRepositoryWritePath pins the
// current embedded write-path leak without writing to the repository. P3 must
// make embedded bytes read-only observations so an embedded builder cannot
// silently select a source-tree destination.
func TestF001CharacterizationEmbeddedBuilderCarriesRepositoryWritePath(t *testing.T) {
	builder, err := NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	const repositoryCatalogPath = "internal/embedded/catalog"
	if got := builder.config.resolveWritePath(""); got != repositoryCatalogPath {
		t.Fatalf(
			"F-001 characterization changed: embedded write path = %q, want %q",
			got,
			repositoryCatalogPath,
		)
	}
}
