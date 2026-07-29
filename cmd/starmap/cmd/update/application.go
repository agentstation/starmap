package update

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
)

type application interface {
	Starmap(...starmap.Option) (*starmap.Client, error)
	Logger() *zerolog.Logger
}
