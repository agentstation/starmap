package modelsdev

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/embedded"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

const (
	// ModelsDevAPIURL is the URL for the models.dev API.
	ModelsDevAPIURL = "https://models.dev/api.json"
	// HTTPCacheTTL is the cache time-to-live for HTTP responses.
	HTTPCacheTTL = 1 * time.Hour

	minimumModelsDevProviders         = 5
	minimumModelsDevPromotionModels   = 100
	minimumRetainedProviderPercentage = 80
	minimumRetainedModelPercentage    = 50
)

// HTTPClient handles HTTP downloading of models.dev api.json.
type HTTPClient struct {
	CacheDir string
	APIURL   string
	Client   *http.Client
	nowFunc  func() time.Time
}

// HTTPAcquisition identifies which evidence path satisfied one HTTP load.
type HTTPAcquisition string

const (
	// HTTPAcquisitionFreshCache means a validated cache within TTL was used.
	HTTPAcquisitionFreshCache HTTPAcquisition = "fresh_cache"
	// HTTPAcquisitionDownloaded means a validated upstream response was promoted.
	HTTPAcquisitionDownloaded HTTPAcquisition = "downloaded"
	// HTTPAcquisitionRevalidatedCache means a conditional request returned 304.
	HTTPAcquisitionRevalidatedCache HTTPAcquisition = "revalidated_cache"
	// HTTPAcquisitionStaleCache means upstream failed and stale last-known-good evidence was used.
	HTTPAcquisitionStaleCache HTTPAcquisition = "stale_cache"
	// HTTPAcquisitionEmbeddedBootstrap means neither upstream nor cache was usable.
	HTTPAcquisitionEmbeddedBootstrap HTTPAcquisition = "embedded_bootstrap"
)

// HTTPAcquisitionResult reports the evidence path and retained source revision.
type HTTPAcquisitionResult struct {
	Kind     HTTPAcquisition
	Revision sources.Revision
	// Issues contains classified evidence explaining why upstream data was
	// rejected before a fallback was selected.
	Issues []sources.ObservationIssue
}

// APIPromotion describes a models.dev payload that passed typed and semantic
// validation and was atomically promoted to its destination.
type APIPromotion struct {
	Checksum           string `json:"checksum"`
	SizeBytes          int64  `json:"size_bytes"`
	ProviderCount      int    `json:"provider_count"`
	ModelCount         int    `json:"model_count"`
	RejectedModelCount int    `json:"rejected_model_count"`
}

const httpCacheMetadataVersion uint64 = 1

type httpCacheMetadata struct {
	Version         uint64          `json:"version"`
	Origin          HTTPAcquisition `json:"origin"`
	ETag            string          `json:"etag,omitempty"`
	LastModified    string          `json:"last_modified,omitempty"`
	ContentChecksum string          `json:"content_checksum"`
	ValidatedAt     time.Time       `json:"validated_at"`
}

// NewHTTPClient creates a new models.dev HTTP client.
func NewHTTPClient(outputDir string) *HTTPClient {
	if outputDir == "" {
		outputDir = expandPath(constants.DefaultSourcesPath)
	}
	cacheDir := filepath.Join(outputDir, "models.dev")
	return &HTTPClient{
		CacheDir: cacheDir,
		APIURL:   ModelsDevAPIURL,
		Client:   &http.Client{Timeout: constants.DefaultHTTPTimeout},
	}
}

// EnsureAPI ensures the api.json is available and up to date.
func (c *HTTPClient) EnsureAPI(ctx context.Context) error {
	_, err := c.AcquireAPI(ctx)
	return err
}

