package starmap

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

type hookCountingReader struct {
	catalogs.Reader
	reads int
}

func (r *hookCountingReader) Providers() catalogs.ProvidersReader {
	r.reads++
	return r.Reader.Providers()
}

func TestModelDiffWithoutObserversDoesNotReadCatalogs(t *testing.T) {
	for _, publicationOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-hooks", true: "publication-only"}[publicationOnly], func(t *testing.T) {
			reader := &hookCountingReader{Reader: mustTestCatalog(t, catalogs.NewEmpty())}
			hooks := newHooks()
			if publicationOnly {
				hooks.OnCatalogPublished(func(CatalogPublishedEvent) error { return nil })
			}
			hooks.triggerUpdate(reader, reader)
			if reader.reads != 0 {
				t.Fatalf("model diff read catalogs %d times without model observers", reader.reads)
			}
		})
	}
}
