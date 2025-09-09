package modelsdev

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// ModelsDevRepoURL is the URL for the models.dev git repository.
	ModelsDevRepoURL = "https://github.com/sst/models.dev.git"
	// DefaultBranch is the default branch to use for models.dev.
	DefaultBranch = "dev"
)

// GitClient handles models.dev repository operations.
type GitClient struct {
	RepoPath string
}

// Client is an alias for backward compatibility.
type Client = GitClient

// NewClient creates a new models.dev git client.
func NewClient(outputDir string) *Client {
	if outputDir == "" {
		outputDir = expandPath(constants.DefaultSourcesPath)
	}
	repoPath := filepath.Join(outputDir, "models.dev")
	return &Client{
		RepoPath: repoPath,
	}
}

// NewGitClient creates a new models.dev git client.
func NewGitClient(outputDir string) *GitClient {
	if outputDir == "" {
		outputDir = expandPath(constants.DefaultSourcesPath)
	}
	repoPath := filepath.Join(outputDir, "models.dev")
	return &GitClient{
		RepoPath: repoPath,
	}
}

// EnsureRepository ensures the models.dev repository is available and up to date.
func (c *GitClient) EnsureRepository(ctx context.Context) error {
	var needsInstall bool

	if c.repositoryExists() {
		fmt.Printf("  🔄 Updating models.dev repository...\n")
		if err := c.updateRepository(ctx); err != nil {
			return err
		}
		fmt.Printf("  ✅ Repository updated successfully\n")
		needsInstall = true // Always install after pull to ensure deps are current
	} else {
		fmt.Printf("  📥 Cloning models.dev repository...\n")
		if err := c.cloneRepository(ctx); err != nil {
			return err
		}
		fmt.Printf("  ✅ Repository cloned successfully\n")
		needsInstall = true // Always install after clone
	}

	if needsInstall {
		fmt.Printf("  📦 Installing dependencies...\n")
		if err := c.installDependencies(ctx); err != nil {
			return err
		}
		fmt.Printf("  ✅ Dependencies installed successfully\n")
	}

	return nil
}

// BuildAPI runs the build process to generate api.json.
func (c *GitClient) BuildAPI(ctx context.Context) error {
	if !c.repositoryExists() {
		return &errors.NotFoundError{
			Resource: "repository",
			ID:       c.RepoPath,
		}
	}

	fmt.Printf("  🔨 Building api.json (this may take a moment)...\n")

	// Change to repo directory and run build
	cmd := exec.CommandContext(ctx, "bun", "run", "script/build.ts")
	cmd.Dir = filepath.Join(c.RepoPath, "packages", "web")

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("  ❌ Build failed\n")
		return &errors.ProcessError{
			Operation: "build api.json",
			Command:   "npm run build",
			Output:    string(output),
			Err:       err,
		}
	}

	// Verify api.json was created
	apiPath := filepath.Join(c.RepoPath, "packages", "web", "dist", "_api.json")
	if _, err := os.Stat(apiPath); os.IsNotExist(err) {
		fmt.Printf("  ❌ Build completed but api.json not found\n")
		return &errors.NotFoundError{
			Resource: "file",
			ID:       apiPath,
		}
	}

	fmt.Printf("  ✅ API build completed successfully\n")
	return nil
}

// installDependencies runs bun install in the repository root.
func (c *GitClient) installDependencies(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "bun", "install")
	cmd.Dir = c.RepoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("  ❌ Dependency installation failed\n")
		return &errors.ProcessError{
			Operation: "install dependencies",
			Command:   "npm install",
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

	cmd := exec.CommandContext(ctx, "git", "clone", "--branch", DefaultBranch, "--depth", "1", ModelsDevRepoURL, c.RepoPath) //nolint:gosec // All parameters are controlled
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("  ❌ Clone failed\n")
		return &errors.ProcessError{
			Operation: "clone repository",
			Command:   "git clone",
			Output:    string(output),
			Err:       err,
		}
	}

	return nil
}

// updateRepository updates the existing models.dev repository.
func (c *GitClient) updateRepository(ctx context.Context) error {
	// Reset any local changes
	resetCmd := exec.CommandContext(ctx, "git", "reset", "--hard", "HEAD")
	resetCmd.Dir = c.RepoPath
	if output, err := resetCmd.CombinedOutput(); err != nil {
		fmt.Printf("  ❌ Reset failed\n")
		return &errors.ProcessError{
			Operation: "reset repository",
			Command:   "git reset",
			Output:    string(output),
			Err:       err,
		}
	}

	// Pull latest changes
	pullCmd := exec.CommandContext(ctx, "git", "pull", "origin", DefaultBranch)
	pullCmd.Dir = c.RepoPath
	if output, err := pullCmd.CombinedOutput(); err != nil {
		fmt.Printf("  ❌ Pull failed\n")
		return &errors.ProcessError{
			Operation: "pull latest changes",
			Command:   "git pull",
			Output:    string(output),
			Err:       err,
		}
	}

	return nil
}