// AcquireAPI ensures api.json is available and reports the exact evidence path.
func (c *HTTPClient) AcquireAPI(ctx context.Context) (HTTPAcquisitionResult, error) {
	ctx = logging.WithSource(ctx, sources.ModelsDevHTTPID.String())
	logger := logging.FromContext(ctx)
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(c.CacheDir, constants.DirPermissions); err != nil {
		return HTTPAcquisitionResult{}, errors.WrapIO("create", "cache directory", err)
	}

	apiPath := c.GetAPIPath()
	cachedAPI, cacheValidationErr := readValidatedAPIFile(apiPath)
	cacheUsable := cacheValidationErr == nil
	metadata, metadataErr := c.readCacheMetadata(apiPath)

	// Only metadata-bound cache entries can be called fresh. Legacy or damaged
	// sidecars are re-fetched and become degraded fallback if upstream is down.
	if cacheUsable && metadataErr == nil && c.isCacheValid(apiPath) {
		if metadata.Origin == HTTPAcquisitionEmbeddedBootstrap {
			logger.Info().Str("acquisition", string(HTTPAcquisitionEmbeddedBootstrap)).Msg("Using models.dev bootstrap cache")
			return acquisitionResult(HTTPAcquisitionEmbeddedBootstrap, metadata), nil
		}
		if metadata.Origin == HTTPAcquisitionDownloaded {
			logger.Info().Str("acquisition", string(HTTPAcquisitionFreshCache)).Msg("Using models.dev cache")
			return acquisitionResult(HTTPAcquisitionFreshCache, metadata), nil
		}
	}

	// Download fresh api.json
	logger.Info().Str("url", c.APIURL).Msg("Fetching models.dev catalog")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.APIURL, nil)
	if err != nil {
		return c.useCacheFallback(ctx, apiPath)
	}
	if cacheUsable && metadataErr == nil {
		if metadata.ETag != "" {
			req.Header.Set("If-None-Match", metadata.ETag)
		}
		if metadata.LastModified != "" {
			req.Header.Set("If-Modified-Since", metadata.LastModified)
		}
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		logger.Warn().Err(err).Msg("models.dev HTTP request failed")
		return c.useCacheFallback(ctx, apiPath)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		if !cacheUsable || metadataErr != nil || (metadata.ETag == "" && metadata.LastModified == "") {
			logger.Warn().Int("status_code", resp.StatusCode).Msg("models.dev returned 304 without matching cache metadata")
			return c.useCacheFallback(ctx, apiPath)
		}
		metadata.ETag = responseHeaderOr(resp, "ETag", metadata.ETag)
		metadata.LastModified = responseHeaderOr(resp, "Last-Modified", metadata.LastModified)
		metadata.ValidatedAt = c.now().UTC()
		if err := c.writeCacheMetadata(apiPath, metadata); err != nil {
			return HTTPAcquisitionResult{}, errors.WrapIO("write", "api.json metadata", err)
		}
		logger.Info().Str("acquisition", string(HTTPAcquisitionRevalidatedCache)).Str("revision", acquisitionResult(HTTPAcquisitionRevalidatedCache, metadata).Revision.Value).Msg("Revalidated models.dev cache")
		return acquisitionResult(HTTPAcquisitionRevalidatedCache, metadata), nil
	}

	// Only accept HTTP 200 OK or a validated 304 path above.
	if resp.StatusCode != http.StatusOK {
		logger.Warn().Int("status_code", resp.StatusCode).Str("status", resp.Status).Msg("models.dev HTTP request returned non-success status")
		return c.useCacheFallback(ctx, apiPath)
	}

	// Read response body
	data, err := io.ReadAll(io.LimitReader(resp.Body, constants.MaxSourcePayloadBytes+1))
	if err != nil {
		logger.Warn().Err(err).Msg("Could not read models.dev HTTP response")
		return c.useCacheFallback(ctx, apiPath)
	}
	if len(data) > constants.MaxSourcePayloadBytes {
		logger.Warn().Int("response_bytes", len(data)).Int("maximum_bytes", constants.MaxSourcePayloadBytes).Msg("models.dev response exceeds maximum size")
		return c.useCacheFallback(ctx, apiPath, sources.ObservationIssue{
			Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodePayloadLimit,
			Message: "upstream models.dev payload exceeded the source byte budget and was rejected",
		})
	}

	// Validate minimum size (round number, ~1/3 of typical ~267KB)
	const minValidSize = 100000
	if len(data) < minValidSize {
		logger.Warn().Int("response_bytes", len(data)).Int("minimum_bytes", minValidSize).Msg("models.dev response is below minimum size")
		return c.useCacheFallback(ctx, apiPath)
	}

	// Typed parsing must succeed before the response can replace the
	// last-known-good cache. A syntactically valid object can still be
	// incompatible with the source schema.
	api, err := parseAPIData(data)
	if err != nil {
		logger.Warn().Err(err).Msg("models.dev response is schema-incompatible")
		return c.useCacheFallback(ctx, apiPath, semanticPromotionIssue("typed decoding failed"))
	}
	if err := validateAPIPromotion(api, cachedAPI); err != nil {
		logger.Warn().Err(err).Msg("models.dev response failed semantic promotion")
		return c.useCacheFallback(ctx, apiPath, semanticPromotionIssue(err.Error()))
	}

	// Atomically promote validated data so readers see either the prior complete
	// cache or the new complete cache.
	if err := writeFileAtomically(apiPath, data); err != nil {
		return HTTPAcquisitionResult{}, errors.WrapIO("write", "api.json", err)
	}
	metadata = httpCacheMetadata{
		Version: httpCacheMetadataVersion, Origin: HTTPAcquisitionDownloaded,
		ETag: strings.TrimSpace(resp.Header.Get("ETag")), LastModified: strings.TrimSpace(resp.Header.Get("Last-Modified")),
		ContentChecksum: checksumBytes(data), ValidatedAt: c.now().UTC(),
	}
	if err := c.writeCacheMetadata(apiPath, metadata); err != nil {
		return HTTPAcquisitionResult{}, errors.WrapIO("write", "api.json metadata", err)
	}

	logger.Info().Str("acquisition", string(HTTPAcquisitionDownloaded)).Int("response_bytes", len(data)).Msg("Downloaded models.dev catalog")
	return acquisitionResult(HTTPAcquisitionDownloaded, metadata), nil
}

