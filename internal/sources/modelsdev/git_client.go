package modelsdev

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

const (
	// ModelsDevRepoURL is the URL for the models.dev git repository.
	ModelsDevRepoURL = "https://github.com/sst/models.dev.git"
	lockfileName     = "bun.lock"
)

// GitClient handles models.dev repository operations.
type GitClient struct {
	RepoPath string
	RepoURL  string
	Commit   string
}

// GitInputs records the exact source and dependency graph used for a build.
type GitInputs struct {
	Commit           string
	LockfilePath     string
	LockfileChecksum string
}

// Client is an alias for backward compatibility.
type Client = GitClient

// NewClient creates a new models.dev git client.
func NewClient(outputDir string) *Client {
	if outputDir == "" {
		outputDir = expandPath(constants.DefaultSourcesPath)
	}
	repoPath := filepath.Join(outputDir, "models.dev-git")
	return &Client{
		RepoPath: repoPath,
		RepoURL:  ModelsDevRepoURL,
	}
}

// NewGitClient creates a new models.dev git client.
func NewGitClient(outputDir string) *GitClient {
	if outputDir == "" {
		outputDir = expandPath(constants.DefaultSourcesPath)
	}
	repoPath := filepath.Join(outputDir, "models.dev-git")
	return &GitClient{
		RepoPath: repoPath,
		RepoURL:  ModelsDevRepoURL,
	}
}

// NewPinnedGitClient creates a Git client that checks out one exact commit.
func NewPinnedGitClient(outputDir, commit string) *GitClient {
	client := NewGitClient(outputDir)
	client.Commit = commit
	return client
}

// EnsureRepository ensures the models.dev repository is available and up to date.
func (c *GitClient) EnsureRepository(ctx context.Context) error {
	_, err := c.PrepareRepository(ctx)
	return err
}

// PrepareRepository checks out the configured commit and verifies its frozen lockfile.
func (c *GitClient) PrepareRepository(ctx context.Context) (GitInputs, error) {
	ctx = logging.WithSource(ctx, sources.ModelsDevGitID.String())
	logger := logging.FromContext(ctx)
	if err := validateGitCommit(c.Commit); err != nil {
		return GitInputs{}, err
	}
	if !c.repositoryExists() {
		logger.Info().Str("repository", c.RepoURL).Msg("Cloning models.dev repository")
		if err := c.cloneRepository(ctx); err != nil {
			return GitInputs{}, err
		}
		logger.Info().Msg("Cloned models.dev repository")
	}

	logger.Info().Str("commit", c.Commit).Msg("Checking out pinned models.dev commit")
	if err := c.checkoutPinnedCommit(ctx); err != nil {
		return GitInputs{}, err
	}
	inputs, err := c.gitInputs()
	if err != nil {
		return GitInputs{}, err
	}
	logger.Info().Str("lockfile", inputs.LockfilePath).Msg("Installing models.dev dependencies from frozen lockfile")
	if err := c.installDependencies(ctx); err != nil {
		return GitInputs{}, err
	}
	verified, err := c.gitInputs()
	if err != nil {
		return GitInputs{}, err
	}
	if verified != inputs {
		return GitInputs{}, &errors.ValidationError{
			Field: "models_dev.git.lockfile", Value: verified.LockfileChecksum,
			Message: "changed during frozen dependency installation",
		}
	}
	return inputs, nil
}

// BuildAPI runs the build process to generate api.json.
func (c *GitClient) BuildAPI(ctx context.Context) error {
	ctx = logging.WithSource(ctx, sources.ModelsDevGitID.String())
	logger := logging.FromContext(ctx)
	if !c.repositoryExists() {
		return &errors.NotFoundError{
			Resource: "repository",
			ID:       c.RepoPath,
		}
	}

	logger.Info().Str("commit", c.Commit).Msg("Building models.dev API")

	// Change to repo directory and run build
	cmd := exec.CommandContext(ctx, "bun", "run", "script/build.ts")
	cmd.Dir = filepath.Join(c.RepoPath, "packages", "web")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error().Err(err).Msg("models.dev API build failed")
		return &errors.ProcessError{
			Operation: "build api.json",
			Command:   "bun run script/build.ts",
			Output:    string(output),
			Err:       err,
		}
	}

	// Verify api.json was created
	apiPath := filepath.Join(c.RepoPath, "packages", "web", "dist", "_api.json")
	if _, err := os.Stat(apiPath); os.IsNotExist(err) {
		logger.Error().Str("path", apiPath).Msg("models.dev API build output is missing")
		return &errors.NotFoundError{
			Resource: "file",
			ID:       apiPath,
		}
	}

	logger.Info().Str("path", apiPath).Msg("Built models.dev API")
	return nil
}

