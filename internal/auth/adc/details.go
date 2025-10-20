package adc

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// State represents the authentication state (mirrors auth.State to avoid import cycle).
type State int

const (
	// StateConfigured means credentials are configured.
	StateConfigured State = iota
	// StateMissing means required credentials are missing.
	StateMissing
	// StateInvalid means credentials are found but malformed or invalid.
	StateInvalid
)

// Details contains Google Cloud authentication details.
//
// This type is used in auth.Status.GoogleCloud field (stored as interface{}
// to avoid import cycles).
type Details struct {
	State          State
	Type           string    // "User Credentials" | "Service Account"
	Account        string    // Email address or client ID
	Project        string    // Project ID
	ProjectSource  string    // "ADC (quota_project_id)" | "env (GOOGLE_VERTEX_PROJECT)" | "gcloud config" | "not set"
	Location       string    // Region
	LocationSource string    // "env (GOOGLE_VERTEX_LOCATION)" | "gcloud config" | "default"
	UniverseDomain string    // Usually "googleapis.com"
	ADCPath        string    // Path to ADC file
	LastAuth       time.Time // File modification time
	ErrorMessage   string    // Error message (only for invalid/missing states)
}

// BuildDetails creates comprehensive Google Cloud authentication status.
// Performs local inspection only - no network calls are made.
//
// This function:
//  1. Finds the ADC file
//  2. Parses and validates it
//  3. Extracts project/location from multiple sources
//  4. Returns complete authentication details
func BuildDetails() *Details {
	// Find ADC file
	adcPath := FindFile()
	if adcPath == "" {
		return &Details{
			State:        StateMissing,
			ErrorMessage: "No ADC found. Run: starmap providers auth gcloud",
		}
	}

	// Parse ADC file
	file, err := ParseFile(adcPath)
	if err != nil {
		return &Details{
			State:        StateInvalid,
			ADCPath:      adcPath,
			ErrorMessage: fmt.Sprintf("ADC file invalid: %v", err),
		}
	}

	// Extract details
	return buildConfiguredDetails(file, adcPath)
}

// buildConfiguredDetails constructs details for a valid ADC file.
func buildConfiguredDetails(file *File, adcPath string) *Details {
	details := &Details{
		State:          StateConfigured,
		Type:           credentialType(file.Type),
		Account:        accountIdentifier(file),
		UniverseDomain: universeDomain(file.UniverseDomain),
		ADCPath:        adcPath,
		LastAuth:       fileModTime(adcPath),
	}

	// Resolve project with fallback chain
	details.Project, details.ProjectSource = resolveProject(file)

	// Resolve location with fallback chain
	details.Location, details.LocationSource = resolveLocation()

	return details
}

// credentialType converts ADC type to human-readable string.
func credentialType(adcType string) string {
	if adcType == TypeServiceAccount {
		return "Service Account"
	}
	return "User Credentials"
}

// accountIdentifier extracts account identifier from ADC file.
// Prefers email address, falls back to client ID.
func accountIdentifier(file *File) string {
	if file.Account != "" {
		return file.Account
	}
	if file.ClientID != "" {
		return "(client ID: " + file.ClientID + ")"
	}
	return ""
}

// universeDomain returns the universe domain or default.
func universeDomain(domain string) string {
	if domain == "" {
		return "googleapis.com"
	}
	return domain
}

// fileModTime returns file modification time or zero value.
func fileModTime(path string) time.Time {
	if stat, err := os.Stat(path); err == nil {
		return stat.ModTime()
	}
	return time.Time{}
}

// resolveProject determines project ID using fallback chain.
//
// Priority order:
//  1. ADC quota_project_id
//  2. ADC project_id
//  3. GOOGLE_VERTEX_PROJECT environment variable
//  4. gcloud config (core.project)
//
// Returns empty string and "not set" if no project found.
func resolveProject(file *File) (project, source string) {
	if file.QuotaProjectID != "" {
		return file.QuotaProjectID, "ADC (quota_project_id)"
	}
	if file.ProjectID != "" {
		return file.ProjectID, "ADC (project_id)"
	}
	if envProject := os.Getenv("GOOGLE_VERTEX_PROJECT"); envProject != "" {
		return envProject, "env (GOOGLE_VERTEX_PROJECT)"
	}
	if configProject := ReadConfig("project"); configProject != "" {
		return configProject, "gcloud config"
	}
	return "", "not set"
}

// resolveLocation determines location/region using fallback chain.
//
// Priority order:
//  1. GOOGLE_VERTEX_LOCATION environment variable
//  2. gcloud config (compute.region)
//  3. Default: us-central1
//
// Always returns a location (falls back to us-central1).
func resolveLocation() (location, source string) {
	if envLocation := os.Getenv("GOOGLE_VERTEX_LOCATION"); envLocation != "" {
		return envLocation, "env (GOOGLE_VERTEX_LOCATION)"
	}
	if configRegion := ReadConfig("region"); configRegion != "" {
		return configRegion, "gcloud config"
	}
	return "us-central1", "default"
}

// FormatBrief creates a one-line summary of Google Cloud auth status.
// This is used in the provider table view.
//
// Format: "{Type}, {Project status}, Location: {location}"
// Example: "User Credentials, Project: my-project, Location: us-central1".
func FormatBrief(details *Details) string {
	var parts []string

	// Credential type
	parts = append(parts, details.Type)

	// Project status
	if details.Project != "" {
		parts = append(parts, fmt.Sprintf("Project: %s", details.Project))
	} else {
		parts = append(parts, "No project set")
	}

	// Location
	parts = append(parts, fmt.Sprintf("Location: %s", details.Location))

	return strings.Join(parts, ", ")
}
