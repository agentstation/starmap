package app

import "github.com/agentstation/starmap/internal/constants"

func (a *App) catalogStatePath() (string, error) {
	return expandHomePath(constants.DefaultCatalogStatePath)
}

// CatalogPath returns the configured human provider-YAML workspace, or the
// canonical per-user default when no override is configured.
func (a *App) CatalogPath() (string, error) {
	path := a.config.CatalogPath
	if path == "" {
		path = constants.DefaultCatalogPath
	}
	return expandHomePath(path)
}
