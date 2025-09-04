// Package site provides Hugo-based static site generation for Starmap documentation
package site

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	// Default configuration values
	defaultSiteDir    = "./site"
	defaultContentDir = "./docs"
	defaultBaseURL    = "https://starmap.agentstation.ai/"
	defaultTheme      = "hugo-book"

	// Hugo command and arguments
	hugoCmd             = "hugo"
	hugoVersionArg      = "version"
	hugoServerArg       = "server"
	hugoSourceFlag      = "--source"
	hugoPortFlag        = "--port"
	hugoBuildDraftsFlag = "--buildDrafts"
	hugoNavigateFlag    = "--navigateToChanged"
	hugoFastRenderFlag  = "--disableFastRender"
	hugoGCFlag          = "--gc"
	hugoMinifyFlag      = "--minify"
	hugoBaseURLFlag     = "--baseURL"

	// File patterns and identifiers
	markdownExt       = ".md"
	frontMatterPrefix = "---"
	modelsPathSegment = "/models/"
	readmeFileName    = "README"
	hugoConfigFile    = "hugo.yaml"
	publicDirName     = "public"

	// Weight constants
	defaultWeight = 10
	readmeWeight  = 1

	// String transformations
	dashChar  = "-"
	spaceChar = " "

	// Hugo dependency info
	hugoDependencyName = "hugo"
	hugoInstallHelpMsg = "Hugo not found. Install with: brew install hugo or use devbox shell"

	// Platform-specific installation commands
	brewInstallCmd = "brew"
	brewInstallArg = "install"
	aptInstallCmd  = "apt"
	aptInstallArg  = "install"
	snapInstallCmd = "snap"
	snapInstallArg = "install"

	// User input prompts
	installPrompt    = "Would you like to install Hugo now? (y/N): "
	confirmYes       = "y"
	confirmYesUpper  = "Y"
	osDetectionError = "Unable to detect operating system for automatic installation"

	// Front matter templates
	basicFrontMatterTmpl = `---
title: "%s"
weight: %d
---`

	modelFrontMatterTmpl = `---
title: "%s"
description: "%s"
weight: %d
author: "%s"
---`
)

// Site represents a Hugo-based documentation website
type Site struct {
	rootDir    string
	contentDir string
	hugoConfig string
	baseURL    string
}

// Config holds website configuration
type Config struct {
	RootDir    string // Root directory for the site (default: ./site)
	ContentDir string // Content directory (default: ./docs)
	BaseURL    string // Base URL for the site
	Theme      string // Hugo theme to use (default: hugo-book)
}

// New creates a new Site instance
func New(config *Config) (*Site, error) {
	if config == nil {
		config = &Config{}
	}

	// Set defaults
	if config.RootDir == "" {
		config.RootDir = defaultSiteDir
	}
	if config.ContentDir == "" {
		config.ContentDir = defaultContentDir
	}
	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}
	if config.Theme == "" {
		config.Theme = defaultTheme
	}

	site := &Site{
		rootDir:    config.RootDir,
		contentDir: config.ContentDir,
		baseURL:    config.BaseURL,
		hugoConfig: filepath.Join(config.RootDir, hugoConfigFile),
	}

	return site, nil
}

// Generate builds the static site from the current catalog
func (s *Site) Generate(ctx context.Context, catalog catalogs.Reader) error {
	logging.Info().
		Str("root_dir", s.rootDir).
		Str("content_dir", s.contentDir).
		Msg("Generating static site")

	// Ensure Hugo is available
	if err := s.checkHugo(); err != nil {
		return fmt.Errorf("hugo not available: %w", err)
	}

	// Generate front matter for existing markdown files
	if catalog != nil {
		if err := s.addFrontMatter(catalog); err != nil {
			return fmt.Errorf("adding front matter: %w", err)
		}
	}

	// Build the site
	if err := s.build(ctx); err != nil {
		return fmt.Errorf("building site: %w", err)
	}

	logging.Info().
		Str("output_dir", filepath.Join(s.rootDir, publicDirName)).
		Msg("Site generated successfully")

	return nil
}

// Serve starts the Hugo development server
func (s *Site) Serve(ctx context.Context, port int) error {
	logging.Info().
		Int("port", port).
		Msg("Starting Hugo development server")

	cmd := exec.CommandContext(ctx, hugoCmd, hugoServerArg,
		hugoSourceFlag, s.rootDir,
		hugoPortFlag, fmt.Sprintf("%d", port),
		hugoBuildDraftsFlag,
		hugoNavigateFlag,
		hugoFastRenderFlag,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// checkHugo verifies Hugo is installed and available, offers to install if missing
func (s *Site) checkHugo() error {
	cmd := exec.Command(hugoCmd, hugoVersionArg)
	output, err := cmd.Output()
	if err != nil {
		// Hugo not found, offer to install
		logging.Warn().Msg("Hugo not found on system")

		if err := s.offerHugoInstallation(); err != nil {
			return &errors.DependencyError{
				Dependency: hugoDependencyName,
				Message:    hugoInstallHelpMsg,
			}
		}

		// Verify installation succeeded
		cmd = exec.Command(hugoCmd, hugoVersionArg)
		output, err = cmd.Output()
		if err != nil {
			return &errors.DependencyError{
				Dependency: hugoDependencyName,
				Message:    "Hugo installation failed or not found in PATH",
			}
		}
	}

	logging.Debug().
		Str("version", string(output)).
		Msg("Hugo version")

	return nil
}

// build runs Hugo to build the static site
func (s *Site) build(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, hugoCmd,
		hugoSourceFlag, s.rootDir,
		hugoGCFlag,
		hugoMinifyFlag,
		hugoBaseURLFlag, s.baseURL,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.Error().
			Err(err).
			Str("output", string(output)).
			Msg("Hugo build failed")
		return fmt.Errorf("hugo build: %w", err)
	}

	logging.Debug().
		Str("output", string(output)).
		Msg("Hugo build output")

	return nil
}

// addFrontMatter adds Hugo front matter to markdown files
func (s *Site) addFrontMatter(catalog catalogs.Reader) error {
	// Walk through content directory
	err := filepath.Walk(s.contentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip if not a file
		if info.IsDir() {
			return nil
		}

		// Process only markdown files
		if !strings.HasSuffix(path, markdownExt) {
			return nil
		}

		// Check if file already has front matter
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		// Skip if already has front matter
		if strings.HasPrefix(string(content), frontMatterPrefix) {
			return nil
		}

		// Generate front matter based on file location
		frontMatter := s.generateFrontMatter(path, catalog)

		// Write updated content
		updatedContent := fmt.Sprintf("%s\n%s", frontMatter, string(content))
		if err := os.WriteFile(path, []byte(updatedContent), constants.FilePermissions); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}

		logging.Debug().
			Str("file", path).
			Msg("Added front matter")

		return nil
	})

	return err
}

