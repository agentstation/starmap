package update

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/productpaths"
	"github.com/agentstation/starmap/pkg/sources"
)

type application interface {
	Starmap(...starmap.Option) (*starmap.Client, error)
	Logger() *zerolog.Logger
	CredentialResolver() (sources.ProviderCredentialResolver, error)
	SourceDirectories() (productpaths.SourceDirectories, error)
	ResolveOperationPath(string, string) (string, error)
}
