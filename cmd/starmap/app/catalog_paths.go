package app

import "github.com/agentstation/starmap/pkg/constants"

func (a *App) catalogDatabasePath() (string, error) {
	path := a.config.CatalogPath
	if path == "" {
		path = constants.DefaultCatalogDatabasePath
	}
	return expandHomePath(path)
}

// CatalogExportPath returns the configured editable catalog export path, or the
// canonical per-user default when no override is configured.
func (a *App) CatalogExportPath() (string, error) {
	path := a.config.CatalogExportPath
	if path == "" {
		path = constants.DefaultCatalogExportPath
	}
	return expandHomePath(path)
}

func (a *App) configuredCatalogExportPath() (string, error) {
	if a.config.CatalogExportPath == "" {
		return "", nil
	}
	return expandHomePath(a.config.CatalogExportPath)
}
