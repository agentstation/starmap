// Package google provides a unified, dynamic client for Google AI APIs (AI Studio and Vertex AI).
// This package provides configuration-driven behavior based on provider YAML configuration.
package google

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"google.golang.org/genai"

	"github.com/agentstation/starmap/internal/auth/adc"
	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/sourcepayload"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// Client acquires and normalizes model metadata from Google AI Studio or Vertex AI.
type Client struct {
	provider *catalogs.Provider

	// Authentication
	credentials *auth.Credentials // Centralized credentials management

	// Vertex AI specific fields (lazy-loaded)
	projectID string
	location  string

	// GenAI client - reused across calls when possible
	genaiClient *genai.Client

	mu sync.RWMutex
}

// NewClient creates a new dynamic Google client that works for both AI Studio and Vertex AI.
func NewClient(provider *catalogs.Provider) *Client {
	return &Client{
		provider: provider,
	}
}

// Configure sets the provider for this client.
func (c *Client) Configure(provider *catalogs.Provider) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.provider = provider

	// Reset cached clients and credentials
	c.genaiClient = nil
	c.credentials = nil
	c.projectID = ""
	c.location = ""
}

// Close releases any resources held by the client.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.genaiClient != nil {
		// GenAI client doesn't have a Close method, but we clear the reference
		c.genaiClient = nil
	}

	// Clear credentials to force re-initialization if needed
	c.credentials = nil

	return nil
}

// IsAPIKeyRequired returns true if the client requires an API key.
func (c *Client) IsAPIKeyRequired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider.IsAPIKeyRequired()
}

// HasAPIKey returns true if the client has an API key.
func (c *Client) HasAPIKey() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider.HasAPIKey()
}

// initCredentials initializes or returns cached credentials for Google Cloud authentication.
func (c *Client) initCredentials(ctx context.Context) (*auth.Credentials, error) {
	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.credentials != nil {
		return c.credentials, nil
	}

	// Detect credentials with aggressive timeout (2 seconds max)
	// DetectDefault doesn't accept context, so we run it in a goroutine
	type result struct {
		creds *auth.Credentials
		err   error
	}

	resultChan := make(chan result, 1)
	go func() {
		creds, err := credentials.DetectDefault(&credentials.DetectOptions{
			Scopes: []string{
				"https://www.googleapis.com/auth/cloud-platform",
				"https://www.googleapis.com/auth/generative-language",
			},
		})
		resultChan <- result{creds: creds, err: err}
	}()

	// Wait for result or timeout (2 seconds - realistic time is under 100ms)
	timeout := time.After(2 * time.Second)
	select {
	case res := <-resultChan:
		if res.err != nil {
			return nil, &errors.ConfigError{
				Component: string(c.provider.ID),
				Message:   "no valid credentials found - configure Application Default Credentials or set GOOGLE_CLOUD_PROJECT",
			}
		}
		c.credentials = res.creds
		return res.creds, nil

	case <-timeout:
		return nil, &errors.ConfigError{
			Component: string(c.provider.ID),
			Message:   "credential detection timed out (2s) - likely not configured or network issue",
		}

	case <-ctx.Done():
		return nil, &errors.ConfigError{
			Component: string(c.provider.ID),
			Message:   "credential detection cancelled",
		}
	}

}

// ListModels retrieves all available models using the appropriate Google API.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil {
		return nil, &errors.ValidationError{
			Field:   "provider",
			Message: "provider not configured",
		}
	}

	// Determine which backend to use based on provider configuration
	useVertex := c.shouldUseVertexBackend()

	if useVertex {
		return c.listModelsVertex(ctx)
	}

	// Check if AI Studio is configured
	if !provider.HasAPIKey() {
		return nil, &errors.ConfigError{
			Component: string(provider.ID),
			Message:   "no valid configuration found - set GOOGLE_API_KEY for AI Studio or GOOGLE_CLOUD_PROJECT for Vertex AI",
		}
	}

	return c.listModelsAIStudio(ctx)
}