// generateFrontMatter creates appropriate front matter for a file
func (s *Site) generateFrontMatter(path string, catalog catalogs.Reader) string {
	// Extract title from path
	title := filepath.Base(path)
	title = strings.TrimSuffix(title, markdownExt)
	title = strings.ReplaceAll(title, dashChar, spaceChar)
	title = cases.Title(language.English).String(title)

	// Determine weight based on path
	weight := defaultWeight
	if strings.Contains(path, readmeFileName) {
		weight = readmeWeight
	}

	// Basic front matter
	frontMatter := fmt.Sprintf(basicFrontMatterTmpl, title, weight)

	// Add model-specific metadata if this is a model file
	if strings.Contains(path, modelsPathSegment) && catalog != nil {
		modelID := strings.TrimSuffix(filepath.Base(path), markdownExt)
		model, err := catalog.FindModel(modelID)
		if err == nil {
			description := model.Description
			authorNames := ""
			if len(model.Authors) > 0 {
				authorNames = model.Authors[0].Name
			}

			frontMatter = fmt.Sprintf(modelFrontMatterTmpl, model.Name, description, weight, authorNames)
		}
	}

	return frontMatter
}

// offerHugoInstallation prompts user to install Hugo and attempts installation
func (s *Site) offerHugoInstallation() error {
	fmt.Print(installPrompt)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading user input: %w", err)
	}

	response = strings.TrimSpace(response)
	if response != confirmYes && response != confirmYesUpper {
		return fmt.Errorf("user declined Hugo installation")
	}

	logging.Info().Msg("Attempting to install Hugo...")
	return s.installHugo()
}

// installHugo attempts to install Hugo based on the detected platform
func (s *Site) installHugo() error {
	switch runtime.GOOS {
	case "darwin":
		return s.installHugoMacOS()
	case "linux":
		return s.installHugoLinux()
	case "windows":
		return s.installHugoWindows()
	default:
		return fmt.Errorf(osDetectionError)
	}
}

// installHugoMacOS installs Hugo on macOS using brew
func (s *Site) installHugoMacOS() error {
	// Check if brew is available
	if !s.isCommandAvailable(brewInstallCmd) {
		return fmt.Errorf("brew not found. Please install Homebrew first: https://brew.sh")
	}

	logging.Info().Msg("Installing Hugo using Homebrew...")
	cmd := exec.Command(brewInstallCmd, brewInstallArg, hugoDependencyName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Hugo with brew: %w", err)
	}

	logging.Info().Msg("Hugo installed successfully via Homebrew")
	return nil
}

// installHugoLinux attempts to install Hugo on Linux using available package managers
func (s *Site) installHugoLinux() error {
	// Try snap first (more likely to have recent version)
	if s.isCommandAvailable(snapInstallCmd) {
		logging.Info().Msg("Installing Hugo using snap...")
		cmd := exec.Command("sudo", snapInstallCmd, snapInstallArg, hugoDependencyName, "--channel=extended")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err == nil {
			logging.Info().Msg("Hugo installed successfully via snap")
			return nil
		}
		logging.Warn().Msg("Snap installation failed, trying apt...")
	}

	// Fallback to apt
	if s.isCommandAvailable(aptInstallCmd) {
		logging.Info().Msg("Installing Hugo using apt...")

		// Update package list first
		updateCmd := exec.Command("sudo", aptInstallCmd, "update")
		updateCmd.Stdout = os.Stdout
		updateCmd.Stderr = os.Stderr
		if err := updateCmd.Run(); err != nil {
			logging.Warn().Err(err).Msg("Failed to update package list")
		}

		// Install Hugo
		cmd := exec.Command("sudo", aptInstallCmd, aptInstallArg, "-y", hugoDependencyName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install Hugo with apt: %w", err)
		}

		logging.Info().Msg("Hugo installed successfully via apt")
		return nil
	}

	return fmt.Errorf("no supported package manager found (tried snap, apt)")
}

// installHugoWindows provides guidance for Windows installation
func (s *Site) installHugoWindows() error {
	// Windows installation is more complex, provide guidance
	logging.Info().Msg("Automatic installation not supported on Windows")
	logging.Info().Msg("Please install Hugo manually:")
	logging.Info().Msg("1. Using Chocolatey: choco install hugo-extended")
	logging.Info().Msg("2. Using Scoop: scoop install hugo-extended")
	logging.Info().Msg("3. Download from: https://github.com/gohugoio/hugo/releases")

	return fmt.Errorf("manual installation required on Windows")
}

// isCommandAvailable checks if a command is available in PATH
func (s *Site) isCommandAvailable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}
