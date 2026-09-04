package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// acceptJSON is the pinned GitHub REST media type.
	acceptJSON = "application/vnd.github+json"

	// acceptBinary asks for the asset bytes instead of the asset metadata.
	acceptBinary = "application/octet-stream"

	// acceptRaw asks the contents endpoint for the file bytes instead of the
	// file metadata.
	acceptRaw = "application/vnd.github.raw+json"

	// apiVersion pins the REST API version, so a later default cannot change
	// the reply shape under a running fleet.
	apiVersion = "2022-11-28"

	// maxRedirects bounds the redirect chain of one asset download. GitHub
	// answers an asset request with one redirect to object storage.
	maxRedirects = 5

	// digestPrefix names the digest algorithm in an attestation path.
	digestPrefix = "sha256:"

	// resourceRelease, resourceAsset, and resourceAttestation are the safe
	// operation labels that every error and progress report carries. They
	// name the operation and never the URL, the token, or the host.
	resourceRelease     = "release"
	resourceAsset       = "asset"
	resourceAttestation = "attestation"
	resourceChannel     = "channel"
)

// RefusalError reports a refused GitHub reply together with the hard
// not-before boundary the reply declared. A caller passes the boundary to
// fleet.NotBefore, which adds the jitter that keeps a fleet from retrying at
// one instant.
type RefusalError struct {
	// Status is the refused HTTP status code.
	Status int

	// Resource is the safe operation label.
	Resource string

	// NotBefore is the earliest time the caller may retry. It is zero when
	// the reply declared no boundary.
	NotBefore time.Time

	// Budget is the request budget the refusal reported.
	Budget RateLimitBudget

	// Err is the underlying typed API error.
	Err error
}

// Error implements the error interface.
func (e *RefusalError) Error() string {
	if e.NotBefore.IsZero() {
		return fmt.Sprintf("github %s request refused with status %d", e.Resource, e.Status)
	}
	return fmt.Sprintf("github %s request refused with status %d until %s",
		e.Resource, e.Status, e.NotBefore.Format(time.RFC3339))
}

// Unwrap implements errors.Unwrap, so a caller keeps the typed API error.
func (e *RefusalError) Unwrap() error { return e.Err }

// reply is one complete bounded GitHub REST reply.
type reply struct {
	remote.Reply
}

// etag returns the conditional-request validator the reply carried.
func (r reply) etag() string {
	return strings.TrimSpace(r.Header.Get("ETag"))
}

// releaseAsset is one published asset of a GitHub release.
type releaseAsset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

// releaseDocument is the subset of a GitHub release that this source reads.
type releaseDocument struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

// asset returns the named asset of the release.
func (d releaseDocument) asset(name string) (releaseAsset, error) {
	for _, candidate := range d.Assets {
		if candidate.Name == name {
			return candidate, nil
		}
	}
	return releaseAsset{}, errors.NewNotFoundError("catalog release asset", name)
}

// attestationEntry holds one provenance bundle that the repository stores.
type attestationEntry struct {
	Bundle json.RawMessage `json:"bundle"`
}

// attestationDocument is the reply of the repository attestation endpoint.
type attestationDocument struct {
	Attestations []attestationEntry `json:"attestations"`
}

// client holds the immutable request settings of one source. It carries no
// mutable state, so concurrent observations share one client safely.
type client struct {
	config   Config
	transfer remote.Transfer
}

// newClient builds the REST client of one configuration.
func newClient(config Config) (*client, error) {
	httpClient := config.HTTPClient
	if httpClient == nil {
		built, err := remote.NewTransferClient(config.TransferPolicy)
		if err != nil {
			return nil, err
		}
		httpClient = built
	}
	if httpClient.CheckRedirect == nil {
		// GitHub answers an asset request with a redirect to object storage.
		// Follow a bounded chain. The standard client drops the credential on
		// a cross-host hop, so the token never reaches object storage.
		httpClient.CheckRedirect = func(_ *http.Request, chain []*http.Request) error {
			if len(chain) >= maxRedirects {
				return sourceValidation("redirects", len(chain), "exceeds the redirect bound")
			}
			return nil
		}
	}
	return &client{
		config: config,
		transfer: remote.Transfer{
			Client:   httpClient,
			Policy:   config.TransferPolicy,
			Progress: config.Progress,
		},
	}, nil
}

// repositoryURL joins one repository path under the configured API root.
func (c *client) repositoryURL(segments ...string) string {
	parts := make([]string, 0, len(segments)+3)
	parts = append(parts, c.config.APIBaseURL, "repos", c.config.Repository)
	for _, segment := range segments {
		parts = append(parts, url.PathEscape(segment))
	}
	return strings.Join(parts, "/")
}

// cycle is the request accumulator of one refresh. Every request of one
// Observe call passes through one cycle, so the source reports an exact
// request count and the budget the last reply declared.
type cycle struct {
	client   *client
	requests int
	budget   RateLimitBudget
}

// newCycle starts one refresh cycle.
func (c *client) newCycle() *cycle {
	return &cycle{client: c}
}

// budgetResult returns the budget of the cycle with its request count.
func (c *cycle) budgetResult() RateLimitBudget {
	budget := c.budget
	budget.Requests = c.requests
	return budget
}

