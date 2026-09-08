package app

import (
	"path/filepath"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths"
)

const relativePathBaseName = "STARMAP_RELATIVE_PATH_BASE"

// relativePathBase requires explicit intent before legacy selectors change anchors.
func relativePathBase(config *Config) (string, string, error) {
	for _, layer := range pathLayers(config) {
		if value, present := layer.values[relativePathBaseName]; present {
			if value != "" && value != "config" {
				return "", "", &errors.ValidationError{Field: relativePathBaseName, Message: "must be config or empty"}
			}
			return value, layer.name, nil
		}
	}
	return "", "unspecified", nil
}

func selectedLegacyLeaf(config *Config, root productpaths.Path, value, origin, field string) (productpaths.Path, error) {
	base, _, err := relativePathBase(config)
	if err != nil {
		return productpaths.Path{}, err
	}
	expanded, err := expandHomePath(value)
	if err != nil {
		return productpaths.Path{}, err
	}
	if expanded != "" && !filepath.IsAbs(expanded) && base != "config" {
		return productpaths.Path{}, &errors.ValidationError{Field: field, Message: "legacy relative anchor is unknown. Use the previous absolute path or explicitly select relative_path_base=config for configuration-root paths"}
	}
	return productpaths.Leaf(root, expanded, origin)
}

// ResolveOperationPath resolves an explicit legacy command path before catalog access.
func (a *App) ResolveOperationPath(value, field string) (string, error) {
	roots, err := resolveProductPaths(a.config)
	if err != nil {
		return "", err
	}
	selected, err := selectedLegacyLeaf(a.config, roots.Roots[productpaths.Config], value, "operation", field)
	if err != nil {
		return "", err
	}
	return selected.Path, nil
}
