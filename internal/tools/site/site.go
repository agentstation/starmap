// Package site provides Hugo-based static site generation for Starmap documentation
package site

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
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
		config.RootDir = "./site"
	}
	if config.ContentDir == "" {
		config.ContentDir = "./docs"
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://starmap.agentstation.ai/"
	}
	if config.Theme == "" {
		config.Theme = "hugo-book"
	}

	site := &Site{
		rootDir:    config.RootDir,
		contentDir: config.ContentDir,
		baseURL:    config.BaseURL,
		hugoConfig: filepath.Join(config.RootDir, "hugo.yaml"),
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
		if err := s.addFrontMatter(ctx, catalog); err != nil {
			return fmt.Errorf("adding front matter: %w", err)
		}
	}

	// Build the site
	if err := s.build(ctx); err != nil {
		return fmt.Errorf("building site: %w", err)
	}

	logging.Info().
		Str("output_dir", filepath.Join(s.rootDir, "public")).
		Msg("Site generated successfully")

	return nil
}

// Serve starts the Hugo development server
func (s *Site) Serve(ctx context.Context, port int) error {
	logging.Info().
		Int("port", port).
		Msg("Starting Hugo development server")

	cmd := exec.CommandContext(ctx, "hugo", "server",
		"--source", s.rootDir,
		"--port", fmt.Sprintf("%d", port),
		"--buildDrafts",
		"--navigateToChanged",
		"--disableFastRender",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Deploy deploys the site to GitHub Pages
func (s *Site) Deploy(ctx context.Context) error {
	logging.Info().Msg("Deploying site to GitHub Pages")

	// This would typically trigger the GitHub Actions workflow
	// or use gh-pages branch deployment
	return fmt.Errorf("deploy not implemented: use GitHub Actions workflow")
}

// checkHugo verifies Hugo is installed and available
func (s *Site) checkHugo() error {
	cmd := exec.Command("hugo", "version")
	output, err := cmd.Output()
	if err != nil {
		return &errors.DependencyError{
			Dependency: "hugo",
			Message:    "Hugo not found. Install with: brew install hugo or use devbox shell",
		}
	}

	logging.Debug().
		Str("version", string(output)).
		Msg("Hugo version")

	return nil
}

// build runs Hugo to build the static site
func (s *Site) build(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "hugo",
		"--source", s.rootDir,
		"--gc",
		"--minify",
		"--baseURL", s.baseURL,
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
func (s *Site) addFrontMatter(ctx context.Context, catalog catalogs.Reader) error {
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
		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		// Check if file already has front matter
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		// Skip if already has front matter
		if strings.HasPrefix(string(content), "---") {
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
	title = strings.TrimSuffix(title, ".md")
	title = strings.ReplaceAll(title, "-", " ")
	title = strings.Title(title)

	// Determine weight based on path
	weight := 10
	if strings.Contains(path, "README") {
		weight = 1
	}

	// Basic front matter
	frontMatter := fmt.Sprintf(`---
title: "%s"
weight: %d
---`, title, weight)

	// Add model-specific metadata if this is a model file
	if strings.Contains(path, "/models/") && catalog != nil {
		modelID := strings.TrimSuffix(filepath.Base(path), ".md")
		model, err := catalog.Model(modelID)
		if err == nil {
			description := model.Description
			authorNames := ""
			if len(model.Authors) > 0 {
				authorNames = model.Authors[0].Name
			}
			
			frontMatter = fmt.Sprintf(`---
title: "%s"
description: "%s"
weight: %d
author: "%s"
---`, model.Name, description, weight, authorNames)
		}
	}

	return frontMatter
}