// GetAPIPath returns the path to the cached api.json file.
func (c *HTTPClient) GetAPIPath() string {
	return filepath.Join(c.CacheDir, "api.json")
}

// Cleanup removes the cache directory.
func (c *HTTPClient) Cleanup() error {
	if _, err := os.Stat(c.CacheDir); os.IsNotExist(err) {
		return nil // Already cleaned up
	}
	return os.RemoveAll(c.CacheDir)
}

// isCacheValid checks if the cached api.json is recent enough.
func (c *HTTPClient) isCacheValid(apiPath string) bool {
	if metadata, err := c.readCacheMetadata(apiPath); err == nil {
		return c.now().Sub(metadata.ValidatedAt) < HTTPCacheTTL
	}
	info, err := os.Stat(apiPath)
	if err != nil {
		return false // File doesn't exist
	}

	// Check if file is recent enough
	return c.now().Sub(info.ModTime()) < HTTPCacheTTL
}

// useCacheFallback tries cached data first, then embedded as final fallback.
func (c *HTTPClient) useCacheFallback(ctx context.Context, apiPath string, issues ...sources.ObservationIssue) (HTTPAcquisitionResult, error) {
	logger := logging.FromContext(ctx)
	// Try existing cache file (even if stale), but only after the same typed
	// validation required for a downloaded response.
	if err := validateAPIFile(apiPath); err == nil {
		if metadata, metadataErr := c.readCacheMetadata(apiPath); metadataErr == nil {
			if metadata.Origin == HTTPAcquisitionEmbeddedBootstrap {
				logger.Info().Str("acquisition", string(HTTPAcquisitionEmbeddedBootstrap)).Msg("Using models.dev bootstrap cache")
				result := acquisitionResult(HTTPAcquisitionEmbeddedBootstrap, metadata)
				result.Issues = append(result.Issues, issues...)
				return result, nil
			}
			logger.Warn().Str("acquisition", string(HTTPAcquisitionStaleCache)).Msg("Using stale models.dev cache")
			result := acquisitionResult(HTTPAcquisitionStaleCache, metadata)
			result.Issues = append(result.Issues, issues...)
			return result, nil
		}
		logger.Warn().Str("acquisition", string(HTTPAcquisitionStaleCache)).Msg("Using unverified legacy models.dev cache")
		return HTTPAcquisitionResult{
			Kind: HTTPAcquisitionStaleCache, Issues: append([]sources.ObservationIssue(nil), issues...),
		}, nil
	} else if !os.IsNotExist(err) {
		logger.Warn().Err(err).Msg("models.dev cache is unusable")
	}

	// Fall back to embedded data as last resort
	result, err := c.useEmbeddedFallback(ctx, apiPath)
	result.Issues = append(result.Issues, issues...)
	return result, err
}

