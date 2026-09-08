package app

import (
	"context"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime"
)

// CatalogAcquisition composes manual acquisition for CLI updates and HTTP requests.
// It applies the same provider binding set as the connected runtime.
// Source startup follows the configured runtime policy.
func (a *App) CatalogAcquisition(client *starmap.Client) (*acquisition.Syncer, error) {
	if client == nil {
		return nil, &errors.ValidationError{Field: "acquisition.client", Message: "is required"}
	}
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
	connected, err := a.Runtime(context.Background(), runtime.WithSourcePollInterval(0), runtime.WithAcquisitionEnabled(false), runtime.WithClientOptions(starmap.WithCatalogPath(client.WorkspacePath())))
	if err != nil {
		return nil, err
	}
	return acquisition.NewForRuntime(connected, options...)
}
