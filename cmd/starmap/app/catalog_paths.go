package app

import "github.com/agentstation/starmap/pkg/constants"

func (a *App) catalogDatabasePath() (string, error) {
	path := a.config.CatalogPath
	if path == "" {
		path = constants.DefaultCatalogDatabasePath
	}
	return expandHomePath(path)
}

func (a *App) catalogExportPath() (string, error) {
	if a.config.CatalogExportPath == "" {
		return "", nil
	}
	return expandHomePath(a.config.CatalogExportPath)
}
