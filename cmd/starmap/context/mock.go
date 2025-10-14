package context

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// MockContext provides a mock implementation of Context for testing.
// Each method can be customized by setting the corresponding function field.
// If a function field is nil, the method returns a default/zero value.
//
// Example Usage:
//
//	mock := &context.MockContext{
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
type MockContext struct {
	CatalogFunc      func() (catalogs.Catalog, error)
	StarmapFunc      func(opts ...starmap.Option) (starmap.Starmap, error)
	LoggerFunc       func() *zerolog.Logger
	OutputFormatFunc func() string
	VersionFunc      func() string
	CommitFunc       func() string
	DateFunc         func() string
	BuiltByFunc      func() string
}

// Catalog returns a catalog using the mock function or nil.
func (m *MockContext) Catalog() (catalogs.Catalog, error) {
	if m.CatalogFunc != nil {
		return m.CatalogFunc()
	}
	return nil, nil
}

// Starmap returns a starmap using the mock function or nil.
func (m *MockContext) Starmap(opts ...starmap.Option) (starmap.Starmap, error) {
	if m.StarmapFunc != nil {
		return m.StarmapFunc(opts...)
	}
	return nil, nil
}

// Logger returns a logger using the mock function or a no-op logger.
func (m *MockContext) Logger() *zerolog.Logger {
	if m.LoggerFunc != nil {
		return m.LoggerFunc()
	}
	logger := zerolog.Nop()
	return &logger
}

// OutputFormat returns output format using the mock function or "table".
func (m *MockContext) OutputFormat() string {
	if m.OutputFormatFunc != nil {
		return m.OutputFormatFunc()
	}
	return "table"
}

// Version returns version using the mock function or "dev".
func (m *MockContext) Version() string {
	if m.VersionFunc != nil {
		return m.VersionFunc()
	}
	return "dev"
}

// Commit returns commit using the mock function or "unknown".
func (m *MockContext) Commit() string {
	if m.CommitFunc != nil {
		return m.CommitFunc()
	}
	return "unknown"
}

// Date returns date using the mock function or "unknown".
func (m *MockContext) Date() string {
	if m.DateFunc != nil {
		return m.DateFunc()
	}
	return "unknown"
}

// BuiltBy returns builtBy using the mock function or "test".
func (m *MockContext) BuiltBy() string {
	if m.BuiltByFunc != nil {
		return m.BuiltByFunc()
	}
	return "test"
}

// Ensure MockContext implements Context at compile time.
var _ Context = (*MockContext)(nil)
