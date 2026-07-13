// Package catalogdistribution provides the versioned hosted catalog
// distribution contract used by starmap.agentstation.ai and Starport clients.
package catalogdistribution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agentstation/starmap/pkg/catalogartifact"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// DefaultBaseURL is the canonical hosted catalog distribution origin.
	DefaultBaseURL = "https://starmap.agentstation.ai"
	// APIPrefix is the versioned hosted catalog route prefix.
	APIPrefix = "/v1/catalogs"
	// PointerVersion is the current latest-pointer schema.
	PointerVersion uint64 = 1
	// ImmutableCacheControl is sent for generation-addressed assets.
	ImmutableCacheControl = "public, max-age=31536000, immutable"
	// LatestCacheControl is sent for the mutable exact-schema latest pointer.
	LatestCacheControl = "public, max-age=60, must-revalidate"
	maxHostedBodyBytes = 64 << 20
)

// AssetDescriptor identifies one immutable hosted byte object.
type AssetDescriptor struct {
	URL       string `json:"url"`
	MediaType string `json:"media_type"`
	Checksum  string `json:"checksum"`
	SizeBytes int64  `json:"size_bytes"`
}

// LatestPointer selects one immutable generation for the exact current schema.
type LatestPointer struct {
	Version       uint64          `json:"version"`
	Channel       Channel         `json:"channel"`
	GenerationID  string          `json:"generation_id"`
	SchemaVersion uint64          `json:"schema_version"`
	Artifact      AssetDescriptor `json:"artifact"`
	Attestation   AssetDescriptor `json:"attestation"`
}

// PublishedGeneration is one verified generation and its exact distribution
// artifact set.
type PublishedGeneration struct {
	Generation catalogstore.Generation
	Artifact   catalogartifact.Artifact
}

// Validate verifies that the artifact opens to exactly the supplied generation.
func (p PublishedGeneration) Validate() error {
	if err := p.Generation.Validate(); err != nil {
		return err
	}
	opened, err := catalogartifact.Open(p.Artifact.Data, p.Artifact.Attestation)
	if err != nil {
		return err
	}
	if opened.Manifest.GenerationID != p.Generation.Manifest.GenerationID ||
		opened.Manifest.Payload != p.Generation.Manifest.Payload || !bytes.Equal(opened.Payload, p.Generation.Payload) {
		return &errors.ValidationError{Field: "catalog_distribution.generation", Value: opened.Manifest.GenerationID, Message: "artifact does not match published generation"}
	}
	return nil
}

// Repository is the narrow hosted read boundary.
type Repository interface {
	LatestForChannel(channel Channel, schemaVersion uint64) (PublishedGeneration, error)
	Get(generationID string) (PublishedGeneration, error)
}

// MemoryRepository is a thread-safe immutable hosted repository for tests and
// single-process deployments.
type MemoryRepository struct {
	mu       sync.RWMutex
	items    map[string]PublishedGeneration
	channels map[Channel]string
	history  map[Channel]map[string]struct{}
	events   []PromotionEvent
	sequence uint64
	policy   PromotionPolicy
	now      func() time.Time
}

// NewMemoryRepository creates an empty hosted repository.
func NewMemoryRepository() *MemoryRepository {
	return newMemoryRepository(DefaultPromotionPolicy())
}

// Publish adds an immutable generation. An exact retry is idempotent; reuse of
// an existing generation ID with different bytes is a conflict.
func (r *MemoryRepository) Publish(published PublishedGeneration) error {
	if err := published.Validate(); err != nil {
		return err
	}
	id := published.Generation.Manifest.GenerationID
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, found := r.items[id]; found {
		if existing.Artifact.Checksum != published.Artifact.Checksum ||
			!bytes.Equal(existing.Artifact.Attestation, published.Artifact.Attestation) {
			return &errors.ConflictError{Resource: "hosted catalog generation", Expected: id, Actual: id, Message: "generation ID is immutable"}
		}
		return nil
	}
	r.items[id] = copyPublished(published)
	return nil
}

// Latest returns the stable generation only for the exact current schema.
func (r *MemoryRepository) Latest(schemaVersion uint64) (PublishedGeneration, error) {
	return r.LatestForChannel(ChannelStable, schemaVersion)
}

