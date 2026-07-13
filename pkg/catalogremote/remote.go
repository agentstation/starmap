// Package catalogremote defines the versioned online Starmap-to-Starmap
// generation protocol. Artifact distribution remains in catalogdistribution.
package catalogremote

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// CatalogPath is appended to a versioned API base URL.
	CatalogPath = "/catalog"
	// ManifestPath returns the current strict generation manifest.
	ManifestPath = CatalogPath + "/manifest"
	// GenerationsPath prefixes immutable generation snapshot routes.
	GenerationsPath = CatalogPath + "/generations"
	// ManifestMediaType identifies strict generation-manifest JSON.
	ManifestMediaType = "application/vnd.agentstation.starmap.catalog-manifest+json"
	maxBodyBytes      = 64 << 20
)

// SnapshotPath returns the immutable canonical payload route for generationID.
func SnapshotPath(generationID string) string {
	return GenerationsPath + "/" + url.PathEscape(generationID) + "/snapshot"
}

// Client fetches one exact current generation from a versioned Starmap API.
type Client struct {
	baseURL       *url.URL
	httpClient    *http.Client
	schemaVersion uint64
}

// NewClient creates a remote generation client. baseURL is the versioned API
// root, for example https://starmap.example.com/api/v1.
func NewClient(baseURL string, httpClient *http.Client, schemaVersion uint64) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, &errors.ValidationError{Field: "catalog_remote.base_url", Value: baseURL, Message: "must be an absolute HTTP(S) versioned API URL"}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, &errors.ValidationError{Field: "catalog_remote.base_url", Value: baseURL, Message: "must use HTTP or HTTPS"}
	}
	if schemaVersion == 0 {
		return nil, &errors.ValidationError{Field: "catalog_remote.schema_version", Value: schemaVersion, Message: "must be positive"}
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: constants.DefaultHTTPTimeout}
	}
	client := *httpClient
	previousRedirectPolicy := client.CheckRedirect
	client.CheckRedirect = sameOriginRedirectPolicy(parsed, previousRedirectPolicy, "catalog_remote.redirect")
	return &Client{baseURL: parsed, httpClient: &client, schemaVersion: schemaVersion}, nil
}

func sameOriginRedirectPolicy(origin *url.URL, previous func(*http.Request, []*http.Request) error, field string) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if !strings.EqualFold(request.URL.Scheme, origin.Scheme) || !strings.EqualFold(request.URL.Host, origin.Host) {
			return &errors.ValidationError{Field: field, Value: request.URL.String(), Message: "must remain on the configured origin"}
		}
		if previous != nil {
			return previous(request, via)
		}
		if len(via) >= 10 {
			return &errors.ValidationError{Field: field, Value: len(via), Message: "exceeds maximum redirect count"}
		}
		return nil
	}
}

// FetchCurrent fetches the current manifest followed by its immutable,
// generation-addressed snapshot and validates their binding and exact schema.
func (c *Client) FetchCurrent(ctx context.Context) (catalogstore.Generation, error) {
	manifestData, err := c.fetch(ctx, ManifestPath, ManifestMediaType)
	if err != nil {
		return catalogstore.Generation{}, err
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(manifestData)
	if err != nil {
		return catalogstore.Generation{}, errors.WrapResource("parse", "remote catalog manifest", "current", err)
	}
	if manifest.SchemaVersion != catalogs.CurrentCatalogSchemaVersion || c.schemaVersion != catalogs.CurrentCatalogSchemaVersion {
		return catalogstore.Generation{}, &errors.ValidationError{
			Field: "catalog_remote.schema_version", Value: c.schemaVersion,
			Message: fmt.Sprintf("must match exact current schema %d", catalogs.CurrentCatalogSchemaVersion),
		}
	}
	payload, err := c.fetch(ctx, SnapshotPath(manifest.GenerationID), catalogs.CatalogPayloadMediaType)
	if err != nil {
		return catalogstore.Generation{}, err
	}
	generation := catalogstore.Generation{Manifest: manifest, Payload: payload}
	if err := generation.Validate(); err != nil {
		return catalogstore.Generation{}, errors.WrapResource("verify", "remote catalog generation", manifest.GenerationID, err)
	}
	return generation, nil
}

func (c *Client) fetch(ctx context.Context, resourcePath, mediaType string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	target := *c.baseURL
	target.Path = path.Join(strings.TrimSuffix(c.baseURL.Path, "/"), resourcePath)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, errors.WrapResource("create", "remote catalog request", target.String(), err)
	}
	request.Header.Set("Accept", mediaType)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, &errors.APIError{Provider: "starmap-server", Endpoint: target.String(), Message: "request failed", Err: err}
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, &errors.APIError{Provider: "starmap-server", Endpoint: target.String(), StatusCode: response.StatusCode, Message: "unexpected response status"}
	}
	actualMediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || actualMediaType != mediaType {
		return nil, &errors.ValidationError{Field: "catalog_remote.content_type", Value: response.Header.Get("Content-Type"), Message: "does not match " + mediaType}
	}
	limited := io.LimitReader(response.Body, maxBodyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, errors.WrapIO("read", target.String(), err)
	}
	if len(data) > maxBodyBytes {
		return nil, &errors.ValidationError{Field: "catalog_remote.body", Value: len(data), Message: "exceeds maximum size"}
	}
	return data, nil
}

// MarshalManifest returns strict JSON bytes for the server route.
func MarshalManifest(manifest catalogs.GenerationManifest) ([]byte, error) {
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, errors.WrapResource("encode", "remote catalog manifest", manifest.GenerationID, err)
	}
	return data, nil
}
