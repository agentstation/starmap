package validate

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/pkg/catalogs"
)

type testApplication struct {
	CatalogFunc      func() (*catalogs.Catalog, error)
	OutputFormatFunc func() string
}

func (a *testApplication) Catalog() (*catalogs.Catalog, error) {
	if a.CatalogFunc != nil {
		return a.CatalogFunc()
	}
	return nil, nil
}

func (a *testApplication) Logger() *zerolog.Logger {
	logger := zerolog.Nop()
	return &logger
}

func (a *testApplication) OutputFormat() string {
	if a.OutputFormatFunc != nil {
		return a.OutputFormatFunc()
	}
	return "table"
}
