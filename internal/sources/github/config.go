package github

import (
	"net/http"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/attestation"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// DefaultRepository is the public Starmap catalog repository.
	DefaultRepository = "agentstation/starmap"

	// DefaultChannel is the public discovery channel release.
	DefaultChannel = artifact.ChannelName

	// DefaultSignerWorkflow is the repository-relative path of the workflow
	// that signs every published catalog release.
	DefaultSignerWorkflow = ".github/workflows/catalog-generation.yaml"

	// DefaultAPIBaseURL is the public GitHub REST API root.
	DefaultAPIBaseURL = "https://api.github.com"

	// SourceIdentity is the safe name of this source. It carries no URL, no
	// token, and no host of a custom deployment.
	SourceIdentity = "public_github"
)

// Config holds the settings of one GitHub catalog source.
//
// Each field carries one canonical `CATALOG_SOURCE_*` setting. The package
// never reads the environment. A caller resolves the environment and passes
// the result through the options below.
type Config struct {
	// Repository is `CATALOG_SOURCE_REPOSITORY`, in `owner/name` form.
	Repository string

	// Channel is `CATALOG_SOURCE_CHANNEL`, the discovery release tag.
	Channel string

	// SignerWorkflow is `CATALOG_SOURCE_SIGNER_WORKFLOW`.
	SignerWorkflow string

	// Token is `CATALOG_SOURCE_TOKEN`. It is optional, because every public
	// catalog release is readable without one.
	Token string

	// APIBaseURL is `CATALOG_SOURCE_URL`, the REST API root.
	APIBaseURL string

	// StateDirectory holds the durable ETag, sequence, and rollback target.
	// The caller owns the directory and supplies it.
	StateDirectory string

	// TrustedRootJSON overrides the compiled Sigstore trusted root. A
	// connected caller refreshes the root through TUF and passes it here.
	TrustedRootJSON []byte

	// TransferPolicy bounds every request and every body read.
	TransferPolicy remote.TransferPolicy

	// Progress receives transfer progress. It may be nil.
	Progress remote.ProgressFunc

	// HTTPClient overrides the transfer client. It may be nil.
	HTTPClient *http.Client

	// Attester verifies one Sigstore bundle. It defaults to the hermetic
	// engine in internal/attestation.
	Attester Attester

	// Now reads the clock. It defaults to time.Now.
	Now func() time.Time
}

// Option configures one GitHub catalog source.
type Option func(*Config)

// WithRepository sets the `owner/name` repository that publishes the catalog.
func WithRepository(repository string) Option {
	return func(c *Config) { c.Repository = repository }
}

// WithChannel sets the discovery release tag.
func WithChannel(channel string) Option {
	return func(c *Config) { c.Channel = channel }
}

// WithSignerWorkflow sets the workflow path that the trust policy requires.
func WithSignerWorkflow(workflow string) Option {
	return func(c *Config) { c.SignerWorkflow = workflow }
}

// WithToken sets the optional GitHub token.
func WithToken(token string) Option {
	return func(c *Config) { c.Token = token }
}

// WithAPIBaseURL sets the REST API root.
func WithAPIBaseURL(baseURL string) Option {
	return func(c *Config) { c.APIBaseURL = baseURL }
}

// WithStateDirectory sets the directory that holds the durable source state.
func WithStateDirectory(directory string) Option {
	return func(c *Config) { c.StateDirectory = directory }
}

// WithTrustedRoot overrides the compiled Sigstore trusted root.
func WithTrustedRoot(trustedRootJSON []byte) Option {
	return func(c *Config) {
		c.TrustedRootJSON = append([]byte(nil), trustedRootJSON...)
	}
}

// WithTransferPolicy overrides the transfer bounds.
func WithTransferPolicy(policy remote.TransferPolicy) Option {
	return func(c *Config) { c.TransferPolicy = policy }
}

// WithProgress sets the transfer progress callback.
func WithProgress(progress remote.ProgressFunc) Option {
	return func(c *Config) { c.Progress = progress }
}

// WithHTTPClient overrides the HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) { c.HTTPClient = client }
}

// WithAttester overrides the Sigstore verification engine. A caller that
// refreshes its own engine build passes it here. The default engine needs no
// network and no override.
func WithAttester(attester Attester) Option {
	return func(c *Config) { c.Attester = attester }
}

// WithClock overrides the clock.
func WithClock(now func() time.Time) Option {
	return func(c *Config) { c.Now = now }
}

// defaultConfig returns the settings that apply before any option runs.
func defaultConfig() Config {
	return Config{
		Repository:      DefaultRepository,
		Channel:         DefaultChannel,
		SignerWorkflow:  DefaultSignerWorkflow,
		APIBaseURL:      DefaultAPIBaseURL,
		TrustedRootJSON: attestation.DefaultTrustedRootJSON(),
		TransferPolicy:  remote.DefaultTransferPolicy(),
		Attester:        attestation.Verify,
		Now:             time.Now,
	}
}

// normalize trims every setting and rejects an unusable configuration.
func (c *Config) normalize() error {
	c.Repository = strings.TrimSpace(c.Repository)
	c.Channel = strings.TrimSpace(c.Channel)
	c.SignerWorkflow = strings.TrimSpace(c.SignerWorkflow)
	c.Token = strings.TrimSpace(c.Token)
	c.APIBaseURL = strings.TrimRight(strings.TrimSpace(c.APIBaseURL), "/")
	c.StateDirectory = strings.TrimSpace(c.StateDirectory)

	owner, name, found := strings.Cut(c.Repository, "/")
	if !found || owner == "" || name == "" || strings.Contains(name, "/") {
		return sourceValidation("repository", c.Repository, "must name one owner and one repository")
	}
	if c.Channel == "" {
		return sourceValidation("channel", c.Channel, "is required")
	}
	if c.SignerWorkflow == "" {
		return sourceValidation("signer_workflow", c.SignerWorkflow, "is required")
	}
	if c.APIBaseURL == "" {
		return sourceValidation("url", c.APIBaseURL, "is required")
	}
	if c.StateDirectory == "" {
		return sourceValidation("state_directory", c.StateDirectory, "is required")
	}
	if len(c.TrustedRootJSON) == 0 {
		return sourceValidation("trusted_root", 0, "is required")
	}
	if c.Attester == nil {
		return sourceValidation("attester", nil, "is required")
	}
	if c.Now == nil {
		return sourceValidation("clock", nil, "is required")
	}
	return c.TransferPolicy.Validate()
}

// policy returns the attestation trust policy of this configuration.
func (c Config) policy() attestation.Policy {
	return attestation.Policy{
		Repository:            c.Repository,
		Workflow:              c.SignerWorkflow,
		Issuer:                attestation.GitHubOIDCIssuer,
		PredicateType:         attestation.BuildProvenancePredicateType,
		TrustedRootJSON:       c.TrustedRootJSON,
		DenySelfHostedRunners: true,
	}
}

func sourceValidation(field string, value any, message string) error {
	return &errors.ValidationError{
		Field:   "catalog_source." + field,
		Value:   value,
		Message: message,
	}
}
