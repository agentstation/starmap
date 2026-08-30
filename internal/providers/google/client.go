// Package google provides a unified, dynamic client for Google AI APIs (AI Studio and Vertex AI).
// This package provides configuration-driven behavior based on provider YAML configuration.
package google

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/url"
	"sync"

	"cloud.google.com/go/auth"
	"google.golang.org/genai"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
	sourcepayload "github.com/agentstation/starmap/pkg/sources/payload"
)

// Client gets and normalizes model metadata from Google AI Studio or Vertex AI.
type Client struct {
	provider *catalogs.Provider

	mu sync.RWMutex
}

// NewClient creates a new dynamic Google client that works for both AI Studio and Vertex AI.
func NewClient(provider *catalogs.Provider) *Client {
	return &Client{
		provider: provider,
	}
}

// ValidateCatalogEndpoint validates the typed Google catalog-acquisition contract.
func ValidateCatalogEndpoint(provider *catalogs.Provider) error {
	if provider == nil || provider.Catalog == nil {
		return nil
	}
	endpoint := provider.Catalog.Endpoint
	if len(endpoint.FieldMappings) != 0 {
		return &errors.ValidationError{
			Field: "field_mappings", Value: endpoint.FieldMappings,
			Message: "Google catalog acquisition does not expose configurable field mappings",
		}
	}
	if len(endpoint.CapabilityMappings) != 0 {
		return &errors.ValidationError{
			Field: "capability_mappings", Value: endpoint.CapabilityMappings,
			Message: "Google catalog acquisition does not expose configurable capability mappings",
		}
	}
	if endpoint.AuthorMapping == nil {
		return nil
	}
	if err := endpoint.AuthorMapping.Validate(); err != nil {
		return err
	}
	if endpoint.Type != catalogs.EndpointTypeGoogleCloud || endpoint.AuthorMapping.Field != "publisher" {
		return &errors.ValidationError{
			Field: "author_mapping.field", Value: endpoint.AuthorMapping.Field,
			Message: "only Google Cloud catalog acquisition supports the publisher author field",
		}
	}
	return nil
}

// Configure sets the provider for this client.
func (c *Client) Configure(provider *catalogs.Provider) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.provider = provider

}

// Close releases any resources held by the client.
func (c *Client) Close() error {
	return nil
}

// ListModels retrieves all available models using the appropriate Google API.
func (c *Client) ListModels(
	ctx context.Context,
	material sources.ProviderCredentialMaterial,
) ([]catalogs.Model, error) {
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
	useVertex := shouldUseVertexBackend(provider)

	if useVertex {
		return c.listModelsVertex(ctx, material)
	}

	if apiKeyFromMaterial(material) == "" {
		return nil, &errors.ConfigError{
			Component: string(provider.ID),
			Message:   "catalog API key is not configured",
		}
	}

	return c.listModelsAIStudio(ctx, material)
}

// shouldUseVertexBackend determines if the provider uses the Vertex AI backend.
func shouldUseVertexBackend(provider *catalogs.Provider) bool {
	// Check endpoint type first
	if provider.Catalog != nil && provider.Catalog.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
		return true
	}

	return false
}

// newGenAIClient creates a request-scoped GenAI client from resolved
// credential material. The provider client retains no credential value.
func (c *Client) newGenAIClient(
	ctx context.Context,
	forVertex bool,
	material sources.ProviderCredentialMaterial,
) (*genai.Client, error) {
	var config *genai.ClientConfig

	if forVertex {
		projectID := c.getProjectID(material)
		location := c.getLocation(material)

		if projectID == "" {
			return nil, &errors.ConfigError{
				Component: c.providerID(),
				Message:   "resolved project ID is required",
			}
		}

		config = &genai.ClientConfig{
			Backend:  genai.BackendVertexAI,
			Project:  projectID,
			Location: location,
		}

		creds, err := credentialsFromMaterial(material)
		if err != nil {
			return nil, err
		}
		config.Credentials = creds
	} else {
		apiKey := apiKeyFromMaterial(material)
		if apiKey == "" {
			return nil, &errors.AuthenticationError{
				Provider: c.providerID(),
				Method:   "api-key",
				Message:  "catalog API key is required",
			}
		}

		config = &genai.ClientConfig{
			Backend: genai.BackendGeminiAPI,
			APIKey:  apiKey,
		}
	}

	return genai.NewClient(ctx, config)
}

