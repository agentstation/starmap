// Package auth provides authentication checking for AI model providers.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// State represents the authentication state of a provider.
type State int

const (
	// StateConfigured means the provider has credentials configured.
	StateConfigured State = iota
	// StateMissing means required credentials are missing.
	StateMissing
	// StateInvalid means credentials are found but malformed or invalid.
	StateInvalid
	// StateOptional means the provider has optional or no auth requirements.
	StateOptional
	// StateUnsupported means the provider has no client implementation.
	StateUnsupported
)

// Status represents the authentication status of a provider.
type Status struct {
	State   State
	Details string
	Extra   interface{} // Provider-specific detailed data
}

// GoogleVertexDetails contains detailed Google Vertex AI authentication information.
type GoogleVertexDetails struct {
	State          State
	Type           string    // "user" | "service_account"
	Account        string    // Email address
	Project        string    // Project ID
	ProjectSource  string    // "ADC" | "env" | "gcloud config" | "not set"
	Location       string    // Region
	LocationSource string    // "env" | "gcloud config" | "default"
	UniverseDomain string    // Usually "googleapis.com"
	ADCPath        string    // File path
	LastAuth       time.Time // File modification time
	ErrorMessage   string    // For invalid/missing states
}

// GCloudStatus represents Google Cloud authentication status.
type GCloudStatus struct {
	Authenticated     bool
	Project           string
	Location          string
	HasVertexProvider bool
}

// Checker checks authentication status for providers.
type Checker struct {
	// Add fields as needed for caching, etc.
}

const (
	// ADC credential type constants.
	adcTypeAuthorizedUser = "authorized_user"
	adcTypeServiceAccount = "service_account"
)

// adcFile represents the structure of an Application Default Credentials JSON file.
type adcFile struct {
	Type           string `json:"type"`
	QuotaProjectID string `json:"quota_project_id"`
	ProjectID      string `json:"project_id"`
	Account        string `json:"account"`
	ClientID       string `json:"client_id"`
	UniverseDomain string `json:"universe_domain"`
}

// NewChecker creates a new authentication checker.
func NewChecker() *Checker {
	return &Checker{}
}

// CheckProvider checks the authentication status of a provider.
func (c *Checker) CheckProvider(provider *catalogs.Provider, supportedMap map[string]bool) *Status {
	// Check if provider is supported
	if !supportedMap[string(provider.ID)] {
		return &Status{
			State:   StateUnsupported,
			Details: "No client implementation available",
		}
	}

	// Special handling for Google Vertex AI
	if provider.ID == "google-vertex" {
		return c.checkGoogleVertexLocal()
	}

	// Check if provider requires API key
	if provider.APIKey == nil {
		return &Status{
			State:   StateOptional,
			Details: "No API key required",
		}
	}

	// Check if API key is configured
	envValue := os.Getenv(provider.APIKey.Name)
	if envValue == "" {
		// Check if it's required
		if provider.Catalog != nil && provider.Catalog.Endpoint.AuthRequired {
			return &Status{
				State:   StateMissing,
				Details: fmt.Sprintf("Set %s environment variable", provider.APIKey.Name),
			}
		}
		return &Status{
			State:   StateOptional,
			Details: fmt.Sprintf("Optional: %s not set", provider.APIKey.Name),
		}
	}

	// Validate pattern if specified
	if provider.APIKey.Pattern != "" && provider.APIKey.Pattern != ".*" {
		matched, err := regexp.MatchString(provider.APIKey.Pattern, envValue)
		if err != nil || !matched {
			return &Status{
				State:   StateInvalid,
				Details: "API key does not match required pattern",
			}
		}
	}

	return &Status{
		State:   StateConfigured,
		Details: fmt.Sprintf("API key configured (%s)", provider.APIKey.Name),
	}
}

// findADCFile locates the Application Default Credentials file.
// Returns empty string if not found.
func (c *Checker) findADCFile() string {
	// 1. Check GOOGLE_APPLICATION_CREDENTIALS environment variable
	if path := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// 2. Check default ADC location
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	defaultPath := filepath.Join(home, ".config/gcloud/application_default_credentials.json")
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}

	return ""
}

// parseADCFile parses and validates an ADC JSON file.
func (c *Checker) parseADCFile(path string) (*adcFile, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- Reading well-known ADC credential file
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}

	var adc adcFile
	if err := json.Unmarshal(data, &adc); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Basic validation
	if adc.Type == "" {
		return nil, fmt.Errorf("missing 'type' field")
	}
	if adc.Type != adcTypeAuthorizedUser && adc.Type != adcTypeServiceAccount {
		return nil, fmt.Errorf("unknown type: %s", adc.Type)
	}

	return &adc, nil
}

