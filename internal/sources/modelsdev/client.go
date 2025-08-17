package modelsdev

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	ModelsDevRepoURL = "https://github.com/sst/models.dev.git"
	DefaultBranch    = "dev"
)

// Client handles models.dev repository operations
type Client struct {
	RepoPath string
}

// NewClient creates a new models.dev client
func NewClient(outputDir string) *Client {
	repoPath := filepath.Join(outputDir, "models.dev")
	return &Client{
		RepoPath: repoPath,
	}
}

// EnsureRepository ensures the models.dev repository is available and up to date
func (c *Client) EnsureRepository() error {
	var needsInstall bool

	if c.repositoryExists() {
		fmt.Printf("  🔄 Updating models.dev repository...\n")
		if err := c.updateRepository(); err != nil {
			return err
		}
		fmt.Printf("  ✅ Repository updated successfully\n")
		needsInstall = true // Always install after pull to ensure deps are current
	} else {
		fmt.Printf("  📥 Cloning models.dev repository...\n")
		if err := c.cloneRepository(); err != nil {
			return err
		}
		fmt.Printf("  ✅ Repository cloned successfully\n")
		needsInstall = true // Always install after clone
	}

	if needsInstall {
		fmt.Printf("  📦 Installing dependencies...\n")
		if err := c.installDependencies(); err != nil {
			return err
		}
		fmt.Printf("  ✅ Dependencies installed successfully\n")
	}

	return nil
}

// BuildAPI runs the build process to generate api.json
func (c *Client) BuildAPI() error {
	if !c.repositoryExists() {
		return fmt.Errorf("models.dev repository not found at %s", c.RepoPath)
	}

	fmt.Printf("  🔨 Building api.json (this may take a moment)...\n")

	// Change to repo directory and run build
	cmd := exec.Command("bun", "run", "script/build.ts")
	cmd.Dir = filepath.Join(c.RepoPath, "packages", "web")

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("  ❌ Build failed\n")
		return fmt.Errorf("building api.json: %w\nOutput: %s", err, output)
	}

	// Verify api.json was created
	apiPath := filepath.Join(c.RepoPath, "packages", "web", "dist", "_api.json")
	if _, err := os.Stat(apiPath); os.IsNotExist(err) {
		fmt.Printf("  ❌ Build completed but api.json not found\n")
		return fmt.Errorf("api.json not generated at expected path: %s", apiPath)
	}

	fmt.Printf("  ✅ API build completed successfully\n")
	return nil
}

// installDependencies runs bun install in the repository root
func (c *Client) installDependencies() error {
	cmd := exec.Command("bun", "install")
	cmd.Dir = c.RepoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("  ❌ Dependency installation failed\n")
		return fmt.Errorf("installing dependencies: %w\nOutput: %s", err, output)
	}

	return nil
}

// GetAPIPath returns the path to the generated api.json file
func (c *Client) GetAPIPath() string {
	return filepath.Join(c.RepoPath, "packages", "web", "dist", "_api.json")
}

// GetProvidersPath returns the path to the providers directory
func (c *Client) GetProvidersPath() string {
	return filepath.Join(c.RepoPath, "providers")
}

// Cleanup removes the models.dev repository
func (c *Client) Cleanup() error {
	if !c.repositoryExists() {
		return nil // Already cleaned up
	}
	return os.RemoveAll(c.RepoPath)
}

// repositoryExists checks if the models.dev repository exists
func (c *Client) repositoryExists() bool {
	gitDir := filepath.Join(c.RepoPath, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

// cloneRepository clones the models.dev repository
func (c *Client) cloneRepository() error {
	// Create parent directory if it doesn't exist
	parentDir := filepath.Dir(c.RepoPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}

	cmd := exec.Command("git", "clone", "--branch", DefaultBranch, "--depth", "1", ModelsDevRepoURL, c.RepoPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("  ❌ Clone failed\n")
		return fmt.Errorf("cloning models.dev repository: %w\nOutput: %s", err, output)
	}

	return nil
}

// updateRepository updates the existing models.dev repository
func (c *Client) updateRepository() error {
	// Reset any local changes
	resetCmd := exec.Command("git", "reset", "--hard", "HEAD")
	resetCmd.Dir = c.RepoPath
	if output, err := resetCmd.CombinedOutput(); err != nil {
		fmt.Printf("  ❌ Reset failed\n")
		return fmt.Errorf("resetting repository: %w\nOutput: %s", err, output)
	}

	// Pull latest changes
	pullCmd := exec.Command("git", "pull", "origin", DefaultBranch)
	pullCmd.Dir = c.RepoPath
	if output, err := pullCmd.CombinedOutput(); err != nil {
		fmt.Printf("  ❌ Pull failed\n")
		return fmt.Errorf("pulling latest changes: %w\nOutput: %s", err, output)
	}

	return nil
}
