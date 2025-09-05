//nolint:gosec // Internal documentation generation tool with controlled file operations
package docs

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
)

// LogoCopier handles copying logos from embedded resources.
type LogoCopier struct {
	embedFS   embed.FS
	sourceDir string
	targetDir string
}

// NewLogoCopier creates a new logo copier.
func NewLogoCopier(embedFS embed.FS, sourceDir, targetDir string) *LogoCopier {
	return &LogoCopier{
		embedFS:   embedFS,
		sourceDir: sourceDir,
		targetDir: targetDir,
	}
}

// CopyProviderLogos copies all provider logos to the documentation directory.
func (lc *LogoCopier) CopyProviderLogos(providers []*catalogs.Provider) error {
	logoDir := filepath.Join(lc.targetDir, "assets", "logos", "providers")
	if err := os.MkdirAll(logoDir, constants.DirPermissions); err != nil {
		return fmt.Errorf("creating logo directory: %w", err)
	}

	for _, provider := range providers {
		if err := lc.copyProviderLogo(provider.ID, logoDir); err != nil {
			// Log error but continue with other logos
			fmt.Printf("Warning: failed to copy logo for %s: %v\n", provider.ID, err)
		}
	}

	return nil
}

// CopyAuthorLogos copies all author logos to the documentation directory.
func (lc *LogoCopier) CopyAuthorLogos(authors []*catalogs.Author) error {
	logoDir := filepath.Join(lc.targetDir, "assets", "logos", "authors")
	if err := os.MkdirAll(logoDir, constants.DirPermissions); err != nil {
		return fmt.Errorf("creating logo directory: %w", err)
	}

	for _, author := range authors {
		if err := lc.copyAuthorLogo(author.ID, logoDir); err != nil {
			// Log error but continue with other logos
			fmt.Printf("Warning: failed to copy logo for %s: %v\n", author.ID, err)
		}
	}

	return nil
}

// copyProviderLogo copies a single provider logo from embedded FS.
func (lc *LogoCopier) copyProviderLogo(providerID catalogs.ProviderID, targetDir string) error {
	// Check for logo.svg in the provider's directory
	sourcePath := filepath.Join(lc.sourceDir, "providers", string(providerID), "logo.svg")
	if err := lc.copyLogo(sourcePath, targetDir, string(providerID)+".svg"); err == nil {
		return nil // Successfully copied
	}

	// Try different logo formats in the logos/providers directory as fallback
	formats := []string{".svg", ".png", ".jpg", ".jpeg", ".webp"}
	for _, format := range formats {
		sourcePath := filepath.Join(lc.sourceDir, "logos", "providers", string(providerID)+format)
		if err := lc.copyLogo(sourcePath, targetDir, string(providerID)+format); err == nil {
			return nil // Successfully copied
		}
	}

	return fmt.Errorf("no logo found for provider %s", providerID)
}

// copyAuthorLogo copies a single author logo from embedded FS.
func (lc *LogoCopier) copyAuthorLogo(authorID catalogs.AuthorID, targetDir string) error {
	// Check for logo.svg in the author's directory
	sourcePath := filepath.Join(lc.sourceDir, "authors", string(authorID), "logo.svg")
	if err := lc.copyLogo(sourcePath, targetDir, string(authorID)+".svg"); err == nil {
		return nil // Successfully copied
	}

	// Try different logo formats in the logos/authors directory as fallback
	formats := []string{".svg", ".png", ".jpg", ".jpeg", ".webp"}
	for _, format := range formats {
		sourcePath := filepath.Join(lc.sourceDir, "logos", "authors", string(authorID)+format)
		if err := lc.copyLogo(sourcePath, targetDir, string(authorID)+format); err == nil {
			return nil // Successfully copied
		}
	}

	return fmt.Errorf("no logo found for author %s", authorID)
}

// copyLogo copies a single logo file.
func (lc *LogoCopier) copyLogo(sourcePath, targetDir, filename string) error {
	// Open source file from embedded FS
	sourceFile, err := lc.embedFS.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	// Create target file
	targetPath := filepath.Join(targetDir, filename)
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("creating target file: %w", err)
	}
	defer func() { _ = targetFile.Close() }()

	// Copy file contents
	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return fmt.Errorf("copying logo: %w", err)
	}

	return nil
}

// CopyAllLogos copies all logos from embedded resources.
func (lc *LogoCopier) CopyAllLogos() error {
	logoSourceDir := filepath.Join(lc.sourceDir, "logos")
	logoTargetDir := filepath.Join(lc.targetDir, "assets", "logos")

	// Walk through embedded logos directory
	err := fs.WalkDir(lc.embedFS, logoSourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(logoSourceDir, path)
		if err != nil {
			return fmt.Errorf("calculating relative path: %w", err)
		}

		// Create target directory structure
		targetPath := filepath.Join(logoTargetDir, relPath)
		targetDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetDir, constants.DirPermissions); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}

		// Copy the file
		sourceFile, err := lc.embedFS.Open(path)
		if err != nil {
			return fmt.Errorf("opening source file: %w", err)
		}
		defer func() { _ = sourceFile.Close() }()

		targetFile, err := os.Create(targetPath)
		if err != nil {
			return fmt.Errorf("creating target file: %w", err)
		}
		defer func() { _ = targetFile.Close() }()

		if _, err := io.Copy(targetFile, sourceFile); err != nil {
			return fmt.Errorf("copying file: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walking logos directory: %w", err)
	}

	return nil
}

