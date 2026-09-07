package app

import (
	"os"
	"path/filepath"
	"strings"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths"
	"github.com/agentstation/starmap/runtime"
)

type pathInput struct {
	name   string
	values map[string]string
}

type pathSetting struct {
	name, key, flag string
	root            productpaths.Root
	description     string
}

func pathSettings() []pathSetting {
	return []pathSetting{
		{"STARMAP_HOME", "home", "home", productpaths.Home, "absolute parent for configuration, data, state, and cache"},
		{"STARMAP_CONFIG_DIR", "config_dir", "config-dir", productpaths.Config, "absolute configuration root"},
		{"STARMAP_DATA_DIR", "data_dir", "data-dir", productpaths.Data, "absolute durable data root"},
		{"STARMAP_STATE_ROOT", "state_root", "state-root", productpaths.State, "absolute process state root"},
		{"STARMAP_CACHE_DIR", "cache_dir", "cache-dir", productpaths.Cache, "absolute disposable cache root"},
		{"STARMAP_DEPLOYMENT_ID", "deployment_id", "deployment-id", "", "persistent deployment identity (default: local)"},
		{"STARMAP_INSTANCE_ID", "instance_id", "instance-id", "", "portable instance name for one process (default: default)"},
		{"STARMAP_CATALOG_STORE_PATH", "catalog_store_path", "catalog-store-path", "", "catalog store path, relative to the configuration root unless absolute"},
		{relativePathBaseName, "relative_path_base", "relative-path-base", "", "explicit relative leaf anchor: config (unset requires absolute legacy selectors)"},
	}
}

func isPathEnvironmentName(name string) bool {
	for _, setting := range pathSettings() {
		if setting.name == name {
			return true
		}
	}
	return false
}

func pathEnvironmentInput(name string, values map[string]string) pathInput {
	result := pathInput{name: name, values: make(map[string]string)}
	for _, setting := range pathSettings() {
		if value, present := values[setting.name]; present {
			result.values[setting.name] = value
		}
	}
	return result
}

func pathValuesFromFile(values map[string]any) (map[string]string, error) {
	result := make(map[string]string)
	for _, setting := range pathSettings() {
		if value, present := values[setting.key]; present {
			text, ok := value.(string)
			if !ok {
				return nil, &errors.ValidationError{Field: setting.key, Message: "must be text"}
			}
			result[setting.name] = text
		}
	}
	return result, nil
}

// ProductPaths reports the selected roots and stable catalog paths without creating files.
type ProductPaths struct {
	Roots                   productpaths.Roots `json:"roots"`
	Configuration           productpaths.Path  `json:"configuration"`
	Workspace               productpaths.Path  `json:"workspace"`
	CatalogStore            productpaths.Path  `json:"catalog_store"`
	Runtime                 productpaths.Path  `json:"runtime"`
	Baselines               productpaths.Path  `json:"baselines"`
	SourceCache             productpaths.Path  `json:"source_cache"`
	SourceCheckout          productpaths.Path  `json:"source_checkout"`
	SourceFile              productpaths.Path  `json:"source_file,omitzero"`
	DeploymentID            string             `json:"deployment_id"`
	InstanceID              string             `json:"instance_id"`
	SchedulerIdentity       string             `json:"scheduler_identity,omitempty"`
	SchedulerIdentityOrigin string             `json:"scheduler_identity_origin"`
	RelativePathBase        string             `json:"relative_path_base"`
	RelativePathBaseOrigin  string             `json:"relative_path_base_origin"`
}

// ResolvedPaths uses the same directory resolution as application startup.
func (a *App) ResolvedPaths() (ProductPaths, error) {
	result, err := resolveProductPaths(a.config)
	if err != nil {
		return ProductPaths{}, err
	}
	apply := func(name string, target *productpaths.Path, optional bool) error {
		value, present := a.catalogSettings.Value(name)
		if !present {
			return nil
		}
		origin := a.config.CatalogOrigins[name]
		if origin == "" {
			origin = "catalog-settings"
		}
		if optional && value == "" {
			*target = productpaths.Path{Origin: origin}
			return nil
		}
		path, err := selectedLegacyLeaf(a.config, result.Roots[productpaths.Config], value, origin, name)
		if err != nil {
			return err
		}
		*target = path
		return nil
	}
	if _, present := a.catalogSettings.Value(catalogconfig.WorkspacePath); !present && a.config.CatalogPath != "" {
		result.Workspace, err = selectedLegacyLeaf(a.config, result.Roots[productpaths.Config], a.config.CatalogPath, "catalog_path", "catalog_path")
		if err != nil {
			return ProductPaths{}, err
		}
	}
	if err := apply(catalogconfig.WorkspacePath, &result.Workspace, true); err != nil {
		return ProductPaths{}, err
	}
	if err := apply(catalogconfig.StateDirectory, &result.Runtime, false); err != nil {
		return ProductPaths{}, err
	}
	if a.catalogSettings.SourceKind == runtime.SourceFile {
		origin := a.config.CatalogOrigins[catalogconfig.SourceURL]
		if origin == "" {
			origin = "catalog-settings"
		}
		result.SourceFile, err = selectedLegacyLeaf(a.config, result.Roots[productpaths.Config], a.catalogSettings.SourceURL, origin, catalogconfig.SourceURL)
		if err != nil {
			return ProductPaths{}, err
		}
	}
	result.SchedulerIdentityOrigin = "derived"
	if value, present := a.catalogSettings.Value(catalogconfig.SchedulerIdentity); present {
		result.SchedulerIdentity = value
		result.SchedulerIdentityOrigin = a.config.CatalogOrigins[catalogconfig.SchedulerIdentity]
		if result.SchedulerIdentityOrigin == "" {
			result.SchedulerIdentityOrigin = "catalog-settings"
		}
	}
	return result, nil
}