// installDependencies runs bun install in the repository root.
func (c *GitClient) installDependencies(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "bun", "install", "--frozen-lockfile")
	cmd.Dir = c.RepoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.FromContext(ctx).Error().Err(err).Msg("models.dev frozen dependency installation failed")
		return &errors.ProcessError{
			Operation: "install dependencies",
			Command:   "bun install --frozen-lockfile",
			Output:    string(output),
			Err:       err,
		}
	}

	return nil
}

// GetAPIPath returns the path to the generated api.json file.
func (c *GitClient) GetAPIPath() string {
	return filepath.Join(c.RepoPath, "packages", "web", "dist", "_api.json")
}

// GetProvidersPath returns the path to the providers directory.
func (c *GitClient) GetProvidersPath() string {
	return filepath.Join(c.RepoPath, "providers")
}

// Cleanup removes the models.dev repository.
func (c *GitClient) Cleanup() error {
	if !c.repositoryExists() {
		return nil // Already cleaned up
	}
	return os.RemoveAll(c.RepoPath)
}

// repositoryExists checks if the models.dev repository exists.
func (c *GitClient) repositoryExists() bool {
	gitDir := filepath.Join(c.RepoPath, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

// cloneRepository clones the models.dev repository.
func (c *GitClient) cloneRepository(ctx context.Context) error {
	// Create parent directory if it doesn't exist
	parentDir := filepath.Dir(c.RepoPath)
	if err := os.MkdirAll(parentDir, constants.DirPermissions); err != nil {
		return errors.WrapIO("create", "parent directory", err)
	}

	repoURL := c.RepoURL
	if repoURL == "" {
		repoURL = ModelsDevRepoURL
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--no-checkout", "--filter=blob:none", repoURL, c.RepoPath) //nolint:gosec // Repository URL and path are configured inputs.
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.FromContext(ctx).Error().Err(err).Msg("models.dev repository clone failed")
		return &errors.ProcessError{
			Operation: "clone repository",
			Command:   "git clone",
			Output:    string(output),
			Err:       err,
		}
	}

	return nil
}

func (c *GitClient) checkoutPinnedCommit(ctx context.Context) error {
	commands := [][]string{
		{"fetch", "--depth", "1", "origin", c.Commit},
		{"checkout", "--detach", "--force", c.Commit},
		{"reset", "--hard", c.Commit},
		{"clean", "-fdx"},
	}
	for _, args := range commands {
		cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // Exact git operation; commit is validated hexadecimal input.
		cmd.Dir = c.RepoPath
		if output, err := cmd.CombinedOutput(); err != nil {
			return &errors.ProcessError{
				Operation: "checkout pinned repository", Command: "git " + strings.Join(args, " "),
				Output: string(output), Err: err,
			}
		}
	}
	headCmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	headCmd.Dir = c.RepoPath
	output, err := headCmd.CombinedOutput()
	if err != nil {
		return &errors.ProcessError{Operation: "verify pinned repository", Command: "git rev-parse HEAD", Output: string(output), Err: err}
	}
	if !strings.EqualFold(strings.TrimSpace(string(output)), c.Commit) {
		return &errors.ValidationError{Field: "models_dev.git.commit", Value: strings.TrimSpace(string(output)), Message: "does not match requested commit"}
	}
	return nil
}

func (c *GitClient) gitInputs() (GitInputs, error) {
	path := filepath.Join(c.RepoPath, lockfileName)
	data, err := os.ReadFile(path) //nolint:gosec // Path is fixed under the checked-out repository.
	if err != nil {
		return GitInputs{}, errors.WrapResource("read", "models.dev lockfile", path, err)
	}
	digest := sha256.Sum256(data)
	return GitInputs{Commit: strings.ToLower(c.Commit), LockfilePath: lockfileName, LockfileChecksum: "sha256:" + hex.EncodeToString(digest[:])}, nil
}

func validateGitCommit(commit string) error {
	if len(commit) != 40 && len(commit) != 64 {
		return &errors.ValidationError{Field: "models_dev.git.commit", Value: commit, Message: "must be an exact 40- or 64-character hexadecimal commit"}
	}
	if _, err := hex.DecodeString(commit); err != nil {
		return &errors.ValidationError{Field: "models_dev.git.commit", Value: commit, Message: "must be hexadecimal"}
	}
	return nil
}
