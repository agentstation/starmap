// Package auth provides authentication checking for AI model providers.
package auth

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/auth/credentials"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// State represents the authentication state of a provider.
type State int

const (
	// StateConfigured means the provider has credentials configured.
	StateConfigured State = iota
	// StateMissing means required credentials are missing.
	StateMissing
	// StateOptional means the provider has optional or no auth requirements.
	StateOptional
	// StateUnsupported means the provider has no client implementation.
	StateUnsupported
)

// Status represents the authentication status of a provider.
type Status struct {
	State   State
	Details string
}

// GCloudStatus represents Google Cloud authentication status.
type GCloudStatus struct {
	Authenticated    bool
	Project          string
	Location         string
	HasVertexProvider bool
}

// Checker checks authentication status for providers.
type Checker struct {
	// Add fields as needed for caching, etc.
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
		return c.checkGoogleVertex()
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
		if provider.Catalog != nil && provider.Catalog.APIKeyRequired != nil && *provider.Catalog.APIKeyRequired {
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

	return &Status{
		State:   StateConfigured,
		Details: fmt.Sprintf("API key configured (%s)", provider.APIKey.Name),
	}
}

// checkGoogleVertex checks Google Vertex AI authentication.
func (c *Checker) checkGoogleVertex() *Status {
	ctx := context.Background()

	// Check for Application Default Credentials
	creds, err := credentials.DetectDefault(&credentials.DetectOptions{
		Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
	})
	if err != nil {
		return &Status{
			State:   StateMissing,
			Details: "Google Cloud authentication required. Run: starmap auth gcloud",
		}
	}

	// Try to get a token to verify auth works
	_, err = creds.Token(ctx)
	if err != nil {
		return &Status{
			State:   StateMissing,
			Details: "Google Cloud authentication expired. Run: starmap auth gcloud",
		}
	}

	// Get project information
	var details string
	if projectID, err := creds.QuotaProjectID(ctx); err == nil && projectID != "" {
		details = fmt.Sprintf("Authenticated (Project: %s)", projectID)
	} else if projectID, err := creds.ProjectID(ctx); err == nil && projectID != "" {
		details = fmt.Sprintf("Authenticated (Project: %s)", projectID)
	} else if projectID := os.Getenv("GOOGLE_CLOUD_PROJECT"); projectID != "" {
		details = fmt.Sprintf("Authenticated (Project: %s from env)", projectID)
	} else {
		details = "Authenticated (No default project set)"
	}

	return &Status{
		State:   StateConfigured,
		Details: details,
	}
}

// CheckGCloud checks Google Cloud authentication status.
func (c *Checker) CheckGCloud() *GCloudStatus {
	ctx := context.Background()
	status := &GCloudStatus{}

	// Check if we have Google Vertex provider
	// This would be set by the caller if needed
	status.HasVertexProvider = true // TODO: Check from catalog

	// Check for Application Default Credentials
	creds, err := credentials.DetectDefault(&credentials.DetectOptions{
		Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
	})
	if err != nil {
		return status
	}

	// Try to get a token to verify auth works
	_, err = creds.Token(ctx)
	if err != nil {
		return status
	}

	status.Authenticated = true

	// Get project information
	if projectID, err := creds.QuotaProjectID(ctx); err == nil && projectID != "" {
		status.Project = projectID
	} else if projectID, err := creds.ProjectID(ctx); err == nil && projectID != "" {
		status.Project = projectID
	} else {
		status.Project = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}

	// Get location from environment
	status.Location = os.Getenv("GOOGLE_VERTEX_LOCATION")
	if status.Location == "" {
		status.Location = "us-central1" // Default
	}

	return status
}