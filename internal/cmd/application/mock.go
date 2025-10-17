package application

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Mock provides a mock implementation of Application for testing.
// Each method can be customized by setting the corresponding function field.
// If a function field is nil, the method returns a default/zero value.
//
// Example Usage:
//
//	mock := &application.Mock{
//	    CatalogFunc: func() (catalogs.Catalog, error) {
//	        return testCatalog, nil
//	    },
//	    LoggerFunc: func() *zerolog.Logger {
//	        logger := zerolog.Nop()
//	        return &logger
//	    },
//	}
//	cmd := list.NewCommand(mock)
//	// ... test command
type Mock struct {
	CatalogFunc      func() (catalogs.Catalog, error)
	StarmapFunc      func(opts ...starmap.Option) (starmap.Client, error)
	LoggerFunc       func() *zerolog.Logger
	OutputFormatFunc func() string
	VersionFunc      func() string
	CommitFunc       func() string
	DateFunc         func() string
	BuiltByFunc      func() string
}

// Catalog returns a catalog using the mock function or nil.
func (m *Mock) Catalog() (catalogs.Catalog, error) {
	if m.CatalogFunc != nil {
		return m.CatalogFunc()
	}
	return nil, nil
}

// Starmap returns a starmap using the mock function or nil.
func (m *Mock) Starmap(opts ...starmap.Option) (starmap.Client, error) {
	if m.StarmapFunc != nil {
		return m.StarmapFunc(opts...)
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

// OutputFormat returns output format using the mock function or "table".
func (m *Mock) OutputFormat() string {
	if m.OutputFormatFunc != nil {
		return m.OutputFormatFunc()
	}
	return "table"
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

// Ensure Mock implements Application at compile time.
var _ Application = (*Mock)(nil)
