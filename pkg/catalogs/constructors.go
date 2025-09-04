package catalogs

import (
	"io/fs"
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

// NewEmbedded creates a catalog backed by embedded files.
// This is the recommended catalog for production use as it includes
// all model data compiled into the binary.
func NewEmbedded() (Catalog, error) {
	return New(WithEmbedded())
}

// NewFiles creates a catalog backed by files on disk.
// This is useful for development when you want to edit catalog files
// without recompiling the binary.
//
// Example:
//
//	catalog, err := NewFiles("./internal/embedded/catalog")
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewFiles(path string) (Catalog, error) {
	// Verify path exists
	if _, err := os.Stat(path); err != nil {
		return nil, errors.WrapIO("stat", path, err)
	}
	return New(WithFiles(path))
}

// NewMemory creates an in-memory catalog.
// This is useful for testing or temporary catalogs that don't
// need persistence.
//
// Example:
//
//	catalog := NewMemory()
//	provider := Provider{ID: "openai", Models: map[string]Model{}}
//	catalog.SetProvider(provider)
func NewMemory() Catalog {
	// Memory catalog cannot fail
	catalog, _ := New() // No options = memory catalog
	return catalog
}

// NewFromFS creates a catalog from a custom filesystem implementation.
// This allows for advanced use cases like virtual filesystems or
// custom storage backends.
//
// Example:
//
//	var myFS embed.FS
//	catalog, err := NewFromFS(myFS, "catalog")
func NewFromFS(fsys fs.FS, root string) (Catalog, error) {
	subFS, err := fs.Sub(fsys, root)
	if err != nil {
		return nil, errors.WrapResource("create", "sub filesystem", root, err)
	}
	return New(WithFS(subFS))
}

// NewReadOnly creates a read-only wrapper around an existing catalog.
// Write operations will return errors.
//
// Example:
//
//	embedded, _ := NewEmbedded()
//	readOnly := NewReadOnly(embedded)
//	err := readOnly.SetProvider(provider) // Returns error
func NewReadOnly(source Catalog) Catalog {
	return &readOnlyCatalog{source: source}
}

// Compile-time interface checks for readOnlyCatalog
var (
	_ Catalog         = (*readOnlyCatalog)(nil)
	_ ReadOnlyCatalog = (*readOnlyCatalog)(nil)
)

// readOnlyCatalog wraps a catalog to make it read-only
type readOnlyCatalog struct {
	source Catalog
}

func (r *readOnlyCatalog) Providers() *Providers {
	return r.source.Providers()
}

func (r *readOnlyCatalog) Authors() *Authors {
	return r.source.Authors()
}

func (r *readOnlyCatalog) Endpoints() *Endpoints {
	return r.source.Endpoints()
}

func (r *readOnlyCatalog) Provider(id ProviderID) (Provider, error) {
	return r.source.Provider(id)
}

func (r *readOnlyCatalog) Author(id AuthorID) (Author, error) {
	return r.source.Author(id)
}

func (r *readOnlyCatalog) Endpoint(id string) (Endpoint, error) {
	return r.source.Endpoint(id)
}

func (r *readOnlyCatalog) GetAllModels() []Model {
	return r.source.GetAllModels()
}

func (r *readOnlyCatalog) FindModel(id string) (Model, error) {
	return r.source.FindModel(id)
}

func (r *readOnlyCatalog) SetProvider(provider Provider) error {
	return errors.ErrReadOnly
}

func (r *readOnlyCatalog) SetAuthor(author Author) error {
	return errors.ErrReadOnly
}

func (r *readOnlyCatalog) SetEndpoint(endpoint Endpoint) error {
	return errors.ErrReadOnly
}

func (r *readOnlyCatalog) DeleteProvider(id ProviderID) error {
	return errors.ErrReadOnly
}

func (r *readOnlyCatalog) DeleteAuthor(id AuthorID) error {
	return errors.ErrReadOnly
}

func (r *readOnlyCatalog) DeleteEndpoint(id string) error {
	return errors.ErrReadOnly
}

func (r *readOnlyCatalog) ReplaceWith(source Reader) error {
	return errors.ErrReadOnly
}

func (r *readOnlyCatalog) MergeWith(source Reader, opts ...MergeOption) error {
	return errors.ErrReadOnly
}

func (r *readOnlyCatalog) Copy() (Catalog, error) {
	// Copy is allowed - returns another read-only wrapper
	sourceCopy, err := r.source.Copy()
	if err != nil {
		return nil, err
	}
	return &readOnlyCatalog{source: sourceCopy}, nil
}

func (r *readOnlyCatalog) MergeStrategy() MergeStrategy {
	return r.source.MergeStrategy()
}

func (r *readOnlyCatalog) SetMergeStrategy(strategy MergeStrategy) {
	// Silently ignore - read-only
}
