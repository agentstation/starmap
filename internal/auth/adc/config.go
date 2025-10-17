package adc

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadConfig reads a value from gcloud configuration files.
// Supports "project" from [core] section and "region" from [compute] section.
// Returns empty string if config not found or key doesn't exist.
func ReadConfig(key string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	configName := readActiveConfig(home)
	configPath := filepath.Join(home, ".config/gcloud/configurations", "config_"+configName)

	data, err := os.ReadFile(configPath) // #nosec G304 -- Reading well-known gcloud config file
	if err != nil {
		return ""
	}

	return parseINIValue(string(data), key)
}

// readActiveConfig returns the active gcloud configuration name.
// Returns "default" if active_config file doesn't exist.
func readActiveConfig(homeDir string) string {
	activeConfigPath := filepath.Join(homeDir, ".config/gcloud/active_config")
	data, err := os.ReadFile(activeConfigPath) // #nosec G304 -- Reading well-known gcloud config file
	if err != nil {
		return "default"
	}
	return strings.TrimSpace(string(data))
}

// parseINIValue extracts a value from INI-style configuration.
// Supports "project" in [core] section and "region" in [compute] section.
func parseINIValue(content, key string) string {
	lines := strings.Split(content, "\n")
	var currentSection string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Track current section
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			continue
		}

		// Look for key in appropriate section
		if key == "project" && currentSection == "core" {
			if value := extractValue(line, "project"); value != "" {
				return value
			}
		}
		if key == "region" && currentSection == "compute" {
			if value := extractValue(line, "region"); value != "" {
				return value
			}
		}
	}

	return ""
}

// extractValue extracts value from "key = value" line.
// Returns empty string if line doesn't match pattern.
func extractValue(line, key string) string {
	prefix := key + " = "
	if value, found := strings.CutPrefix(line, prefix); found {
		return strings.TrimSpace(value)
	}
	return ""
}
