package starmap

import (
	"strings"

	"github.com/agentstation/starmap/internal/catalog/workspace"
)

type filesystemCatalogStore interface {
	Root() string
}

func validateCatalogLayout(store any, catalogPath string) error {
	if err := validateCatalogPathSeparation(store, catalogPath); err != nil {
		return err
	}
	migrationTarget := ""
	if filesystemStore, ok := store.(filesystemCatalogStore); ok {
		migrationTarget = filesystemStore.Root()
	}
	return workspace.ValidateHumanLayout(catalogPath, migrationTarget)
}

func validateCatalogPathSeparation(store any, catalogPath string) error {
	filesystemStore, ok := store.(filesystemCatalogStore)
	if !ok || strings.TrimSpace(catalogPath) == "" {
		return nil
	}
	return workspace.ValidateMachineSeparation(catalogPath, filesystemStore.Root(), "catalog state")
}
