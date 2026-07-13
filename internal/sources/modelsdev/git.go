package modelsdev

import (
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

// No init() - sources are created explicitly

// GitSource enhances models with models.dev data.
type GitSource struct {
	providers  *catalogs.Providers
	sourcesDir string
	commit     string
	loadAPI    func(context.Context, string) (*API, error)
}

var _ sources.Source = (*GitSource)(nil)

// NewGitSource creates a new models.dev git source.
func NewGitSource(opts ...GitSourceOption) *GitSource {
	s := &GitSource{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithGitCommit pins Git verification to one exact commit.
func WithGitCommit(commit string) GitSourceOption {
	return func(s *GitSource) {
		s.commit = commit
	}
}

// GitSourceOption configures a GitSource.
type GitSourceOption func(*GitSource)

// WithSourcesDir configures the sources directory for the git source.
func WithSourcesDir(dir string) GitSourceOption {
	return func(s *GitSource) {
		s.sourcesDir = dir
	}
}

// ID returns the ID of this source.
func (s *GitSource) ID() sources.ID {
	return sources.ModelsDevGitID
}

// Name returns the human-friendly name of this source.
func (s *GitSource) Name() string { return "models.dev (Git)" }

// ensureGitRepo loads models.dev data for this call and configured directory.
func ensureGitRepo(ctx context.Context, outputDir, commit string) (*API, sources.Revision, error) {
	if outputDir == "" {
		outputDir = expandPath(constants.DefaultSourcesPath)
	}
	client := NewPinnedGitClient(outputDir, commit)
	inputs, err := client.PrepareRepository(ctx)
	if err != nil {
		return nil, sources.Revision{}, err
	}
	if err := client.BuildAPI(ctx); err != nil {
		return nil, sources.Revision{}, err
	}
	api, err := ParseAPI(client.GetAPIPath())
	if err != nil {
		return nil, sources.Revision{}, err
	}
	if err := validateAPIPromotion(api, nil); err != nil {
		return nil, sources.Revision{}, errors.WrapResource("validate", "models.dev Git build semantics", inputs.Commit, err)
	}
	return api, revisionForGitInputs(inputs), nil
}

func revisionForGitInputs(inputs GitInputs) sources.Revision {
	return sources.Revision{
		Kind: sources.RevisionKindGitCommit, Value: inputs.Commit,
		InputName: inputs.LockfilePath, InputChecksum: inputs.LockfileChecksum,
	}
}

// Setup initializes the source with dependencies.
func (s *GitSource) Setup(providers *catalogs.Providers) error {
	s.providers = providers
	return nil
}

// Observe returns a catalog with mapped models.dev data directly.
func (s *GitSource) Observe(ctx context.Context, opts ...sources.Option) (sources.Observation, error) {
	ctx = logging.WithSource(ctx, s.ID().String())
	// Create a new catalog to build into
	builder := catalogs.NewEmpty()

	// Use configured sources directory or default
	outputDir := s.sourcesDir
	if outputDir == "" {
		outputDir = expandPath(constants.DefaultSourcesPath)
	}

	// Initialize models.dev data once
	revision := sources.Revision{Kind: sources.RevisionKindContentDigest}
	var api *API
	var err error
	if s.loadAPI != nil {
		api, err = s.loadAPI(ctx, outputDir)
	} else {
		api, revision, err = ensureGitRepo(ctx, outputDir, s.commit)
	}
	if err != nil {
		return sources.Observation{}, errors.WrapResource("initialize", "models.dev", "", err)
	}

	// Process the API data using shared logic
	added, rejected, recordIssues, err := processFetch(builder, api, opts...)
	if err != nil {
		return sources.Observation{}, err
	}

	catalog, err := builder.Build()
	if err != nil {
		return sources.Observation{}, errors.WrapResource("publish", "models.dev Git observation", "", err)
	}

	logging.FromContext(ctx).Info().
		Int("model_count", added).
		Msg("Found models with catalog data from models.dev Git")
	metadata := sources.ObservationMetadata{
		ObservedAt:   time.Now().UTC(),
		Revision:     revision,
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
		Records:      sources.ObservationRecordCounts{Accepted: added, Rejected: rejected},
	}
	if len(recordIssues) > 0 {
		metadata.Completeness = sources.ObservationCompletenessPartial
		metadata.Status = sources.ObservationStatusDegraded
		metadata.Issues = recordIssues
	}
	return sources.NewObservation(s.ID(), catalog, metadata)
}

// Cleanup releases any resources.
func (s *GitSource) Cleanup() error {
	// GitSource doesn't hold persistent resources
	return nil
}

// Dependencies returns the list of external dependencies required by this source.
// Git source requires bun (for building) and git (for cloning).
func (s *GitSource) Dependencies() []sources.Dependency {
	return []sources.Dependency{
		{
			Name:          "bun",
			DisplayName:   "Bun JavaScript runtime",
			Required:      false, // HTTP fallback exists
			CheckCommands: []string{"bun"},
			MinVersion:    "1.0.0",

			InstallURL:         "https://bun.sh/docs/installation",
			AutoInstallCommand: "curl -fsSL https://bun.sh/install | bash",

			Description:       "Fast JavaScript runtime for building models.dev data",
			WhyNeeded:         "Builds api.json from models.dev TypeScript source",
			AlternativeSource: "models_dev_http provides same data without dependencies",
		},
		{
			Name:          "git",
			DisplayName:   "Git version control",
			Required:      false, // HTTP fallback exists
			CheckCommands: []string{"git"},
			MinVersion:    "2.0.0",

			InstallURL: "https://git-scm.com/downloads",

			Description:       "Version control system for cloning models.dev repository",
			WhyNeeded:         "Clones models.dev repository to build local data",
			AlternativeSource: "models_dev_http provides same data without dependencies",
		},
	}
}

// IsOptional returns whether this source is optional.
// Git source is optional - HTTP source provides the same data without dependencies.
func (s *GitSource) IsOptional() bool {
	return true
}