func pathLayers(config *Config) []pathInput {
	var layers []pathInput
	if config != nil && len(config.pathOverrides) > 0 {
		layers = append(layers, pathInput{"options", config.pathOverrides})
	}
	if config != nil && !config.catalogFileRead && len(config.PathValues) > 0 {
		layers = append(layers, pathInput{"go-configuration", config.PathValues})
	}
	environment := make(map[string]string)
	for _, setting := range pathSettings() {
		if value, present := os.LookupEnv(setting.name); present {
			environment[setting.name] = value
		}
	}
	layers = append(layers, pathInput{"environment", environment})
	if config != nil {
		for i := len(config.dotenvPaths) - 1; i >= 0; i-- {
			layers = append(layers, config.dotenvPaths[i])
		}
		if config.catalogFileRead {
			layers = append(layers, pathInput{"configuration-file", config.PathValues})
		}
	}
	return layers
}

func resolveProductPaths(config *Config) (ProductPaths, error) {
	base, baseOrigin, err := relativePathBase(config)
	if err != nil {
		return ProductPaths{}, err
	}
	rootLayers := productRootLayers(config)
	layers := pathLayers(config)
	for _, layer := range layers {
		for name := range layer.values {
			if !isPathEnvironmentName(name) {
				return ProductPaths{}, &errors.ValidationError{Field: name, Message: "unknown node path setting"}
			}
		}
	}
	if config != nil && config.configRoot.Path != "" {
		rootLayers = append([]productpaths.Layer{{Name: "selected-configuration-root", Values: map[productpaths.Root]string{productpaths.Config: config.configRoot.Path}}}, rootLayers...)
	}
	roots, err := productpaths.Resolve(productpaths.UserDefaults(productpaths.Starmap), rootLayers...)
	if err != nil {
		return ProductPaths{}, err
	}
	if config != nil && config.configRoot.Path != "" {
		roots[productpaths.Config] = config.configRoot
	}
	selected := func(name string) (string, string, bool) {
		for _, layer := range layers {
			if value, present := layer.values[name]; present {
				return value, layer.name, true
			}
		}
		return "", "", false
	}
	instance := "default"
	if value, _, present := selected("STARMAP_INSTANCE_ID"); present {
		instance = value
	}
	deployment := "local"
	if value, _, present := selected("STARMAP_DEPLOYMENT_ID"); present {
		deployment = value
	}
	if err := (runtime.DirectoryOwner{Product: "starmap", Deployment: deployment, Instance: instance}).Validate(); err != nil {
		return ProductPaths{}, err
	}
	child := func(root productpaths.Root, parts ...string) productpaths.Path {
		base := roots[root]
		return productpaths.Path{Path: filepath.Join(append([]string{base.Path}, parts...)...), Origin: base.Origin, Anchor: base.Path}
	}
	result := ProductPaths{Roots: roots, Configuration: child(productpaths.Config, "config.yaml"), Workspace: child(productpaths.Data, "catalog", "workspace"),
		CatalogStore: child(productpaths.State, "catalog"), Runtime: child(productpaths.State, "catalog", "runtime", instance), Baselines: child(productpaths.Data, "catalog", "baseline"), InstanceID: instance, DeploymentID: deployment, SourceCache: child(productpaths.Cache, "models.dev"), SourceCheckout: child(productpaths.Cache, "sources", "models.dev-git")}
	result.RelativePathBase, result.RelativePathBaseOrigin = base, baseOrigin
	if config != nil && config.ConfigFile != "" {
		result.Configuration, err = selectedLegacyLeaf(config, roots[productpaths.Config], config.ConfigFile, "selected-file", "config")
		if err != nil {
			return ProductPaths{}, err
		}
	}
	if value, origin, present := selected("STARMAP_CATALOG_STORE_PATH"); present {
		result.CatalogStore, err = productpaths.Leaf(roots[productpaths.Config], value, origin)
		if err != nil {
			return ProductPaths{}, err
		}
	}
	return result, nil
}

func pathKeyKnown(key string) bool {
	for _, setting := range pathSettings() {
		if setting.key == strings.ToLower(key) {
			return true
		}
	}
	return false
}

func productRootLayers(config *Config) []productpaths.Layer {
	layers := pathLayers(config)
	rootLayers := make([]productpaths.Layer, 0, len(layers))
	for _, layer := range layers {
		roots := make(map[productpaths.Root]string)
		for _, setting := range pathSettings() {
			if setting.root == "" {
				continue
			}
			if config != nil && config.configRoot.Path != "" && layer.name == "configuration-file" && setting.root == productpaths.Config {
				continue
			}
			if value, present := layer.values[setting.name]; present {
				roots[setting.root] = value
			}
		}
		rootLayers = append(rootLayers, productpaths.Layer{Name: layer.name, Values: roots})
	}

	return rootLayers
}