// LatestForChannel returns the selected channel generation only for the exact
// current schema.
func (r *MemoryRepository) LatestForChannel(channel Channel, schemaVersion uint64) (PublishedGeneration, error) {
	if err := channel.Validate(); err != nil {
		return PublishedGeneration{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	generationID := r.channels[channel]
	published, found := r.items[generationID]
	if !found {
		return PublishedGeneration{}, &errors.NotFoundError{Resource: "hosted catalog channel " + channel.String(), ID: generationID}
	}
	if schemaVersion != catalogs.CurrentCatalogSchemaVersion || published.Generation.Manifest.SchemaVersion != catalogs.CurrentCatalogSchemaVersion {
		return PublishedGeneration{}, &errors.ValidationError{Field: "catalog_distribution.schema_version", Value: schemaVersion, Message: "must match the exact current schema"}
	}
	return copyPublished(published), nil
}

// Get returns one immutable generation by logical ID.
func (r *MemoryRepository) Get(generationID string) (PublishedGeneration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	published, found := r.items[generationID]
	if !found {
		return PublishedGeneration{}, &errors.NotFoundError{Resource: "hosted catalog generation", ID: generationID}
	}
	return copyPublished(published), nil
}

// Handler serves exact-schema latest pointers and immutable generation assets.
type Handler struct {
	repository Repository
}

// NewHandler creates the versioned hosted distribution HTTP adapter.
func NewHandler(repository Repository) (*Handler, error) {
	if repository == nil {
		return nil, &errors.ValidationError{Field: "catalog_distribution.repository", Message: "is required"}
	}
	return &Handler{repository: repository}, nil
}

// ServeHTTP serves /v1/catalogs/latest and immutable generation assets.
func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimSuffix(request.URL.Path, "/")
	if path == APIPrefix+"/latest" {
		h.serveLatest(writer, request)
		return
	}
	prefix := APIPrefix + "/"
	if !strings.HasPrefix(path, prefix) {
		http.NotFound(writer, request)
		return
	}
	remainder := strings.TrimPrefix(path, prefix)
	parts := strings.Split(remainder, "/")
	if len(parts) == 1 {
		h.serveArtifact(writer, request, parts[0], false)
		return
	}
	if len(parts) == 2 && parts[1] == "attestation" {
		h.serveArtifact(writer, request, parts[0], true)
		return
	}
	http.NotFound(writer, request)
}

func (h *Handler) serveLatest(writer http.ResponseWriter, request *http.Request) {
	schemaVersion, err := strconv.ParseUint(request.URL.Query().Get("schema_version"), 10, 64)
	if err != nil || schemaVersion == 0 {
		http.Error(writer, "schema_version is required", http.StatusBadRequest)
		return
	}
	channel, err := ParseChannel(request.URL.Query().Get("channel"))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	published, err := h.repository.LatestForChannel(channel, schemaVersion)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusNotFound)
		return
	}
	pointer := pointerFor(published, channel)
	data, err := json.Marshal(pointer)
	if err != nil {
		http.Error(writer, "could not encode latest pointer", http.StatusInternalServerError)
		return
	}
	etag := entityTag(checksum(data))
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", LatestCacheControl)
	writer.Header().Set("ETag", etag)
	if request.Header.Get("If-None-Match") == etag {
		writer.WriteHeader(http.StatusNotModified)
		return
	}
	_, _ = writer.Write(data)
}

