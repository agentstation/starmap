package modelsdev

import (
	"io"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// CopyProviderLogos copies provider logos from models.dev to output directory.
func CopyProviderLogos(outputDir string, providerIDs []catalogs.ProviderID) error {
	// The models.dev repo is always cloned to this location by git.Fetch()
	modelsDevRepo := filepath.Join("internal/embedded/catalog/providers", "models.dev-git")
	providersPath := filepath.Join(modelsDevRepo, "providers")

	for _, providerID := range providerIDs {
		sourceLogo := filepath.Join(providersPath, string(providerID), "logo.svg")
		// Logos should go in providers/<provider_id>/logo.svg
		destLogo := filepath.Join(outputDir, "providers", string(providerID), "logo.svg")

		// Check if source logo exists before copying
		if _, err := os.Stat(sourceLogo); os.IsNotExist(err) {
			// Skip if logo doesn't exist in models.dev
			continue
		}

		if err := copyFile(sourceLogo, destLogo); err != nil {
			logging.Warn().
				Err(err).
				Str("provider_id", string(providerID)).
				Msg("Could not copy logo for provider")
			// Don't fail the entire operation for missing logos
		}
	}

	return nil
}

// copyFile copies a file from source to destination.
func copyFile(src, dst string) error {
	// Check if source file exists
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return &errors.NotFoundError{
			Resource: "file",
			ID:       src,
		}
	}

	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(dst)
	if err := os.MkdirAll(destDir, constants.DirPermissions); err != nil {
		return errors.WrapIO("create", "destination directory", err)
	}

	// Open source file
	srcFile, err := os.Open(src) //nolint:gosec // Input paths are controlled by internal tooling
	if err != nil {
		return errors.WrapIO("open", src, err)
	}
	defer func() { _ = srcFile.Close() }()

	// Create destination file
	dstFile, err := os.Create(dst) //nolint:gosec // Output paths are controlled by internal tooling
	if err != nil {
		return errors.WrapIO("create", dst, err)
	}
	defer func() { _ = dstFile.Close() }()

	// Copy file contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return errors.WrapIO("copy", "file contents", err)
	}

	return nil
}
