package app

import "github.com/agentstation/starmap/pkg/productpaths"

func (a *App) catalogStatePath() (string, error) {
	paths, err := a.ResolvedPaths()
	if err != nil {
		return "", err
	}
	return paths.CatalogStore.Path, nil
}

// CatalogPath returns the resolved human catalog workspace without creating it.
// An explicit empty canonical workspace disables the optional workspace.
func (a *App) CatalogPath() (string, error) {
	paths, err := a.ResolvedPaths()
	if err != nil {
		return "", err
	}
	return paths.Workspace.Path, nil
}

// SourceDirectories returns the configured source cache and checkout roots without creating files.
func (a *App) SourceDirectories() (productpaths.SourceDirectories, error) {
	paths, err := a.ResolvedPaths()
	if err != nil {
		return productpaths.SourceDirectories{}, err
	}
	return productpaths.SourceDirectoriesAt(paths.Roots[productpaths.Cache].Path)
}
