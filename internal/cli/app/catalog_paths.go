package app

import "github.com/agentstation/starmap/internal/constants"

func (a *App) catalogStatePath() (string, error) {
	return expandHomePath(constants.DefaultCatalogStatePath)
}

// CatalogPath returns the configured human provider-YAML workspace. The
// canonical workspace setting wins, so every composition in one process reads
// one workspace. Without any override, it returns the per-user default.
func (a *App) CatalogPath() (string, error) {
	path := a.catalogSettings.WorkspacePath
	if path == "" {
		path = a.config.CatalogPath
	}
	if path == "" {
		path = constants.DefaultCatalogPath
	}
	return expandHomePath(path)
}
