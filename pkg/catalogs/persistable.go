package catalogs

// Persistable represents a catalog that can be saved to and loaded from persistent storage
type Persistable interface {
	// Save saves the catalog to persistent storage
	Save() error

	// SaveTo saves the catalog to a specific path
	SaveTo(path string) error

	// Load loads the catalog from persistent storage
	Load() error

	// Write saves the catalog to disk
	// If a path is provided, it saves to that location
	// Otherwise, it uses the configured location
	Write(paths ...string) error
}