// listModelsAIStudio fetches models using Google AI Studio API via GenAI SDK.
func (c *Client) listModelsAIStudio(
	ctx context.Context,
	material sources.ProviderCredentialMaterial,
) ([]catalogs.Model, error) {
	if models, err := c.listModelsAIStudioREST(ctx, material); err == nil {
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
	client, err := c.newGenAIClient(ctx, false, material)
	if err != nil {
		return nil, err
	}

	return c.listModelsViaGenAI(ctx, client)
}

func (c *Client) listModelsAIStudioREST(
	ctx context.Context,
	material sources.ProviderCredentialMaterial,
) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()
	if provider == nil || provider.Catalog == nil || provider.CatalogEndpointURL() == "" {
		return nil, &errors.ValidationError{
			Field:   "catalog.endpoint.url",
			Message: "Google AI Studio REST endpoint not configured",
		}
	}

	endpoint, err := provider.BindCatalogEndpoint(material.EndpointBindings())
	if err != nil {
		return nil, err
	}
	httpClient := transport.New()
	pageToken := ""
	seenPageTokens := make(map[string]struct{})
	models := make([]catalogs.Model, 0)
	report := sourcepayload.RecordReport{}
	for {
		requestURL, err := googleListURL(endpoint, pageToken)
		if err != nil {
			return nil, err
		}
		resp, err := httpClient.Get(ctx, requestURL, provider, material)
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

// listModelsVertex fetches models using Vertex AI API.
func (c *Client) listModelsVertex(
	ctx context.Context,
	material sources.ProviderCredentialMaterial,
) ([]catalogs.Model, error) {
	// Bound the complete paginated operation while respecting any shorter caller
	// deadline. Vertex model listings can span multiple requests, so a per-call
	// latency assumption is not an appropriate operation deadline.
	vertexCtx, cancel := context.WithTimeout(ctx, constants.ProviderFetchTimeout)
	defer cancel()

	// Use GenAI SDK only
	client, err := c.newGenAIClient(vertexCtx, true, material)
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
			Provider:   c.providerID(),
			Endpoint:   "models",
			StatusCode: 0,
			Message:    message,
			Err:        vertexCtx.Err(),
		}
	}

	return models, nil
}

// listModelsViaGenAI uses the GenAI SDK to list models (works for both backends).
func (c *Client) listModelsViaGenAI(ctx context.Context, client *genai.Client) ([]catalogs.Model, error) {
	var models []catalogs.Model
	providerID := c.providerID()
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

func (c *Client) getProjectID(
	material sources.ProviderCredentialMaterial,
) string {
	if projectID := material.EndpointBindings()["project"]; projectID != "" {
		return projectID
	}

	return ""
}

// getLocation returns the resolved catalog endpoint binding.
func (c *Client) getLocation(
	material sources.ProviderCredentialMaterial,
) string {
	if location := material.EndpointBindings()["location"]; location != "" {
		return location
	}
	return ""
}

// ValidateCredentials validates that the client can authenticate properly.
func (c *Client) ValidateCredentials(
	_ context.Context,
	material sources.ProviderCredentialMaterial,
) error {
	provider := c.providerSnapshot()
	if provider == nil {
		return &errors.ValidationError{Field: "provider", Message: "provider not configured"}
	}
	if shouldUseVertexBackend(provider) {
		if _, err := credentialsFromMaterial(material); err != nil {
			return err
		}

		// Verify project ID is available
		projectID := c.getProjectID(material)
		if projectID == "" {
			return &errors.ConfigError{
				Component: string(provider.ID),
				Message:   "resolved project ID is required",
			}
		}
	} else {
		// For AI Studio, just check API key
		if apiKeyFromMaterial(material) == "" {
			return &errors.AuthenticationError{
				Provider: string(provider.ID),
				Method:   "api-key",
				Message:  "API key not configured",
			}
		}
	}

	return nil
}

func (c *Client) providerSnapshot() *catalogs.Provider {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider
}

func (c *Client) providerID() string {
	provider := c.providerSnapshot()
	if provider == nil {
		return ""
	}
	return string(provider.ID)
}

type resolvedTokenProvider struct {
	token auth.Token
}

func (p resolvedTokenProvider) Token(ctx context.Context) (*auth.Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	token := p.token
	return &token, nil
}

func credentialsFromMaterial(
	material sources.ProviderCredentialMaterial,
) (*auth.Credentials, error) {
	profile := material.Profile()
	for _, placement := range profile.Placements {
		if placement.Kind != catalogs.ProviderCredentialPlacementHeader ||
			placement.Scheme != catalogs.ProviderCredentialSchemeBearer {
			continue
		}
		value, found := material.Value(placement.Field)
		if !found || value == "" {
			break
		}
		token := auth.Token{Value: value, Type: "Bearer"}
		if expiresAt, exists := material.ExpiresAt(); exists {
			token.Expiry = expiresAt
		}
		return auth.NewCredentials(&auth.CredentialsOptions{
			TokenProvider: resolvedTokenProvider{token: token},
		}), nil
	}
	return nil, &errors.AuthenticationError{
		Method: "google-default", Message: "resolved access token is required",
	}
}

func apiKeyFromMaterial(material sources.ProviderCredentialMaterial) string {
	for _, placement := range material.Profile().Placements {
		if value, exists := material.Value(placement.Field); exists && value != "" {
			return value
		}
	}
	return ""
}