// shouldUseVertexBackend determines if we should use Vertex AI backend.
func (c *Client) shouldUseVertexBackend() bool {
	// Check endpoint type first
	if c.provider.Catalog != nil && c.provider.Catalog.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
		return true
	}

	// Check for Vertex-specific configuration
	if c.provider.EnvVar("GOOGLE_VERTEX_PROJECT") != "" ||
		c.provider.EnvVar("GOOGLE_CLOUD_PROJECT") != "" {
		return true
	}

	// If we have an API key, prefer AI Studio
	if c.provider.HasAPIKey() {
		return false
	}

	// Don't default to Vertex - it requires explicit configuration
	// Without project ID or API key, we can't use either backend
	return false
}

// getOrCreateGenAIClient gets or creates a GenAI client for the appropriate backend.
func (c *Client) getOrCreateGenAIClient(ctx context.Context, forVertex bool) (*genai.Client, error) {
	c.mu.RLock()
	if c.genaiClient != nil {
		client := c.genaiClient
		c.mu.RUnlock()
		return client, nil
	}
	projectID := c.projectID
	location := c.location
	c.mu.RUnlock()

	var config *genai.ClientConfig

	if forVertex {
		// Ensure we have project and location
		if projectID == "" {
			projectID = c.getProjectID(ctx)
		}
		if location == "" {
			location = c.getLocation(ctx)
		}

		if projectID == "" {
			return nil, &errors.ConfigError{
				Component: "google-vertex",
				Message:   "project ID not configured - set GOOGLE_CLOUD_PROJECT or configure ADC with project",
			}
		}

		config = &genai.ClientConfig{
			Backend:  genai.BackendVertexAI,
			Project:  projectID,
			Location: location,
		}

		// Check if API key is available for Vertex AI (optional)
		if apiKey, err := c.provider.APIKeyValue(); err == nil && apiKey != "" {
			// Use API key for Vertex AI if available
			config.APIKey = apiKey
		} else {
			// Fall back to Application Default Credentials
			creds, err := c.initCredentials(ctx)
			if err != nil {
				return nil, err
			}
			config.Credentials = creds
		}
	} else {
		// AI Studio configuration with API key
		apiKey, err := c.provider.APIKeyValue()
		if err != nil || apiKey == "" {
			return nil, &errors.AuthenticationError{
				Provider: "google-ai-studio",
				Method:   "api-key",
				Message:  "API key required for Google AI Studio",
				Err:      err,
			}
		}

		config = &genai.ClientConfig{
			Backend: genai.BackendGeminiAPI,
			APIKey:  apiKey,
		}
	}

	client, err := genai.NewClient(ctx, config)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.genaiClient != nil {
		return c.genaiClient, nil
	}
	c.projectID = projectID
	c.location = location
	c.genaiClient = client
	return client, nil
}

// listModelsAIStudio fetches models using Google AI Studio API via GenAI SDK.
func (c *Client) listModelsAIStudio(ctx context.Context) ([]catalogs.Model, error) {
	if models, err := c.listModelsAIStudioREST(ctx); err == nil {
		if len(models) > 0 {
			return models, nil
		}
	} else {
		var quarantineErr *sourcepayload.QuarantineError
		if stderrors.As(err, &quarantineErr) {
			return models, err
		}
		var parseErr *errors.ParseError
		if stderrors.As(err, &parseErr) {
			return nil, err
		}
	}

	// Use GenAI SDK only
	client, err := c.getOrCreateGenAIClient(ctx, false)
	if err != nil {
		return nil, err
	}

	return c.listModelsViaGenAI(ctx, client)
}

