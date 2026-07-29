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
// It tries the provider ID first, then checks aliases if the primary ID isn't found.
func CopyProviderLogos(outputDir string, providers []*catalogs.Provider) error {
	// The models.dev repo is always cloned to this location by git.Fetch()
	sourcesPath := expandPath(constants.DefaultSourcesPath)
	modelsDevRepo := filepath.Join(sourcesPath, "models.dev-git")
	providersPath := filepath.Join(modelsDevRepo, "providers")

	for _, provider := range providers {
		// Try provider ID first
		sourceLogo := filepath.Join(providersPath, string(provider.ID), "logo.svg")
		destLogo := filepath.Join(outputDir, "providers", string(provider.ID), "logo.svg")

		// Check if logo exists with primary ID
		if _, err := os.Stat(sourceLogo); err == nil {
			// Found with primary ID - copy it
			if err := copyFile(sourceLogo, destLogo); err != nil {
				logging.Warn().
					Err(err).
					Str("provider_id", string(provider.ID)).
					Msg("Could not copy logo for provider")
			}
			continue
		}

		// Primary ID not found - try aliases
		found := false
		for _, alias := range provider.Aliases {
			aliasSourceLogo := filepath.Join(providersPath, string(alias), "logo.svg")
			if _, err := os.Stat(aliasSourceLogo); err == nil {
				// Found with alias - copy it
				if err := copyFile(aliasSourceLogo, destLogo); err != nil {
					logging.Warn().
						Err(err).
						Str("provider_id", string(provider.ID)).
						Str("alias", string(alias)).
						Msg("Could not copy logo for provider from alias")
				} else {
					logging.Debug().
						Str("provider_id", string(provider.ID)).
						Str("alias", string(alias)).
						Msg("Copied logo from alias")
					found = true
				}
				break
			}
		}

		if !found {
			// No logo found with primary ID or any alias - skip silently
			logging.Debug().
				Str("provider_id", string(provider.ID)).
				Msg("No logo found in models.dev (checked ID and aliases)")
		}
	}

	return nil
}

// CopyAuthorLogos copies author logos from models.dev provider logos to author directories.
// Since models.dev doesn't have a separate authors directory, we copy from the provider
// directory when the author ID matches a provider ID (or alias).
func CopyAuthorLogos(outputDir string, authors []catalogs.Author, providers catalogs.ProvidersReader) error {
	sourcesPath := expandPath(constants.DefaultSourcesPath)
	modelsDevRepo := filepath.Join(sourcesPath, "models.dev-git")
	providersPath := filepath.Join(modelsDevRepo, "providers")

	for _, author := range authors {
		sourceLogo := findAuthorLogo(providersPath, author, providers)
		destLogo := filepath.Join(outputDir, "authors", string(author.ID), "logo.svg")
		if sourceLogo == "" {
			logging.Debug().
				Str("author_id", string(author.ID)).
				Msg("No logo found in models.dev")
			continue
		}
		if err := copyFile(sourceLogo, destLogo); err != nil {
			logging.Warn().
				Err(err).
				Str("author_id", string(author.ID)).
				Msg("Could not copy logo for author")
			continue
		}
		logging.Debug().
			Str("author_id", string(author.ID)).
			Msg("Copied logo for author")
	}

	return nil
}

func findAuthorLogo(
	providersPath string,
	author catalogs.Author,
	providers catalogs.ProvidersReader,
) string {
	if author.Catalog != nil &&
		author.Catalog.Attribution != nil &&
		author.Catalog.Attribution.ProviderID != "" {
		if provider, exists := providers.Get(author.Catalog.Attribution.ProviderID); exists {
			ids := logoIDs(provider.ID, provider.Aliases)
			if logo, matched := firstExistingLogo(providersPath, ids); logo != "" {
				if matched != string(provider.ID) {
					logging.Debug().
						Str("author_id", string(author.ID)).
						Str("provider_alias", matched).
						Msg("Found logo using provider alias")
				}
				return logo
			}
		}
	}

	ids := logoIDs(author.ID, author.Aliases)
	logo, matched := firstExistingLogo(providersPath, ids)
	if logo == "" {
		return ""
	}
	event := logging.Debug().Str("author_id", string(author.ID))
	if matched == string(author.ID) {
		event.Msg("Found logo using author ID")
	} else {
		event.Str("author_alias", matched).Msg("Found logo using author alias")
	}
	return logo
}

func logoIDs[T ~string](id T, aliases []T) []string {
	ids := make([]string, 1, len(aliases)+1)
	ids[0] = string(id)
	for _, alias := range aliases {
		ids = append(ids, string(alias))
	}
	return ids
}

func firstExistingLogo(providersPath string, ids []string) (string, string) {
	for _, id := range ids {
		logo := filepath.Join(providersPath, id, "logo.svg")
		if _, err := os.Stat(logo); err == nil {
			return logo, id
		}
	}
	return "", ""
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
