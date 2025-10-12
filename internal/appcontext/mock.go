package appcontext

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Mock provides a mock implementation of Interface for testing.
// Each method can be customized by setting the corresponding function field.
// If a function field is nil, the method returns a default/zero value.
type Mock struct {
	CatalogFunc             func() (catalogs.Catalog, error)
	StarmapFunc             func() (starmap.Starmap, error)
	StarmapWithOptionsFunc  func(...starmap.Option) (starmap.Starmap, error)
	LoggerFunc              func() *zerolog.Logger
	VersionFunc             func() string
	CommitFunc              func() string
	DateFunc                func() string
	BuiltByFunc             func() string
}

// Catalog returns a catalog using the mock function or nil.
func (m *Mock) Catalog() (catalogs.Catalog, error) {
	if m.CatalogFunc != nil {
		return m.CatalogFunc()
	}
	return nil, nil
}

// Starmap returns a starmap using the mock function or nil.
func (m *Mock) Starmap() (starmap.Starmap, error) {
	if m.StarmapFunc != nil {
		return m.StarmapFunc()
	}
	return nil, nil
}

// StarmapWithOptions returns a starmap using the mock function or nil.
func (m *Mock) StarmapWithOptions(opts ...starmap.Option) (starmap.Starmap, error) {
	if m.StarmapWithOptionsFunc != nil {
		return m.StarmapWithOptionsFunc(opts...)
	}
	return nil, nil
}

// Logger returns a logger using the mock function or a no-op logger.
func (m *Mock) Logger() *zerolog.Logger {
	if m.LoggerFunc != nil {
		return m.LoggerFunc()
	}
	logger := zerolog.Nop()
	return &logger
}

// Version returns version using the mock function or "dev".
func (m *Mock) Version() string {
	if m.VersionFunc != nil {
		return m.VersionFunc()
	}
	return "dev"
}

// Commit returns commit using the mock function or "unknown".
func (m *Mock) Commit() string {
	if m.CommitFunc != nil {
		return m.CommitFunc()
	}
	return "unknown"
}

// Date returns date using the mock function or "unknown".
func (m *Mock) Date() string {
	if m.DateFunc != nil {
		return m.DateFunc()
	}
	return "unknown"
}

// BuiltBy returns builtBy using the mock function or "test".
func (m *Mock) BuiltBy() string {
	if m.BuiltByFunc != nil {
		return m.BuiltByFunc()
	}
	return "test"
}

// Ensure Mock implements Interface at compile time.
var _ Interface = (*Mock)(nil)