func (c *Client) listModelsAIStudioREST(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()
	if provider == nil || provider.Catalog == nil || provider.CatalogEndpointURL() == "" {
		return nil, &errors.ValidationError{
			Field:   "catalog.endpoint.url",
			Message: "Google AI Studio REST endpoint not configured",
		}
	}

	httpClient := transport.New(provider)
	pageToken := ""
	seenPageTokens := make(map[string]struct{})
	models := make([]catalogs.Model, 0)
	report := sourcepayload.RecordReport{}
	for {
		requestURL, err := googleListURL(provider.CatalogEndpointURL(), pageToken)
		if err != nil {
			return nil, err
		}
		resp, err := httpClient.Get(ctx, requestURL, provider)
		if err != nil {
			return nil, err
		}
		var result aiStudioModelsResponse
		if err := transport.DecodeResponse(resp, &result); err != nil {
			return nil, err
		}
		if result.Models == nil {
			return nil, errors.NewParseError("json", "google AI Studio response", "required models array is missing or null", nil)
		}
		report.Rejected += result.RecordReport.Rejected
		report.Issues = append(report.Issues, result.RecordReport.Issues...)
		report.Truncated = report.Truncated || result.RecordReport.Truncated
		pageModels := result.Models
		remaining := constants.MaxCatalogModels - len(models) - report.Rejected
		if remaining < 0 {
			remaining = 0
		}
		if len(pageModels) > remaining {
			report.Rejected += len(pageModels) - remaining
			report.Truncated = true
			pageModels = pageModels[:remaining]
		}
		report.Accepted += len(pageModels)
		for _, rawModel := range pageModels {
			rawModel.UnknownFields = append(rawModel.UnknownFields, result.UnknownFields...)
			models = append(models, *c.convertAIStudioModel(rawModel))
		}
		if report.Truncated {
			break
		}
		if result.NextPageToken == "" {
			break
		}
		if _, exists := seenPageTokens[result.NextPageToken]; exists {
			return nil, errors.NewParseError(
				"json",
				"google AI Studio response",
				"nextPageToken repeated without completing the collection",
				nil,
			)
		}
		seenPageTokens[result.NextPageToken] = struct{}{}
		pageToken = result.NextPageToken
	}
	return models, report.Err("google AI Studio models")
}

func googleListURL(endpoint, pageToken string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", errors.WrapParse("url", endpoint, err)
	}
	query := parsed.Query()
	if query.Get("pageSize") == "" {
		query.Set("pageSize", "100")
	}
	if pageToken != "" {
		query.Set("pageToken", pageToken)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// checkVertexPrerequisites performs pre-flight checks for Vertex AI.
// This uses the same logic as `starmap providers auth test` to detect ADC configuration
// locally without making network calls.
func (c *Client) checkVertexPrerequisites() error {
	// Check ADC status using the same logic as `starmap providers auth test`
	details := adc.BuildDetails()

	switch details.State {
	case adc.StateMissing:
		return &errors.ConfigError{
			Component: "google-vertex",
			Message:   "Application Default Credentials not configured - run 'gcloud auth application-default login'",
		}
	case adc.StateInvalid:
		return &errors.ConfigError{
			Component: "google-vertex",
			Message:   "Application Default Credentials invalid - check 'gcloud auth application-default login'",
		}
	case adc.StateConfigured:
		// ADC is configured, now check if project is set
		if os.Getenv("GOOGLE_VERTEX_PROJECT") == "" && os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
			return &errors.ConfigError{
				Component: "google-vertex",
				Message:   "No project configured - set GOOGLE_VERTEX_PROJECT or GOOGLE_CLOUD_PROJECT environment variable",
			}
		}
		// All checks passed
		return nil
	default:
		return &errors.ConfigError{
			Component: "google-vertex",
			Message:   "Unknown ADC state",
		}
	}
}

// listModelsVertex fetches models using Vertex AI API.
func (c *Client) listModelsVertex(ctx context.Context) ([]catalogs.Model, error) {
	// Pre-flight check: Verify ADC is available before attempting network calls
	// This is the same check used by `starmap providers auth test`
	if err := c.checkVertexPrerequisites(); err != nil {
		return nil, err
	}

	// Bound the complete paginated operation while respecting any shorter caller
	// deadline. Vertex model listings can span multiple requests, so a per-call
	// latency assumption is not an appropriate operation deadline.
	vertexCtx, cancel := context.WithTimeout(ctx, constants.ProviderFetchTimeout)
	defer cancel()

	// Use GenAI SDK only
	client, err := c.getOrCreateGenAIClient(vertexCtx, true)
	if err != nil {
		return nil, err
	}

	// The SDK accepts the bounded context, so call it directly instead of
	// creating an unjoinable timeout goroutine around context-aware work.
	models, err := c.listModelsViaGenAI(vertexCtx, client)
	if err != nil {
		if vertexCtx.Err() == nil {
			return nil, err
		}
		message := fmt.Sprintf("request timed out after %s", constants.ProviderFetchTimeout)
		if vertexCtx.Err() == context.Canceled {
			message = "request canceled"
		}
		return nil, &errors.APIError{
			Provider:   "google-vertex",
			Endpoint:   "models",
			StatusCode: 0,
			Message:    message,
			Err:        vertexCtx.Err(),
		}
	}

	// Add Model Garden models from pre-defined list.
	modelGardenModels := c.getModelGardenModels()
	return c.mergeModels(models, modelGardenModels), nil
}