// useEmbeddedFallback copies embedded api.json to cache when all else fails.
func (c *HTTPClient) useEmbeddedFallback(ctx context.Context, apiPath string) (HTTPAcquisitionResult, error) {
	logger := logging.FromContext(ctx)
	logger.Warn().Str("acquisition", string(HTTPAcquisitionEmbeddedBootstrap)).Msg("Falling back to embedded models.dev bootstrap")

	// Read embedded api.json
	embeddedData, err := embedded.FS.ReadFile("sources/models.dev/api.json")
	if err != nil {
		return HTTPAcquisitionResult{}, errors.WrapResource("read", "embedded api.json", "", err)
	}
	embeddedAPI, err := parseAPIData(embeddedData)
	if err != nil {
		return HTTPAcquisitionResult{}, errors.WrapResource("validate", "embedded api.json", "", err)
	}
	if _, err := validateAPISemantics(embeddedAPI); err != nil {
		return HTTPAcquisitionResult{}, errors.WrapResource("validate", "embedded api.json semantics", "", err)
	}

	// Write to cache location atomically.
	if err := writeFileAtomically(apiPath, embeddedData); err != nil {
		return HTTPAcquisitionResult{}, errors.WrapIO("write", "api.json fallback", err)
	}
	metadata := httpCacheMetadata{
		Version: httpCacheMetadataVersion, Origin: HTTPAcquisitionEmbeddedBootstrap,
		ContentChecksum: checksumBytes(embeddedData), ValidatedAt: c.now().UTC(),
	}
	if err := c.writeCacheMetadata(apiPath, metadata); err != nil {
		return HTTPAcquisitionResult{}, errors.WrapIO("write", "api.json fallback metadata", err)
	}

	logger.Info().Str("acquisition", string(HTTPAcquisitionEmbeddedBootstrap)).Msg("Persisted embedded models.dev bootstrap cache")
	return acquisitionResult(HTTPAcquisitionEmbeddedBootstrap, metadata), nil
}

func validateAPIFile(apiPath string) error {
	_, err := readValidatedAPIFile(apiPath)
	return err
}

func readValidatedAPIFile(apiPath string) (*API, error) {
	_, api, err := readValidatedAPIFileData(apiPath)
	return api, err
}

func readValidatedAPIFileData(apiPath string) ([]byte, *API, error) {
	info, err := os.Stat(apiPath)
	if err != nil {
		return nil, nil, err
	}
	if info.Size() > constants.MaxSourcePayloadBytes {
		return nil, nil, &errors.ValidationError{
			Field: "models_dev.payload_size", Value: info.Size(),
			Message: fmt.Sprintf("must not exceed %d bytes", constants.MaxSourcePayloadBytes),
		}
	}
	data, err := os.ReadFile(apiPath) //nolint:gosec // Cache path is controlled by Starmap.
	if err != nil {
		return nil, nil, err
	}
	if len(data) > constants.MaxSourcePayloadBytes {
		return nil, nil, &errors.ValidationError{
			Field: "models_dev.payload_size", Value: len(data),
			Message: fmt.Sprintf("must not exceed %d bytes", constants.MaxSourcePayloadBytes),
		}
	}
	api, err := parseAPIData(data)
	if err != nil {
		return nil, nil, err
	}
	if _, err := validateAPISemantics(api); err != nil {
		return nil, nil, err
	}
	return data, api, nil
}