// readGCloudConfig reads a configuration value from gcloud config files.
// Supports reading 'project' from [core] section and 'region' from [compute] section.
func (c *Checker) readGCloudConfig(key string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Read active config name
	activeConfigPath := filepath.Join(home, ".config/gcloud/active_config")
	activeConfig, err := os.ReadFile(activeConfigPath) // #nosec G304 -- Reading well-known gcloud config file
	if err != nil {
		activeConfig = []byte("default")
	}
	configName := strings.TrimSpace(string(activeConfig))

	// Read config file
	configPath := filepath.Join(home, ".config/gcloud/configurations", "config_"+configName)
	data, err := os.ReadFile(configPath) // #nosec G304 -- Reading well-known gcloud config file
	if err != nil {
		return ""
	}

	// Simple INI parsing
	lines := strings.Split(string(data), "\n")
	inCore := false
	inCompute := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "[core]" {
			inCore = true
			inCompute = false
			continue
		}
		if line == "[compute]" {
			inCore = false
			inCompute = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inCore = false
			inCompute = false
			continue
		}

		// Look for key
		if key == "project" && inCore {
			if strings.HasPrefix(line, "project = ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "project = "))
			}
		}
		if key == "region" && inCompute {
			if strings.HasPrefix(line, "region = ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "region = "))
			}
		}
	}

	return ""
}

// buildGoogleVertexDetails extracts detailed information from ADC and config files.
func (c *Checker) buildGoogleVertexDetails(adc *adcFile) string {
	var parts []string

	// Credential type
	if adc.Type == adcTypeServiceAccount {
		parts = append(parts, "Service Account")
	} else {
		parts = append(parts, "User Credentials")
	}

	// Project (try multiple sources)
	project := adc.QuotaProjectID
	if project == "" {
		project = adc.ProjectID
	}
	if project == "" {
		project = os.Getenv("GOOGLE_VERTEX_PROJECT")
	}
	if project == "" {
		project = c.readGCloudConfig("project")
	}

	if project != "" {
		parts = append(parts, fmt.Sprintf("Project: %s", project))
	} else {
		parts = append(parts, "No project set")
	}

	// Location
	location := os.Getenv("GOOGLE_VERTEX_LOCATION")
	if location == "" {
		location = c.readGCloudConfig("region")
	}
	if location == "" {
		location = "us-central1"
	}
	parts = append(parts, fmt.Sprintf("Location: %s", location))

	return strings.Join(parts, ", ")
}

// checkGoogleVertexLocal checks Google Vertex AI authentication using local files only.
// No network calls are made.
func (c *Checker) checkGoogleVertexLocal() *Status {
	// Find ADC file
	adcPath := c.findADCFile()
	if adcPath == "" {
		details := &GoogleVertexDetails{
			State:        StateMissing,
			ErrorMessage: "No ADC found. Run: starmap auth gcloud",
		}
		return &Status{
			State:   StateMissing,
			Details: "No ADC found. Run: starmap auth gcloud",
			Extra:   details,
		}
	}

	// Parse ADC JSON
	adc, err := c.parseADCFile(adcPath)
	if err != nil {
		details := &GoogleVertexDetails{
			State:        StateInvalid,
			ADCPath:      adcPath,
			ErrorMessage: fmt.Sprintf("ADC file invalid: %v", err),
		}
		return &Status{
			State:   StateInvalid,
			Details: fmt.Sprintf("ADC file invalid: %v", err),
			Extra:   details,
		}
	}

	// Get file timestamp for Last Authenticated
	var lastAuth time.Time
	if stat, err := os.Stat(adcPath); err == nil {
		lastAuth = stat.ModTime()
	}

	// Determine credential type
	credType := "User Credentials"
	if adc.Type == adcTypeServiceAccount {
		credType = "Service Account"
	}

	// Get account email
	account := adc.Account
	if account == "" && adc.ClientID != "" {
		account = "(client ID: " + adc.ClientID + ")"
	}

	// Determine project and source
	project := ""
	projectSource := "not set"
	if adc.QuotaProjectID != "" {
		project = adc.QuotaProjectID
		projectSource = "ADC (quota_project_id)"
	} else if adc.ProjectID != "" {
		project = adc.ProjectID
		projectSource = "ADC (project_id)"
	} else if envProject := os.Getenv("GOOGLE_VERTEX_PROJECT"); envProject != "" {
		project = envProject
		projectSource = "env (GOOGLE_VERTEX_PROJECT)"
	} else if configProject := c.readGCloudConfig("project"); configProject != "" {
		project = configProject
		projectSource = "gcloud config"
	}

	// Determine location and source
	location := ""
	locationSource := ""
	if envLocation := os.Getenv("GOOGLE_VERTEX_LOCATION"); envLocation != "" {
		location = envLocation
		locationSource = "env (GOOGLE_VERTEX_LOCATION)"
	} else if configRegion := c.readGCloudConfig("region"); configRegion != "" {
		location = configRegion
		locationSource = "gcloud config"
	} else {
		location = "us-central1"
		locationSource = "default"
	}

	// Universe domain
	universeDomain := adc.UniverseDomain
	if universeDomain == "" {
		universeDomain = "googleapis.com"
	}

	// Build detailed struct
	details := &GoogleVertexDetails{
		State:          StateConfigured,
		Type:           credType,
		Account:        account,
		Project:        project,
		ProjectSource:  projectSource,
		Location:       location,
		LocationSource: locationSource,
		UniverseDomain: universeDomain,
		ADCPath:        adcPath,
		LastAuth:       lastAuth,
	}

	// Build brief summary for table view
	briefDetails := c.buildGoogleVertexDetails(adc)

	return &Status{
		State:   StateConfigured,
		Details: briefDetails,
		Extra:   details,
	}
}