// getLogoPath returns the local path for a provider/author logo.
//
//nolint:unused // Used in tests
func getLogoPath(id string, logoType string) string {
	// Return a relative path to the locally copied logo
	return fmt.Sprintf("../../assets/logos/%s/%s.svg", logoType, id)
}

// getProviderLogoPath returns the local path for a provider logo.
//
//nolint:unused // Used in tests
func getProviderLogoPath(providerID catalogs.ProviderID) string {
	return getLogoPath(string(providerID), "providers")
}

// getAuthorLogoPath returns the local path for an author logo.
//
//nolint:unused // Used in tests
func getAuthorLogoPath(authorID catalogs.AuthorID) string {
	return getLogoPath(string(authorID), "authors")
}

// generateLogoHTML generates HTML for embedding a logo.
//
//nolint:unused // Used in tests
func logoHTML(logoPath, alt string, width, height int) string {
	return fmt.Sprintf(`<img src="%s" alt="%s" width="%d" height="%d" style="vertical-align: middle;">`,
		logoPath, alt, width, height)
}

// generateProviderLogoHTML generates HTML for a provider logo.
//
//nolint:unused // Used in tests
func providerLogoHTML(provider *catalogs.Provider) string {
	logoPath := getProviderLogoPath(provider.ID)
	return logoHTML(logoPath, provider.Name, 32, 32)
}

// generateAuthorLogoHTML generates HTML for an author logo.
//
//nolint:unused // Used in tests
func authorLogoHTML(author *catalogs.Author) string {
	logoPath := getAuthorLogoPath(author.ID)
	return logoHTML(logoPath, author.Name, 32, 32)
}

// optimizeSVG performs basic SVG optimization.
//
//nolint:unused // Used in tests
func optimizeSVG(svgContent []byte) []byte {
	svg := string(svgContent)

	// Remove unnecessary whitespace
	svg = strings.ReplaceAll(svg, "\n", " ")
	svg = strings.ReplaceAll(svg, "\r", "")
	svg = strings.ReplaceAll(svg, "\t", " ")

	// Collapse multiple spaces
	for strings.Contains(svg, "  ") {
		svg = strings.ReplaceAll(svg, "  ", " ")
	}

	// Remove spaces around tags
	svg = strings.ReplaceAll(svg, "> <", "><")

	return []byte(svg)
}

// generateFallbackLogo generates a fallback logo when the actual logo is missing.
//
//nolint:unused // Used in tests
func createFallbackLogo(name string, outputPath string) error {
	// Generate a simple SVG with the first letter of the name
	initial := "?"
	if name != "" && len(name) > 0 {
		initial = strings.ToUpper(name[:1])
	}

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32">
  <rect width="32" height="32" rx="4" fill="#e0e0e0"/>
  <text x="50%%" y="50%%" font-family="Arial, sans-serif" font-size="18" font-weight="bold" 
        text-anchor="middle" dominant-baseline="central" fill="#333">%s</text>
</svg>`, initial)

	return os.WriteFile(outputPath, []byte(svg), constants.FilePermissions)
}

// ensureLogosExist ensures all required logos exist, creating fallbacks if needed.
//
//nolint:unused // Used in tests
func ensureLogosExist(providers []*catalogs.Provider, authors []*catalogs.Author, targetDir string) error {
	// Check provider logos
	providerLogoDir := filepath.Join(targetDir, "assets", "logos", "providers")
	if err := os.MkdirAll(providerLogoDir, constants.DirPermissions); err != nil {
		return fmt.Errorf("creating provider logo directory: %w", err)
	}

	for _, provider := range providers {
		logoPath := filepath.Join(providerLogoDir, string(provider.ID)+".svg")
		if _, err := os.Stat(logoPath); os.IsNotExist(err) {
			if err := createFallbackLogo(provider.Name, logoPath); err != nil {
				return fmt.Errorf("generating fallback logo for %s: %w", provider.ID, err)
			}
		}
	}

	// Check author logos
	authorLogoDir := filepath.Join(targetDir, "assets", "logos", "authors")
	if err := os.MkdirAll(authorLogoDir, constants.DirPermissions); err != nil {
		return fmt.Errorf("creating author logo directory: %w", err)
	}

	for _, author := range authors {
		logoPath := filepath.Join(authorLogoDir, string(author.ID)+".svg")
		if _, err := os.Stat(logoPath); os.IsNotExist(err) {
			if err := createFallbackLogo(author.Name, logoPath); err != nil {
				return fmt.Errorf("generating fallback logo for %s: %w", author.ID, err)
			}
		}
	}

	return nil
}
