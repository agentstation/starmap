package modelsdev

import (
	"context"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

// Package-level state for expensive git operations.
var (
	gitOnce sync.Once
	gitAPI  *API
	gitErr  error
)

// No init() - sources are created explicitly

// GitSource enhances models with models.dev data.
type GitSource struct {
	providers  *catalogs.Providers
	catalog    catalogs.Catalog
	sourcesDir string
}

// NewGitSource creates a new models.dev git source.
func NewGitSource(opts ...GitSourceOption) *GitSource {
	s := &GitSource{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// GitSourceOption configures a GitSource.
type GitSourceOption func(*GitSource)

// WithSourcesDir configures the sources directory for the git source.
func WithSourcesDir(dir string) GitSourceOption {
	return func(s *GitSource) {
		s.sourcesDir = dir
	}
}

// WithGitSourcesDir is an alias for WithSourcesDir for backward compatibility.
func WithGitSourcesDir(dir string) GitSourceOption {
	return WithSourcesDir(dir)
}

// ID returns the ID of this source.
func (s *GitSource) ID() sources.ID {
	return sources.ModelsDevGitID
}

// ensureGitRepo initializes models.dev data once using sync.Once.
func ensureGitRepo(ctx context.Context, outputDir string) (*API, error) {
	gitOnce.Do(func() {
		if outputDir == "" {
			outputDir = expandPath(constants.DefaultSourcesPath)
		}

		client := NewClient(outputDir)
		if err := client.EnsureRepository(ctx); err != nil {
			gitErr = err
			return
		}
		if err := client.BuildAPI(ctx); err != nil {
			gitErr = err
			return
		}
		gitAPI, gitErr = ParseAPI(client.GetAPIPath())
	})

	// If the directory changed, we need a new sync.Once but that's rare
	// For now, just use what we have
	return gitAPI, gitErr
}

// Setup initializes the source with dependencies.
func (s *GitSource) Setup(providers *catalogs.Providers) error {
	s.providers = providers
	return nil
}

// Fetch creates a catalog with models that have pricing/limits data from models.dev.
func (s *GitSource) Fetch(ctx context.Context, opts ...sources.Option) error {
	// Create a new catalog to build into
	var err error
	s.catalog, err = catalogs.New()
	if err != nil {
		return errors.WrapResource("create", "memory catalog", "", err)
	}

	// Use configured sources directory or default
	outputDir := s.sourcesDir
	if outputDir == "" {
		outputDir = expandPath(constants.DefaultSourcesPath)
	}

	// Initialize models.dev data once
	api, err := ensureGitRepo(ctx, outputDir)
	if err != nil {
		return errors.WrapResource("initialize", "models.dev", "", err)
	}

	// Process the API data using shared logic
	added, err := processFetch(s.catalog, api)
	if err != nil {
		return err
	}

	logging.Info().
		Int("model_count", added).
		Msg("Found models with pricing/limits from models.dev Git")
	return nil
}

// Catalog returns the catalog of this source.
func (s *GitSource) Catalog() catalogs.Catalog {
	return s.catalog
}

// Cleanup releases any resources.
func (s *GitSource) Cleanup() error {
	// GitSource doesn't hold persistent resources
	return nil
}
