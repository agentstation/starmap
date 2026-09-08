package app

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joho/godotenv"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/errors"
)

// DotenvConflict identifies a duplicate key without retaining either value.
type DotenvConflict struct {
	Name         string
	ReplacedFile string
	SelectedFile string
}

type dotenvResult struct {
	conflicts   []DotenvConflict
	layers      []catalogconfig.Layer
	paths       []pathInput
	legacyNames []string
}

// loadExplicitEnvFiles preserves process values and lets later files replace earlier files.
// List .env.local after .env. Empty process values also prevent file fallback.
func loadExplicitEnvFiles(paths []string) (dotenvResult, error) {
	values := make(map[string]string)
	origins := make(map[string]string)
	var result dotenvResult
	seen := make(map[string]bool)
	for _, suppliedPath := range paths {
		path, err := filepath.Abs(suppliedPath)
		if err != nil {
			return dotenvResult{}, &errors.ConfigError{Component: "dotenv", Message: "cannot resolve the explicit file path"}
		}
		if seen[path] {
			return dotenvResult{}, &errors.ConfigError{Component: "dotenv " + path, Message: "duplicates an explicit file"}
		}
		seen[path] = true
		contents, err := readPrivateInput(path, "dotenv")
		if err != nil {
			return dotenvResult{}, &errors.ConfigError{Component: "dotenv " + path, Message: "cannot read the explicit private file", Err: err}
		}
		parsed, err := godotenv.Unmarshal(string(contents))
		if err != nil {
			return dotenvResult{}, &errors.ConfigError{Component: "dotenv " + path, Message: "cannot parse the explicit environment file"}
		}
		result.paths = append(result.paths, pathEnvironmentInput("dotenv:"+path, parsed))
		input := catalogEnvironmentInput(parsed)
		result.layers = append(result.layers, catalogconfig.Layer{Name: "dotenv:" + path, Values: input.values})
		result.legacyNames = append(result.legacyNames, input.legacyNames...)
		for _, name := range slices.Sorted(maps.Keys(parsed)) {
			value := parsed[name]
			if name == "" || strings.ContainsAny(name, "=\x00") || strings.ContainsRune(value, '\x00') {
				return dotenvResult{}, &errors.ConfigError{Component: "dotenv " + path, Message: "contains an invalid environment entry"}
			}
			if previous, present := values[name]; present && previous != value {
				result.conflicts = append(result.conflicts, DotenvConflict{Name: name, ReplacedFile: origins[name], SelectedFile: path})
			}
			values[name], origins[name] = value, path
		}
	}
	// Parse every file before applying any environment change.
	for _, name := range slices.Sorted(maps.Keys(values)) {
		// Catalog values remain in named layers across repeated commands.
		if isCatalogEnvironmentName(name) || isPathEnvironmentName(name) {
			continue
		}
		if _, present := os.LookupEnv(name); present {
			continue
		}
		if err := os.Setenv(name, values[name]); err != nil {
			return dotenvResult{}, &errors.ConfigError{Component: "dotenv " + origins[name], Message: "cannot set the configured environment entry"}
		}
	}
	return result, nil
}
