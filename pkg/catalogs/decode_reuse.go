package catalogs

import (
	"crypto/sha256"
	"slices"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
)

// decodedCatalogs holds the complete catalogs that open reuse scopes retain.
var decodedCatalogs decodedCatalogReuse

// decodedCatalogReuse retains complete immutable catalogs while a scope is open.
// The SHA-256 digest of the complete payload bytes identifies each catalog.
type decodedCatalogReuse struct {
	mu      sync.Mutex
	scopes  int
	entries []decodedCatalogEntry
}

type decodedCatalogEntry struct {
	digest  [sha256.Size]byte
	catalog *Catalog
}

// RetainDecodedCatalogs opens a reuse scope for one bounded operation.
// Use it when the operation decodes the same catalog payload many times.
//
// In an open scope, DecodeCatalogPayload first compares the complete payload bytes.
// Equal bytes return the immutable catalog from the earlier complete decode.
// DecodeCatalogGeneration then applies each manifest check to that catalog.
// A changed byte selects a full decode.
//
// The scope retains a bounded count of complete catalogs.
// It never retains a failed decode or a partial diagnostic result.
// It never retains a source observation.
// The scope supplies no authority and replaces no caller check.
//
// Call release when the operation ends. The last release drops every retained catalog.
// More than one call to the same release function has no effect.
// Without an open scope, each call decodes the payload in full.
func RetainDecodedCatalogs() (release func()) {
	decodedCatalogs.open()
	return sync.OnceFunc(decodedCatalogs.close)
}

func (r *decodedCatalogReuse) open() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scopes++
}

func (r *decodedCatalogReuse) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scopes--
	if r.scopes == 0 {
		r.entries = nil
	}
}

func (r *decodedCatalogReuse) active() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.scopes > 0
}

// load returns the retained catalog for digest and marks it most recently used.
func (r *decodedCatalogReuse) load(digest [sha256.Size]byte) *Catalog {
	r.mu.Lock()
	defer r.mu.Unlock()
	index := slices.IndexFunc(r.entries, func(entry decodedCatalogEntry) bool { return entry.digest == digest })
	if index < 0 {
		return nil
	}
	entry := r.entries[index]
	r.entries = append(slices.Delete(r.entries, index, index+1), entry)
	return entry.catalog
}

// store retains one complete catalog and returns the catalog that the scope owns.
// A concurrent decode of equal bytes keeps the first retained catalog.
// A closed scope retains nothing.
func (r *decodedCatalogReuse) store(digest [sha256.Size]byte, catalog *Catalog) *Catalog {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.scopes == 0 {
		return catalog
	}
	for _, entry := range r.entries {
		if entry.digest == digest {
			return entry.catalog
		}
	}
	if len(r.entries) >= resourcepolicy.MaxRetainedDecodedCatalogs {
		r.entries = slices.Delete(r.entries, 0, 1)
	}
	r.entries = append(r.entries, decodedCatalogEntry{digest: digest, catalog: catalog})
	return catalog
}
