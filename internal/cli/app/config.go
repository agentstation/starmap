package app

import (
	"bytes"
	"crypto/sha256"
	stderrors "errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/agentstation/starmap/internal/privatefiles"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

// Config holds the application configuration loaded from various sources
// including config files, environment variables, and .env files.
type Config struct {
	// Global flags
	Verbose bool
	Quiet   bool
	NoColor bool
	Output  string

	// Config file
	ConfigFile       string
	configFileDigest [sha256.Size]byte
	// PathValues retains the selected file's node directory and identity settings.
	PathValues    map[string]string
	pathOverrides map[string]string
	dotenvPaths   []pathInput
	configRoot    productpaths.Path

	// Starmap configuration
	// CatalogPath is the human-editable provider YAML workspace.
	CatalogPath                   string
	EmbeddedBootstrapMaxAge       time.Duration
	EmbeddedBootstrapMaxSizeBytes int64
	RemoteServerURL               string
	RemoteServerAPIKey            string
	RemoteServerOnly              bool
	CredentialSources             map[string]map[string]CredentialSourceConfig
	// CatalogValues retains canonical file values before environment resolution.
	CatalogValues map[string]string
	// CatalogOrigins and CatalogIgnored identify configuration authorities without values.
	CatalogOrigins map[string]string
	CatalogIgnored []catalogconfig.Ignored
	// DotenvConflicts lists safe duplicate-key diagnostics from explicit files.
	DotenvConflicts    []DotenvConflict
	LegacyCatalogNames []string
	dotenvLayers       []catalogconfig.Layer
	catalogFileRead    bool

	// Logging configuration
	LogLevel  string
	LogFormat string
	LogOutput string
}

// CredentialSourceConfig selects one explicit provider-field source.
type CredentialSourceConfig struct {
	Reference       string `mapstructure:"reference"`
	FallbackAmbient bool   `mapstructure:"fallback_ambient"`
}

// LoadConfig loads configuration from all sources in order of precedence:
// 1. Command-line flags (handled by cobra)
// 2. Environment variables
// 3. Explicit dotenv files loaded by command composition
// 4. config.yaml under the selected configuration root
// 5. Defaults.
func LoadConfig() (*Config, error) {
	return loadConfig("")
}

// loadConfig loads configuration, using configFile when it is non-empty.
// An explicit file must exist and parse successfully. The default file remains optional.
func loadConfig(configFile string) (*Config, error) { return loadConfigWithPaths(configFile, nil) }

func loadConfigWithPaths(configFile string, bootstrap *Config) (*Config, error) {
	viper.Reset()
	if bootstrap == nil {
		bootstrap = &Config{}
	}
	if _, _, err := relativePathBase(bootstrap); err != nil {
		return nil, err
	}
	configRoot, err := productpaths.ResolveRoot(productpaths.Config, productpaths.UserDefaults(productpaths.Starmap), productRootLayers(bootstrap)...)
	if err != nil {
		return nil, err
	}

	// Try to read config file if it exists
	selectedFile := configFile
	if selectedFile == "" {
		selectedFile = os.Getenv("CONFIG")
	}
	explicitFile := selectedFile != ""
	if explicitFile {
		selected, err := selectedLegacyLeaf(bootstrap, configRoot, selectedFile, "explicit-file", "config")
		if err != nil {
			return nil, err
		}
		selectedFile = selected.Path
	} else {
		selectedFile = filepath.Join(configRoot.Path, "config.yaml")
	}
	if !explicitFile && configRoot.Origin == "platform-default" {
		if home, err := os.UserHomeDir(); err == nil {
			if err := refuseLegacyPath(selectedFile, filepath.Join(home, ".starmap", "config.yaml")); err != nil {
				return nil, err
			}
		}
	}
	viper.SetConfigFile(selectedFile)
	configFileUsed := ""
	configFileBytes, readErr := readConfigurationFile(selectedFile)
	if readErr == nil {
		if err := viper.ReadConfig(bytes.NewReader(configFileBytes)); err != nil {
			return nil, &errors.ConfigError{Component: "configuration file", Message: "cannot read or parse the selected file"}
		}
		configFileUsed = selectedFile
	} else if explicitFile || !stderrors.Is(readErr, os.ErrNotExist) {
		return nil, &errors.ConfigError{Component: "configuration file " + selectedFile, Message: "cannot read the selected private file", Err: readErr}
	}
	pathValues, err := pathValuesFromFile(viper.AllSettings())
	if err != nil {
		return nil, err
	}

	catalogValues, err := catalogValuesFromFile(viper.AllSettings())
	if err != nil {
		return nil, err
	}
	// Capture file presence before enabling environment overrides.
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	_ = viper.BindEnv("output", "OUTPUT")

	credentialSources := make(map[string]map[string]CredentialSourceConfig)
	if err := viper.UnmarshalKey("credential_sources", &credentialSources); err != nil {
		return nil, fmt.Errorf("decode credential source configuration: %w", err)
	}

	// Build config from viper
	config := &Config{
		// Global flags (may be overridden by cobra flags later)
		Verbose: viper.GetBool("verbose"),
		Quiet:   viper.GetBool("quiet"),
		NoColor: viper.GetBool("no-color"),
		Output:  viper.GetString("output"),

		// Config file
		ConfigFile:       configFileUsed,
		configFileDigest: sha256.Sum256(configFileBytes),
		PathValues:       pathValues,
		pathOverrides:    maps.Clone(bootstrap.pathOverrides),
		dotenvPaths:      bootstrap.dotenvPaths,
		configRoot:       configRoot,

		// Starmap configuration
		CatalogPath:                   viper.GetString("catalog_path"),
		EmbeddedBootstrapMaxAge:       viper.GetDuration("embedded_bootstrap_max_age"),
		EmbeddedBootstrapMaxSizeBytes: viper.GetInt64("embedded_bootstrap_max_size_bytes"),
		RemoteServerURL:               viper.GetString("remote_server_url"),
		RemoteServerAPIKey:            viper.GetString("remote_server_api_key"),
		RemoteServerOnly:              viper.GetBool("remote_server_only"),
		CredentialSources:             credentialSources,
		CatalogValues:                 catalogValues.values,
		LegacyCatalogNames:            catalogValues.legacyNames,
		catalogFileRead:               true,

		// An empty LogLevel uses the precedence rules in logger.go.
		// LOG_LEVEL overrides the default level of "info".
		LogLevel:  getEnvOrDefault("LOG_LEVEL", viper.GetString("log_level")),
		LogFormat: getEnvOrDefault("LOG_FORMAT", fileStringOrDefault("log_format", "auto")),
		LogOutput: getEnvOrDefault("LOG_OUTPUT", fileStringOrDefault("log_output", "stderr")),
	}

	return config, nil
}

// maxConfigurationFileBytes bounds each YAML or explicit dotenv input before parsing.
const maxConfigurationFileBytes int64 = 1 << 20

// readConfigurationFile reads one private file after resolving its selected symlink target.
func readConfigurationFile(path string) ([]byte, error) {
	return readPrivateInput(path, "configuration")
}

func readPrivateInput(path, role string) ([]byte, error) {
	if err := policy.Require(role, policy.OwnerOnly); err != nil {
		return nil, err
	}
	if _, err := os.Lstat(path); err != nil {
		return nil, err
	}
	data, err := readConfigurationTarget(path)
	if stderrors.Is(err, os.ErrNotExist) {
		return nil, &errors.ConflictError{Resource: "configuration file", Message: "selected path has no current target"}
	}
	return data, err
}

func readConfigurationTarget(path string) ([]byte, error) {
	if err := privatefiles.ValidateAncestors(path); err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	if err := privatefiles.ValidateAncestors(resolved); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(filepath.Dir(resolved))
	if err != nil {
		return nil, err
	}
	data, err := privatefiles.ReadFile(root, filepath.Base(resolved), maxConfigurationFileBytes)
	return data, stderrors.Join(err, root.Close())
}

// getEnvOrDefault returns the environment variable value or the default if not set.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func fileStringOrDefault(name, fallback string) string {
	if viper.InConfig(name) {
		return viper.GetString(name)
	}
	return fallback
}