// releaseByTag reads one release. A non-empty validator makes the request
// conditional, so an unchanged channel costs one request and no body.
func (c *cycle) releaseByTag(ctx context.Context, tag, validator string) (reply, releaseDocument, error) {
	endpoint := c.client.repositoryURL("releases", "tags", tag)
	answer, err := c.get(ctx, endpoint, acceptJSON, validator, resourceRelease)
	if err != nil {
		return reply{}, releaseDocument{}, err
	}
	if answer.StatusCode == http.StatusNotModified {
		return answer, releaseDocument{}, nil
	}
	if err := c.checkStatus(answer, resourceRelease, tag); err != nil {
		return reply{}, releaseDocument{}, err
	}
	var document releaseDocument
	if err := json.Unmarshal(answer.Body, &document); err != nil {
		return reply{}, releaseDocument{}, errors.NewParseError(
			"json", resourceRelease, "cannot decode the release", err)
	}
	if document.TagName != tag {
		return reply{}, releaseDocument{}, sourceValidation(
			"release.tag", document.TagName, "does not match the requested tag")
	}
	return answer, document, nil
}

// channelFile reads one file from the channel branch through the repository
// contents endpoint. A non-empty validator makes the request conditional, so
// an unchanged channel costs one request and no body. The raw media type
// returns the file bytes, so one request replaces a metadata read and a
// separate download.
//
// The endpoint reads a branch ref, not a release. GitHub freezes an immutable
// release at creation, so a mutable pointer cannot live on one. The source
// never reads the raw.githubusercontent.com host, because that host caches a
// changed file for minutes.
func (c *cycle) channelFile(ctx context.Context, channel, path, validator string) (reply, []byte, error) {
	endpoint := c.client.repositoryURL("contents", path) +
		"?" + url.Values{"ref": []string{channel}}.Encode()
	answer, err := c.get(ctx, endpoint, acceptRaw, validator, resourceChannel)
	if err != nil {
		return reply{}, nil, err
	}
	if answer.StatusCode == http.StatusNotModified {
		return answer, nil, nil
	}
	if err := c.checkStatus(answer, resourceChannel, path); err != nil {
		return reply{}, nil, err
	}
	return answer, answer.Body, nil
}

// assetBytes downloads one release asset and checks its declared size.
func (c *cycle) assetBytes(ctx context.Context, asset releaseAsset) ([]byte, error) {
	if strings.TrimSpace(asset.URL) == "" {
		return nil, sourceValidation("asset.url", asset.Name, "is required")
	}
	answer, err := c.get(ctx, asset.URL, acceptBinary, "", resourceAsset)
	if err != nil {
		return nil, err
	}
	if err := c.checkStatus(answer, resourceAsset, asset.Name); err != nil {
		return nil, err
	}
	if int64(len(answer.Body)) != asset.Size {
		return nil, sourceValidation("asset.size_bytes", asset.Name,
			"does not match the size the release declares")
	}
	return answer.Body, nil
}

// bundles reads every provenance bundle that the repository holds for one
// artifact digest. GitHub may return more than one, so the caller tries each.
func (c *cycle) bundles(ctx context.Context, digest string) ([][]byte, error) {
	endpoint := c.client.repositoryURL("attestations", digestPrefix+digest)
	query := url.Values{"predicate_type": []string{c.client.config.policy().PredicateType}}
	answer, err := c.get(ctx, endpoint+"?"+query.Encode(), acceptJSON, "", resourceAttestation)
	if err != nil {
		return nil, err
	}
	if err := c.checkStatus(answer, resourceAttestation, digest); err != nil {
		return nil, err
	}
	var document attestationDocument
	if err := json.Unmarshal(answer.Body, &document); err != nil {
		return nil, errors.NewParseError(
			"json", resourceAttestation, "cannot decode the attestation list", err)
	}
	found := make([][]byte, 0, len(document.Attestations))
	for _, entry := range document.Attestations {
		if len(entry.Bundle) > 0 {
			found = append(found, append([]byte(nil), entry.Bundle...))
		}
	}
	if len(found) == 0 {
		return nil, errors.NewNotFoundError("catalog attestation bundle", digest)
	}
	return found, nil
}

// get sends one bounded request and returns the complete reply. It counts the
// request before it sends, so a failed request still spends its budget.
func (c *cycle) get(ctx context.Context, endpoint, accept, validator, resource string) (reply, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return reply{}, errors.WrapResource("build", "catalog source request", resource, err)
	}
	request.Header.Set("Accept", accept)
	request.Header.Set("X-GitHub-Api-Version", apiVersion)
	if c.client.config.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.client.config.Token)
	}
	if validator != "" {
		request.Header.Set("If-None-Match", validator)
	}
	c.requests++
	answer, err := c.client.transfer.Body(ctx, request, resource)
	if err != nil {
		return reply{}, err
	}
	if budget := parseRateLimit(answer.Header); budget.Observed {
		c.budget = budget
	}
	return reply{Reply: answer}, nil
}

// checkStatus converts a refused or failed reply into a typed error. The
// message names the safe operation label and never the configured URL.
func (c *cycle) checkStatus(answer reply, resource, subject string) error {
	switch {
	case answer.StatusCode >= http.StatusOK && answer.StatusCode < http.StatusMultipleChoices:
		return nil
	case answer.StatusCode == http.StatusNotFound:
		return errors.NewNotFoundError("catalog "+resource, subject)
	case answer.StatusCode == http.StatusForbidden || answer.StatusCode == http.StatusTooManyRequests:
		refusal := &RefusalError{
			Status:   answer.StatusCode,
			Resource: resource,
			Budget:   c.budgetResult(),
			Err: errors.NewAPIError(SourceIdentity, answer.StatusCode,
				"the catalog source refused the "+resource+" request"),
		}
		if boundary, found := retryBoundary(answer.Header, c.client.config.Now()); found {
			refusal.NotBefore = boundary
		}
		return refusal
	default:
		return errors.NewAPIError(SourceIdentity, answer.StatusCode,
			"the catalog source rejected the "+resource+" request")
	}
}
