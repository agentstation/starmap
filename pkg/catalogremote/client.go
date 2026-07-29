// Package catalogremote implements the versioned online Starmap-to-Starmap
// generation protocol and its verified client.
package catalogremote

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
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
	// EventStreamPath returns post-commit catalog publication hints over SSE.
	EventStreamPath = "/updates/stream"
	// CatalogPublishedEvent is the sole catalog publication event name.
	CatalogPublishedEvent = "catalog.published"
	// ManifestMediaType identifies strict generation-manifest JSON.
	ManifestMediaType    = "application/vnd.agentstation.starmap.catalog-manifest+json"
	maxBodyBytes         = 64 << 20
	maxGenerationIDBytes = 256
)

// Publication identifies one committed immutable catalog generation.
type Publication struct {
	GenerationID string `json:"generation_id"`
	Sequence     uint64 `json:"sequence"`
}

// GenerationManifestPath returns the immutable manifest route for generationID.
func GenerationManifestPath(generationID string) string {
	return GenerationsPath + "/" + url.PathEscape(generationID) + "/manifest"
}

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

// NewClient creates a remote generation client. baseURL is the trusted,
// versioned HTTPS API root, for example
// https://starmap.example.com/api/v1. Plain HTTP is accepted only on loopback.
// The supplied HTTP client may add authentication or stricter TLS policy, but
// HTTPS responses must retain a standard verified certificate chain.
func NewClient(baseURL string, httpClient *http.Client, schemaVersion uint64) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, &errors.ValidationError{Field: "catalog_remote.base_url", Value: baseURL, Message: "must be an absolute HTTP(S) versioned API URL"}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, &errors.ValidationError{Field: "catalog_remote.base_url", Value: baseURL, Message: "must use HTTP or HTTPS"}
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, &errors.ValidationError{
			Field:   "catalog_remote.base_url",
			Value:   baseURL,
			Message: "must not contain credentials, a query, or a fragment",
		}
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return nil, &errors.ValidationError{
			Field:   "catalog_remote.publisher",
			Value:   parsed.Host,
			Message: "non-loopback publishers must use HTTPS",
		}
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

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func sameOriginRedirectPolicy(origin *url.URL, previous func(*http.Request, []*http.Request) error, field string) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if !sameOrigin(request.URL, origin) {
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

func sameOrigin(left, right *url.URL) bool {
	return left != nil && right != nil &&
		strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Host, right.Host)
}

func (c *Client) verifyPublisher(response *http.Response) error {
	if response == nil || response.Request == nil ||
		!sameOrigin(response.Request.URL, c.baseURL) {
		return &errors.ValidationError{
			Field:   "catalog_remote.publisher",
			Message: "response does not identify the configured origin",
		}
	}
	if c.baseURL.Scheme == "http" {
		return nil
	}
	if response.TLS == nil || !response.TLS.HandshakeComplete ||
		len(response.TLS.VerifiedChains) == 0 {
		return &errors.ValidationError{
			Field:   "catalog_remote.publisher",
			Value:   c.baseURL.Host,
			Message: "HTTPS response has no verified publisher certificate chain",
		}
	}
	return nil
}

// FetchCurrent fetches the current manifest followed by its immutable,
// generation-addressed snapshot and validates their binding and compatibility.
func (c *Client) FetchCurrent(ctx context.Context) (catalogstore.Generation, error) {
	manifest, _, err := c.fetchManifest(ctx, ManifestPath, "current", "")
	if err != nil {
		return catalogstore.Generation{}, err
	}
	return c.fetchGenerationPayload(ctx, manifest)
}

// FetchCurrentIfChanged conditionally fetches the current manifest relative to
// generationID. It returns changed=false without fetching a payload when the
// publisher reports that generationID is still current.
func (c *Client) FetchCurrentIfChanged(
	ctx context.Context,
	generationID string,
) (generation catalogstore.Generation, changed bool, err error) {
	if err := validateGenerationID(generationID); err != nil {
		return catalogstore.Generation{}, false, err
	}
	manifest, notModified, err := c.fetchManifest(
		ctx,
		ManifestPath,
		"current",
		ManifestETag(generationID),
	)
	if err != nil {
		return catalogstore.Generation{}, false, err
	}
	if notModified {
		return catalogstore.Generation{}, false, nil
	}
	generation, err = c.fetchGenerationPayload(ctx, manifest)
	if err != nil {
		return catalogstore.Generation{}, false, err
	}
	return generation, true, nil
}

// FetchGeneration fetches and verifies one immutable generation by ID.
func (c *Client) FetchGeneration(ctx context.Context, generationID string) (catalogstore.Generation, error) {
	if err := validateGenerationID(generationID); err != nil {
		return catalogstore.Generation{}, err
	}
	manifest, _, err := c.fetchManifest(
		ctx,
		GenerationManifestPath(generationID),
		generationID,
		"",
	)
	if err != nil {
		return catalogstore.Generation{}, err
	}
	if manifest.GenerationID != generationID {
		return catalogstore.Generation{}, &errors.ValidationError{
			Field:   "catalog_remote.generation_id",
			Value:   manifest.GenerationID,
			Message: "does not match requested generation " + generationID,
		}
	}
	return c.fetchGenerationPayload(ctx, manifest)
}