// PromoteAPIFile validates a downloaded models.dev payload against the same
// typed, semantic, and completeness policy used by runtime cache promotion,
// then atomically replaces the destination. A failed candidate leaves the
// destination byte-for-byte unchanged.
func PromoteAPIFile(candidatePath, destinationPath string) (APIPromotion, error) {
	if strings.TrimSpace(candidatePath) == "" {
		return APIPromotion{}, &errors.ValidationError{Field: "models_dev.candidate_path", Message: "is required"}
	}
	if strings.TrimSpace(destinationPath) == "" {
		return APIPromotion{}, &errors.ValidationError{Field: "models_dev.destination_path", Message: "is required"}
	}

	data, candidate, err := readValidatedAPIFileData(candidatePath)
	if err != nil {
		return APIPromotion{}, errors.WrapResource("validate", "models.dev candidate", candidatePath, err)
	}
	stats, err := validateAPISemantics(candidate)
	if err != nil {
		return APIPromotion{}, errors.WrapResource("validate", "models.dev candidate", candidatePath, err)
	}

	var current *API
	current, err = readValidatedAPIFile(destinationPath)
	if err != nil && !os.IsNotExist(err) {
		return APIPromotion{}, errors.WrapResource("validate", "models.dev current payload", destinationPath, err)
	}
	if err := validateAPIPromotion(candidate, current); err != nil {
		return APIPromotion{}, errors.WrapResource("promote", "models.dev candidate", candidatePath, err)
	}

	if err := os.MkdirAll(filepath.Dir(destinationPath), constants.DirPermissions); err != nil {
		return APIPromotion{}, errors.WrapIO("create", filepath.Dir(destinationPath), err)
	}
	if err := writeFileAtomically(destinationPath, data); err != nil {
		return APIPromotion{}, errors.WrapIO("promote", destinationPath, err)
	}
	return APIPromotion{
		Checksum: checksumBytes(data), SizeBytes: int64(len(data)),
		ProviderCount: stats.providers, ModelCount: stats.models,
		RejectedModelCount: stats.rejectedModels,
	}, nil
}

type apiSemanticStats struct {
	providers      int
	models         int
	rejectedModels int
}

func validateAPISemantics(api *API) (apiSemanticStats, error) {
	if api == nil {
		return apiSemanticStats{}, &errors.ValidationError{Field: "models_dev.api", Message: "is required"}
	}
	stats := apiSemanticStats{providers: len(*api)}
	if stats.providers < minimumModelsDevProviders {
		return stats, &errors.ValidationError{
			Field: "models_dev.providers", Value: stats.providers,
			Message: fmt.Sprintf("must contain at least %d providers", minimumModelsDevProviders),
		}
	}

	providerKeys := make([]string, 0, len(*api))
	for key := range *api {
		providerKeys = append(providerKeys, key)
	}
	sort.Strings(providerKeys)
	for _, providerKey := range providerKeys {
		provider := (*api)[providerKey]
		if strings.TrimSpace(providerKey) == "" || provider.ID != providerKey {
			return stats, &errors.ValidationError{
				Field: "models_dev.provider.id", Value: provider.ID,
				Message: fmt.Sprintf("must match map identity %q", providerKey),
			}
		}
		if strings.TrimSpace(provider.Name) == "" {
			return stats, &errors.ValidationError{Field: "models_dev.provider.name", Value: provider.Name, Message: "is required"}
		}
		if provider.Models == nil {
			return stats, &errors.ValidationError{
				Field: "models_dev.provider.models", Value: providerKey, Message: "must be an object",
			}
		}

		modelKeys := make([]string, 0, len(provider.Models))
		for key := range provider.Models {
			modelKeys = append(modelKeys, key)
		}
		sort.Strings(modelKeys)
		for _, modelKey := range modelKeys {
			model := provider.Models[modelKey]
			if err := validateModelsDevModelIdentity(modelKey, &model); err != nil {
				stats.rejectedModels++
				continue
			}
			stats.models++
		}
	}
	return stats, nil
}

