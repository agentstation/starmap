// Package adc handles Google Application Default Credentials.
package adc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// TypeAuthorizedUser represents user credentials from gcloud auth.
	TypeAuthorizedUser = "authorized_user"
	// TypeServiceAccount represents service account credentials.
	TypeServiceAccount = "service_account"
)

// File represents an Application Default Credentials JSON file.
type File struct {
	Type           string `json:"type"`
	QuotaProjectID string `json:"quota_project_id"`
	ProjectID      string `json:"project_id"`
	Account        string `json:"account"`
	ClientID       string `json:"client_id"`
	UniverseDomain string `json:"universe_domain"`
}

// FindFile locates the ADC file using Google's standard search order.
// Returns empty string if not found.
//
// Search order:
//  1. GOOGLE_APPLICATION_CREDENTIALS environment variable
//  2. Default location: ~/.config/gcloud/application_default_credentials.json
func FindFile() string {
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

// ParseFile reads and validates an ADC JSON file.
func ParseFile(path string) (*File, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- Reading well-known ADC credential file
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}

	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate required fields
	if file.Type == "" {
		return nil, fmt.Errorf("missing 'type' field")
	}
	if file.Type != TypeAuthorizedUser && file.Type != TypeServiceAccount {
		return nil, fmt.Errorf("unknown type: %s", file.Type)
	}

	return &file, nil
}