func (h *Handler) serveArtifact(writer http.ResponseWriter, request *http.Request, generationID string, attestation bool) {
	published, err := h.repository.Get(generationID)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	if attestation {
		writer.Header().Set("Content-Type", "application/vnd.in-toto+json")
		writer.Header().Set("Cache-Control", ImmutableCacheControl)
		writer.Header().Set("ETag", entityTag(checksum(published.Artifact.Attestation)))
		if request.Header.Get("If-None-Match") == writer.Header().Get("ETag") {
			writer.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = writer.Write(published.Artifact.Attestation)
		return
	}
	writer.Header().Set("Content-Type", catalogartifact.MediaType)
	writer.Header().Set("Cache-Control", ImmutableCacheControl)
	writer.Header().Set("ETag", entityTag(published.Artifact.Checksum))
	if request.Header.Get("If-None-Match") == writer.Header().Get("ETag") {
		writer.WriteHeader(http.StatusNotModified)
		return
	}
	_, _ = writer.Write(published.Artifact.Data)
}

func pointerFor(published PublishedGeneration, channel Channel) LatestPointer {
	manifest := published.Generation.Manifest
	return LatestPointer{
		Version: PointerVersion, Channel: channel, GenerationID: manifest.GenerationID,
		SchemaVersion: manifest.SchemaVersion,
		Artifact: AssetDescriptor{
			URL: APIPrefix + "/" + url.PathEscape(manifest.GenerationID), MediaType: catalogartifact.MediaType,
			Checksum: published.Artifact.Checksum, SizeBytes: int64(len(published.Artifact.Data)),
		},
		Attestation: AssetDescriptor{
			URL:       APIPrefix + "/" + url.PathEscape(manifest.GenerationID) + "/attestation",
			MediaType: "application/vnd.in-toto+json", Checksum: checksum(published.Artifact.Attestation),
			SizeBytes: int64(len(published.Artifact.Attestation)),
		},
	}
}

// Client fetches and verifies hosted catalog generations.
type Client struct {
	baseURL       *url.URL
	httpClient    *http.Client
	schemaVersion uint64
}

// NewClient creates a hosted distribution client for one consumer schema.
func NewClient(baseURL string, httpClient *http.Client, schemaVersion uint64) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, &errors.ValidationError{Field: "catalog_distribution.base_url", Value: baseURL, Message: "must be an absolute HTTP(S) URL"}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, &errors.ValidationError{Field: "catalog_distribution.base_url", Value: baseURL, Message: "must use HTTP or HTTPS"}
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: constants.DefaultHTTPTimeout}
	}
	if schemaVersion != catalogs.CurrentCatalogSchemaVersion {
		return nil, &errors.ValidationError{Field: "catalog_distribution.schema_version", Value: schemaVersion, Message: fmt.Sprintf("must be exactly %d", catalogs.CurrentCatalogSchemaVersion)}
	}
	client := *httpClient
	previousRedirectPolicy := client.CheckRedirect
	client.CheckRedirect = sameOriginRedirectPolicy(parsed, previousRedirectPolicy)
	return &Client{baseURL: parsed, httpClient: &client, schemaVersion: schemaVersion}, nil
}

func sameOriginRedirectPolicy(origin *url.URL, previous func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if !strings.EqualFold(request.URL.Scheme, origin.Scheme) || !strings.EqualFold(request.URL.Host, origin.Host) {
			return &errors.ValidationError{Field: "catalog_distribution.redirect", Value: request.URL.String(), Message: "must remain on the configured origin"}
		}
		if previous != nil {
			return previous(request, via)
		}
		if len(via) >= 10 {
			return &errors.ValidationError{Field: "catalog_distribution.redirect", Value: len(via), Message: "exceeds maximum redirect count"}
		}
		return nil
	}
}

// FetchLatest resolves, downloads, and verifies the exact-schema latest immutable generation.
func (c *Client) FetchLatest(ctx context.Context) (catalogstore.Generation, error) {
	return c.FetchChannel(ctx, ChannelStable)
}

// FetchChannel resolves, downloads, and verifies the exact-schema latest
// immutable generation selected for one explicit promotion channel.
func (c *Client) FetchChannel(ctx context.Context, channel Channel) (catalogstore.Generation, error) {
	generation, _, err := c.fetchChannel(ctx, channel)
	return generation, err
}