func validateAPIPromotion(candidate, current *API) error {
	candidateStats, err := validateAPISemantics(candidate)
	if err != nil {
		return err
	}
	if candidateStats.models < minimumModelsDevPromotionModels {
		return &errors.ValidationError{
			Field: "models_dev.models", Value: candidateStats.models,
			Message: fmt.Sprintf("promotion requires at least %d models", minimumModelsDevPromotionModels),
		}
	}
	if current == nil {
		return nil
	}
	currentStats, err := validateAPISemantics(current)
	if err != nil {
		return nil
	}
	if candidateStats.providers*100 < currentStats.providers*minimumRetainedProviderPercentage {
		return &errors.ValidationError{
			Field: "models_dev.providers", Value: candidateStats.providers,
			Message: fmt.Sprintf("promotion retains less than %d%% of the last-known-good provider count %d", minimumRetainedProviderPercentage, currentStats.providers),
		}
	}
	if candidateStats.models*100 < currentStats.models*minimumRetainedModelPercentage {
		return &errors.ValidationError{
			Field: "models_dev.models", Value: candidateStats.models,
			Message: fmt.Sprintf("promotion retains less than %d%% of the last-known-good model count %d", minimumRetainedModelPercentage, currentStats.models),
		}
	}
	return nil
}

func semanticPromotionIssue(message string) sources.ObservationIssue {
	return sources.ObservationIssue{
		Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodeSchemaDrift,
		Message: "upstream models.dev payload failed semantic promotion: " + message,
	}
}

func writeFileAtomically(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".starmap-cache-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()
	defer func() {
		_ = temp.Close()
	}()

	if err := temp.Chmod(constants.FilePermissions); err != nil {
		return err
	}
	if _, err := temp.Write(data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	return nil
}

func (c *HTTPClient) readCacheMetadata(apiPath string) (httpCacheMetadata, error) {
	data, err := os.ReadFile(apiPath + ".metadata.json") //nolint:gosec // Cache path is controlled by Starmap.
	if err != nil {
		return httpCacheMetadata{}, err
	}
	var metadata httpCacheMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return httpCacheMetadata{}, err
	}
	if metadata.Version != httpCacheMetadataVersion || metadata.ValidatedAt.IsZero() {
		return httpCacheMetadata{}, &errors.ValidationError{Field: "models_dev.cache_metadata", Message: "has unsupported version or missing validation time"}
	}
	if metadata.Origin != HTTPAcquisitionDownloaded && metadata.Origin != HTTPAcquisitionEmbeddedBootstrap {
		return httpCacheMetadata{}, &errors.ValidationError{Field: "models_dev.cache_metadata.origin", Value: metadata.Origin, Message: "is not supported"}
	}
	payload, err := os.ReadFile(apiPath) //nolint:gosec // Cache path is controlled by Starmap.
	if err != nil {
		return httpCacheMetadata{}, err
	}
	if metadata.ContentChecksum == "" || metadata.ContentChecksum != checksumBytes(payload) {
		return httpCacheMetadata{}, &errors.ValidationError{Field: "models_dev.cache_metadata.checksum", Value: metadata.ContentChecksum, Message: "does not match api.json"}
	}
	return metadata, nil
}

func (c *HTTPClient) writeCacheMetadata(apiPath string, metadata httpCacheMetadata) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return writeFileAtomically(apiPath+".metadata.json", data)
}

func (c *HTTPClient) now() time.Time {
	if c.nowFunc != nil {
		return c.nowFunc()
	}
	return time.Now()
}

func acquisitionResult(kind HTTPAcquisition, metadata httpCacheMetadata) HTTPAcquisitionResult {
	var revision sources.Revision
	if metadata.ETag != "" {
		revision = sources.Revision{Kind: sources.RevisionKindETag, Value: metadata.ETag}
	} else if metadata.LastModified != "" {
		revision = sources.Revision{Kind: sources.RevisionKindLastModified, Value: metadata.LastModified}
	}
	return HTTPAcquisitionResult{Kind: kind, Revision: revision}
}

func responseHeaderOr(resp *http.Response, name, fallback string) string {
	if value := strings.TrimSpace(resp.Header.Get(name)); value != "" {
		return value
	}
	return fallback
}

func checksumBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}
