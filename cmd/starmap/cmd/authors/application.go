package authors

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/pkg/catalogs"
)

type application interface {
	Catalog() (*catalogs.Catalog, error)
	Logger() *zerolog.Logger
}