func (c *Client) fetchChannel(ctx context.Context, channel Channel) (catalogstore.Generation, LatestPointer, error) {
	if err := channel.Validate(); err != nil {
		return catalogstore.Generation{}, LatestPointer{}, err
	}
	latest := c.baseURL.ResolveReference(&url.URL{Path: APIPrefix + "/latest"})
	query := latest.Query()
	query.Set("schema_version", strconv.FormatUint(c.schemaVersion, 10))
	query.Set("channel", channel.String())
	latest.RawQuery = query.Encode()
	pointerData, pointerMediaType, err := c.fetch(ctx, latest, "application/json")
	if err != nil {
		return catalogstore.Generation{}, LatestPointer{}, err
	}
	if pointerMediaType != "application/json" {
		return catalogstore.Generation{}, LatestPointer{}, &errors.ValidationError{Field: "catalog_distribution.latest", Value: pointerMediaType, Message: "has unexpected media type"}
	}
	var pointer LatestPointer
	if err := decodeStrictJSON(pointerData, &pointer); err != nil {
		return catalogstore.Generation{}, LatestPointer{}, err
	}
	if pointer.Version != PointerVersion || pointer.Channel != channel || pointer.GenerationID == "" ||
		pointer.SchemaVersion != catalogs.CurrentCatalogSchemaVersion || pointer.SchemaVersion != c.schemaVersion {
		return catalogstore.Generation{}, LatestPointer{}, &errors.ValidationError{Field: "catalog_distribution.latest", Value: pointer.GenerationID, Message: "is incompatible or malformed"}
	}
	artifactURL, err := c.assetURL(pointer.Artifact.URL)
	if err != nil {
		return catalogstore.Generation{}, LatestPointer{}, err
	}
	attestationURL, err := c.assetURL(pointer.Attestation.URL)
	if err != nil {
		return catalogstore.Generation{}, LatestPointer{}, err
	}
	artifact, mediaType, err := c.fetch(ctx, artifactURL, pointer.Artifact.MediaType)
	if err != nil {
		return catalogstore.Generation{}, LatestPointer{}, err
	}
	if err := verifyAsset(pointer.Artifact, artifact, mediaType); err != nil {
		return catalogstore.Generation{}, LatestPointer{}, err
	}
	attestation, mediaType, err := c.fetch(ctx, attestationURL, pointer.Attestation.MediaType)
	if err != nil {
		return catalogstore.Generation{}, LatestPointer{}, err
	}
	if err := verifyAsset(pointer.Attestation, attestation, mediaType); err != nil {
		return catalogstore.Generation{}, LatestPointer{}, err
	}
	generation, err := catalogartifact.Open(artifact, attestation)
	if err != nil {
		return catalogstore.Generation{}, LatestPointer{}, err
	}
	manifest := generation.Manifest
	if manifest.GenerationID != pointer.GenerationID || manifest.SchemaVersion != pointer.SchemaVersion {
		return catalogstore.Generation{}, LatestPointer{}, &errors.ValidationError{Field: "catalog_distribution.latest", Value: pointer.GenerationID, Message: "does not match downloaded generation"}
	}
	return generation, pointer, nil
}

func (c *Client) assetURL(reference string) (*url.URL, error) {
	parsed, err := url.Parse(reference)
	if err != nil {
		return nil, &errors.ValidationError{Field: "catalog_distribution.asset_url", Value: reference, Message: err.Error()}
	}
	resolved := c.baseURL.ResolveReference(parsed)
	if resolved.Scheme != c.baseURL.Scheme || resolved.Host != c.baseURL.Host {
		return nil, &errors.ValidationError{Field: "catalog_distribution.asset_url", Value: reference, Message: "must remain on the configured origin"}
	}
	return resolved, nil
}

func (c *Client) fetch(ctx context.Context, endpoint *url.URL, mediaType string) ([]byte, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("Accept", mediaType)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, "", &errors.APIError{Provider: "starmap-distribution", Endpoint: endpoint.String(), Message: "request failed", Err: err}
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, "", &errors.APIError{Provider: "starmap-distribution", Endpoint: endpoint.String(), StatusCode: response.StatusCode, Message: "unexpected hosted response"}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxHostedBodyBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxHostedBodyBytes {
		return nil, "", &errors.ValidationError{Field: "catalog_distribution.response", Value: len(data), Message: "exceeds maximum hosted body size"}
	}
	media := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	return data, media, nil
}

func verifyAsset(descriptor AssetDescriptor, data []byte, mediaType string) error {
	if descriptor.SizeBytes != int64(len(data)) || descriptor.Checksum != checksum(data) || descriptor.MediaType != mediaType {
		return &errors.ValidationError{Field: "catalog_distribution.asset", Value: descriptor.URL, Message: "size, checksum, or media type does not match latest pointer"}
	}
	return nil
}

func decodeStrictJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return &errors.ParseError{Format: "json", File: "hosted latest catalog", Message: err.Error(), Err: err}
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return &errors.ParseError{Format: "json", File: "hosted latest catalog", Message: "invalid trailing JSON", Err: err}
	}
	return nil
}

func checksum(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func entityTag(checksumValue string) string {
	return `"` + strings.TrimPrefix(checksumValue, "sha256:") + `"`
}

func copyPublished(published PublishedGeneration) PublishedGeneration {
	copyArtifact := published.Artifact
	copyArtifact.Data = append([]byte(nil), published.Artifact.Data...)
	copyArtifact.Attestation = append([]byte(nil), published.Artifact.Attestation...)
	return PublishedGeneration{Generation: published.Generation.Copy(), Artifact: copyArtifact}
}

// String returns a concise latest-pointer description.
func (p LatestPointer) String() string {
	return fmt.Sprintf("channel=%s generation=%s schema=%d artifact=%s", p.Channel, p.GenerationID, p.SchemaVersion, p.Artifact.Checksum)
}
