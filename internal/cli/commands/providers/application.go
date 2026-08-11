package providers

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

type application interface {
	Catalog() (*catalogs.Catalog, error)
	Logger() *zerolog.Logger
	OutputFormat() string
	CredentialResolver() (sources.ProviderCredentialResolver, error)
}

func providerCredentialComposition(
	app application,
	providers catalogs.ProvidersReader,
) (*sources.ProviderFetcher, *auth.Checker, error) {
	resolver, err := app.CredentialResolver()
	if err != nil {
		return nil, nil, err
	}
	return acquisition.NewProviderFetcher(
		providers,
		sources.WithProviderCredentialResolver(resolver),
	), auth.NewChecker(auth.WithCredentialResolver(resolver)), nil
}