func (c *Client) fetchManifest(
	ctx context.Context,
	resourcePath string,
	resourceID string,
	ifNoneMatch string,
) (catalogs.GenerationManifest, bool, error) {
	manifestData, notModified, err := c.fetchConditional(
		ctx,
		resourcePath,
		ManifestMediaType,
		ifNoneMatch,
	)
	if err != nil {
		return catalogs.GenerationManifest{}, false, err
	}
	if notModified {
		return catalogs.GenerationManifest{}, true, nil
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(manifestData)
	if err != nil {
		return catalogs.GenerationManifest{}, false, errors.WrapResource(
			"parse",
			"remote catalog manifest",
			resourceID,
			err,
		)
	}
	if err := validateGenerationID(manifest.GenerationID); err != nil {
		return catalogs.GenerationManifest{}, false, err
	}
	if !manifest.ConsumerCompatibility.SupportsSchema(c.schemaVersion) {
		return catalogs.GenerationManifest{}, false, &errors.ValidationError{
			Field: "catalog_remote.schema_version", Value: c.schemaVersion,
			Message: fmt.Sprintf("is incompatible with remote range %d..%d", manifest.ConsumerCompatibility.MinSchemaVersion, manifest.ConsumerCompatibility.MaxSchemaVersion),
		}
	}
	if manifest.Payload.MediaType != catalogs.CatalogPayloadMediaType {
		return catalogs.GenerationManifest{}, false, &errors.ValidationError{
			Field:   "catalog_remote.payload.media_type",
			Value:   manifest.Payload.MediaType,
			Message: "does not match " + catalogs.CatalogPayloadMediaType,
		}
	}
	if manifest.Payload.SizeBytes > maxBodyBytes {
		return catalogs.GenerationManifest{}, false, &errors.ValidationError{
			Field:   "catalog_remote.payload.size_bytes",
			Value:   manifest.Payload.SizeBytes,
			Message: "exceeds maximum size",
		}
	}
	return manifest, false, nil
}

func validateGenerationID(generationID string) error {
	if generationID == "" {
		return &errors.ValidationError{
			Field: "catalog_remote.generation_id", Message: "is required",
		}
	}
	if len(generationID) > maxGenerationIDBytes {
		return &errors.ValidationError{
			Field:   "catalog_remote.generation_id",
			Value:   len(generationID),
			Message: "exceeds maximum length",
		}
	}
	if generationID == "." || generationID == ".." ||
		strings.TrimSpace(generationID) != generationID {
		return &errors.ValidationError{
			Field:   "catalog_remote.generation_id",
			Value:   generationID,
			Message: "must be a canonical URL path segment",
		}
	}
	for _, character := range []byte(generationID) {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return &errors.ValidationError{
			Field:   "catalog_remote.generation_id",
			Value:   generationID,
			Message: "must contain only ASCII letters, digits, dot, dash, or underscore",
		}
	}
	return nil
}

func (c *Client) fetchGenerationPayload(
	ctx context.Context,
	manifest catalogs.GenerationManifest,
) (catalogstore.Generation, error) {
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
	data, _, err := c.fetchConditional(ctx, resourcePath, mediaType, "")
	return data, err
}

func (c *Client) fetchConditional(
	ctx context.Context,
	resourcePath string,
	mediaType string,
	ifNoneMatch string,
) ([]byte, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	target := *c.baseURL
	target.Path = path.Join(strings.TrimSuffix(c.baseURL.Path, "/"), resourcePath)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, false, errors.WrapResource("create", "remote catalog request", target.String(), err)
	}
	request.Header.Set("Accept", mediaType)
	if ifNoneMatch != "" {
		request.Header.Set("If-None-Match", ifNoneMatch)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, false, &errors.APIError{Provider: "starmap-server", Endpoint: target.String(), Message: "request failed", Err: err}
	}
	defer func() { _ = response.Body.Close() }()
	if err := c.verifyPublisher(response); err != nil {
		return nil, false, err
	}
	if response.StatusCode == http.StatusNotModified && ifNoneMatch != "" {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, true, nil
	}
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, false, &errors.APIError{Provider: "starmap-server", Endpoint: target.String(), StatusCode: response.StatusCode, Message: "unexpected response status"}
	}
	actualMediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || actualMediaType != mediaType {
		return nil, false, &errors.ValidationError{Field: "catalog_remote.content_type", Value: response.Header.Get("Content-Type"), Message: "does not match " + mediaType}
	}
	if response.ContentLength > maxBodyBytes {
		return nil, false, &errors.ValidationError{
			Field:   "catalog_remote.body",
			Value:   response.ContentLength,
			Message: "exceeds maximum size",
		}
	}
	limited := io.LimitReader(response.Body, maxBodyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, errors.WrapIO("read", target.String(), err)
	}
	if len(data) > maxBodyBytes {
		return nil, false, &errors.ValidationError{Field: "catalog_remote.body", Value: len(data), Message: "exceeds maximum size"}
	}
	return data, false, nil
}

// ManifestETag returns the strong entity tag for a generation manifest. A
// generation ID is immutable and restricted to HTTP entity-tag-safe bytes.
func ManifestETag(generationID string) string {
	return `"` + generationID + `"`
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