// listModelsViaGenAI uses the GenAI SDK to list models (works for both backends).
func (c *Client) listModelsViaGenAI(ctx context.Context, client *genai.Client) ([]catalogs.Model, error) {
	var models []catalogs.Model
	providerID := "google"
	c.mu.RLock()
	if c.provider != nil {
		providerID = string(c.provider.ID)
	}
	c.mu.RUnlock()
	logger := logging.FromContext(logging.WithProvider(ctx, providerID))

	// Get all base models with pagination
	baseModels, err := c.getAllModelsGenAI(ctx, client, true)
	if err != nil {
		logger.Warn().Err(err).Str("model_scope", "base").Msg("Could not list Google models")
	} else {
		for _, model := range baseModels {
			models = append(models, *model)
		}
	}

	// Get all tuned/custom models with pagination
	tunedModels, err := c.getAllModelsGenAI(ctx, client, false)
	if err != nil {
		logger.Warn().Err(err).Str("model_scope", "tuned").Msg("Could not list Google models")
	} else {
		for _, model := range tunedModels {
			models = append(models, *model)
		}
	}

	if len(models) == 0 && err != nil {
		return nil, err // Return error if we got no models at all
	}

	return models, nil
}

// extractModelID extracts the model ID from the full name.

func (c *Client) getProjectID(ctx context.Context) string {
	// 1. Check environment variables first (highest priority)
	if projectID := c.provider.EnvVar("GOOGLE_CLOUD_PROJECT"); projectID != "" {
		return projectID
	}
	if projectID := c.provider.EnvVar("GOOGLE_VERTEX_PROJECT"); projectID != "" {
		return projectID
	}

	// 2. Get from credentials (no gcloud fallback)
	creds, err := c.initCredentials(ctx)
	if err == nil {
		// Try quota project ID first (for billing)
		if projectID, err := creds.QuotaProjectID(ctx); err == nil && projectID != "" {
			return projectID
		}

		// Fall back to regular project ID
		if projectID, err := creds.ProjectID(ctx); err == nil && projectID != "" {
			return projectID
		}
	}

	return ""
}

// getLocation gets the location from environment variables with sensible defaults.
// Returns empty string if context is cancelled.
func (c *Client) getLocation(ctx context.Context) string {
	// Check if context is already cancelled
	if ctx.Err() != nil {
		return ""
	}

	// Check environment variables
	locations := []string{
		c.provider.EnvVar("GOOGLE_CLOUD_LOCATION"),
		c.provider.EnvVar("GOOGLE_CLOUD_REGION"),
		c.provider.EnvVar("GOOGLE_VERTEX_LOCATION"),
	}

	for _, loc := range locations {
		if loc != "" {
			return loc
		}
	}

	// Default to us-central1 (most commonly available region)
	return "us-central1"
}

// ValidateCredentials validates that the client can authenticate properly.
func (c *Client) ValidateCredentials(ctx context.Context) error {
	if c.shouldUseVertexBackend() {
		// For Vertex, check that we can get credentials and project
		creds, err := c.initCredentials(ctx)
		if err != nil {
			return err
		}

		// Try to get a token to validate credentials work
		_, err = creds.Token(ctx)
		if err != nil {
			return &errors.AuthenticationError{
				Provider: string(c.provider.ID),
				Method:   "oauth2",
				Message:  "credentials validation failed",
				Err:      err,
			}
		}

		// Verify project ID is available
		projectID := c.getProjectID(ctx)
		if projectID == "" {
			return &errors.ConfigError{
				Component: "google-vertex",
				Message:   "no project ID available - set GOOGLE_CLOUD_PROJECT or configure ADC with project",
			}
		}
	} else {
		// For AI Studio, just check API key
		if !c.HasAPIKey() {
			return &errors.AuthenticationError{
				Provider: "google-ai-studio",
				Method:   "api-key",
				Message:  "API key not configured",
			}
		}
	}

	return nil
}

// normalizePublisherToAuthorID maps Google Vertex publisher names to AuthorID.
