package app

import (
	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/acquisition"
)

// CatalogAcquisition composes manual acquisition for CLI updates and HTTP requests.
// It applies the same provider binding set as the connected runtime.
// The constructor reads no source or provider data.
func (a *App) CatalogAcquisition(client *starmap.Client) (*acquisition.Syncer, error) {
	resolver, err := a.CredentialResolver()
	if err != nil {
		return nil, err
	}
	directories, err := a.SourceDirectories()
	if err != nil {
		return nil, err
	}
	options := []acquisition.Option{
		acquisition.WithCredentialResolver(resolver),
		acquisition.WithSourceDirectories(directories),
	}
	bindings, present, err := a.catalogSettings.ProviderAcquisitionBindings()
	if err != nil {
		return nil, err
	}
	if present {
		options = append(options, acquisition.WithProviderBindings(bindings...))
	}
	return acquisition.New(client, options...)
}
