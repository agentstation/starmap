package update

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/acquisition"
)

type application interface {
	Starmap(...starmap.Option) (*starmap.Client, error)
	Logger() *zerolog.Logger
	CatalogAcquisition(*starmap.Client) (*acquisition.Syncer, error)
	ResolveOperationPath(string, string) (string, error)
}
