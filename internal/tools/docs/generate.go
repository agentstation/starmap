package docs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/embedded"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
)

// Generator handles documentation generation
type Generator struct {
	outputDir string
	verbose   bool
}

// Option is a functional option for configuring the Generator
type Option func(*Generator)

// WithOutputDir sets the output directory for generated documentation
func WithOutputDir(dir string) Option {
	return func(g *Generator) {
		g.outputDir = dir
	}
}

// WithVerbose enables verbose output
func WithVerbose(verbose bool) Option {
	return func(g *Generator) {
		g.verbose = verbose
	}
}

// New creates a new documentation generator
func New(opts ...Option) *Generator {
	g := &Generator{
		outputDir: "./docs",
		verbose:   false,
	}

	for _, opt := range opts {
		opt(g)
	}

	return g
}

// Generate generates all documentation for the catalog
func (g *Generator) Generate(ctx context.Context, catalog catalogs.Reader) error {
	if g.verbose {
		fmt.Printf("📝 Generating documentation in %s...\n", g.outputDir)
	}

	// Create output directories
	catalogDir := filepath.Join(g.outputDir, "catalog")
	providersDir := filepath.Join(catalogDir, "providers")
	authorsDir := filepath.Join(catalogDir, "authors")
	modelsDir := filepath.Join(catalogDir, "models")

	dirs := []string{catalogDir, providersDir, authorsDir, modelsDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, constants.DirPermissions); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Generate main catalog index
	if err := g.generateCatalogIndex(catalogDir, catalog); err != nil {
		return fmt.Errorf("generating catalog index: %w", err)
	}

	// Generate provider documentation
	if err := g.generateProviderDocs(providersDir, catalog); err != nil {
		return fmt.Errorf("generating provider docs: %w", err)
	}

	// Generate author documentation
	if err := g.generateAuthorDocs(authorsDir, catalog); err != nil {
		return fmt.Errorf("generating author docs: %w", err)
	}

	// Generate individual model pages
	if err := g.generateModelDocs(modelsDir, catalog); err != nil {
		return fmt.Errorf("generating model docs: %w", err)
	}

	// Copy logos from embedded resources to documentation directory
	if err := g.copyLogos(catalog); err != nil {
		// Log warning but don't fail - documentation is still usable without logos
		if g.verbose {
			fmt.Printf("⚠️  Warning: Failed to copy logos: %v\n", err)
		}
	}

	if g.verbose {
		fmt.Println("✅ Documentation generation complete!")
	}

	return nil
}

// copyLogos copies logos from embedded resources to the documentation directory
func (g *Generator) copyLogos(catalog catalogs.Reader) error {
	// Create logo copier with embedded FS
	logoCopier := NewLogoCopier(embedded.FS, "catalog", g.outputDir)
	
	// Copy provider logos
	providers := catalog.Providers().List()
	if err := logoCopier.CopyProviderLogos(providers); err != nil {
		return fmt.Errorf("copying provider logos: %w", err)
	}
	
	// Copy author logos
	authors := catalog.Authors().List()
	if err := logoCopier.CopyAuthorLogos(authors); err != nil {
		return fmt.Errorf("copying author logos: %w", err)
	}
	
	if g.verbose {
		fmt.Printf("📦 Copied logos for %d providers and %d authors\n", len(providers), len(authors))
	}
	
	return nil
}