package catalogs

import (
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/save"
)

// NewReadOnly creates a read-only wrapper around an existing catalog.
// Write operations will return errors.
//
// Example:
//
//	embedded, _ := NewEmbedded()
//	readOnly := NewReadOnly(embedded)
//	err := readOnly.SetProvider(provider) // Returns error
func NewReadOnly(source Catalog) Catalog {
	return &readonly{source: source}
}

// Compile-time interface checks for readOnlyCatalog.
var (
	_ Catalog         = (*readonly)(nil)
	_ ReadOnlyCatalog = (*readonly)(nil)
)

// readonly wraps a catalog to make it read-only.
type readonly struct {
	source Catalog
}

// Providers implements starmap.Catalog.
func (r *readonly) Providers() *Providers { return r.source.Providers() }

// Authors implements starmap.Catalog.
func (r *readonly) Authors() *Authors { return r.source.Authors() }

// Endpoints implements starmap.Catalog.
func (r *readonly) Endpoints() *Endpoints { return r.source.Endpoints() }

// Provenance implements starmap.Catalog.
func (r *readonly) Provenance() *Provenance { return r.source.Provenance() }

// Provider implements starmap.Catalog.
func (r *readonly) Provider(id ProviderID) (Provider, error) { return r.source.Provider(id) }

// Author implements starmap.Catalog.
func (r *readonly) Author(id AuthorID) (Author, error) { return r.source.Author(id) }

// Endpoint implements starmap.Catalog.
func (r *readonly) Endpoint(id string) (Endpoint, error) { return r.source.Endpoint(id) }

// Models implements starmap.Catalog.
func (r *readonly) Models() *Models { return r.source.Models() }

// FindModel implements starmap.Catalog.
func (r *readonly) FindModel(id string) (Model, error) { return r.source.FindModel(id) }

// SetProvider implements starmap.Catalog.
func (r *readonly) SetProvider(_ Provider) error { return errors.ErrReadOnly }

// SetAuthor implements starmap.Catalog.
func (r *readonly) SetAuthor(_ Author) error { return errors.ErrReadOnly }

// SetEndpoint implements starmap.Catalog.
func (r *readonly) SetEndpoint(_ Endpoint) error { return errors.ErrReadOnly }

// DeleteProvider implements starmap.Catalog.
func (r *readonly) DeleteProvider(_ ProviderID) error { return errors.ErrReadOnly }

// DeleteAuthor implements starmap.Catalog.
func (r *readonly) DeleteAuthor(_ AuthorID) error { return errors.ErrReadOnly }

// DeleteEndpoint implements starmap.Catalog.
func (r *readonly) DeleteEndpoint(_ string) error { return errors.ErrReadOnly }

// ReplaceWith implements starmap.Catalog.
func (r *readonly) ReplaceWith(_ Reader) error { return errors.ErrReadOnly }

// MergeWith implements starmap.Catalog.
func (r *readonly) MergeWith(_ Reader, _ ...MergeOption) error { return errors.ErrReadOnly }

// Copy implements starmap.Catalog.
func (r *readonly) Copy() (Catalog, error) {
	sourceCopy, err := r.source.Copy()
	if err != nil {
		return nil, err
	}
	return &readonly{source: sourceCopy}, nil
}

// MergeStrategy implements starmap.Catalog.
func (r *readonly) MergeStrategy() MergeStrategy { return r.source.MergeStrategy() }

// SetMergeStrategy implements starmap.Catalog.
func (r *readonly) SetMergeStrategy(_ MergeStrategy) { /* Silently ignore - read-only */ }

// Save implements starmap.Catalog.
func (r *readonly) Save(_ ...save.Option) error { return errors.ErrReadOnly }
