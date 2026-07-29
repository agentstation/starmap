package catalogs

import (
	"fmt"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

func validateCatalogIdentities(reader Reader) error {
	providers := reader.Providers().List()
	providerOwners := make(map[ProviderID]ProviderID, len(providers))
	for _, provider := range providers {
		if strings.TrimSpace(string(provider.ID)) == "" {
			return identityValidationError("provider.id", provider.ID, "is required")
		}
		providerOwners[provider.ID] = provider.ID
	}
	for _, provider := range providers {
		for index, alias := range provider.Aliases {
			if strings.TrimSpace(string(alias)) == "" {
				return identityValidationError(
					fmt.Sprintf("provider[%s].aliases[%d]", provider.ID, index),
					alias,
					"must not be empty",
				)
			}
			if owner, exists := providerOwners[alias]; exists {
				return identityConflictError("provider alias", string(alias), string(owner), string(provider.ID))
			}
			providerOwners[alias] = provider.ID
		}
	}

	authors := reader.Authors().List()
	authorOwners := make(map[AuthorID]AuthorID, len(authors))
	for _, author := range authors {
		if strings.TrimSpace(string(author.ID)) == "" {
			return identityValidationError("author.id", author.ID, "is required")
		}
		authorOwners[author.ID] = author.ID
	}
	for _, author := range authors {
		for index, alias := range author.Aliases {
			if strings.TrimSpace(string(alias)) == "" {
				return identityValidationError(
					fmt.Sprintf("author[%s].aliases[%d]", author.ID, index),
					alias,
					"must not be empty",
				)
			}
			if owner, exists := authorOwners[alias]; exists {
				return identityConflictError("author alias", string(alias), string(owner), string(author.ID))
			}
			authorOwners[alias] = author.ID
		}
	}
	return nil
}

func identityValidationError(field string, value any, message string) error {
	return &errors.ValidationError{Field: field, Value: value, Message: message}
}

func identityConflictError(resource, alias, existingOwner, proposedOwner string) error {
	return &errors.ConflictError{
		Resource: resource,
		Message: fmt.Sprintf(
			"%q resolves to both %q and %q",
			alias,
			existingOwner,
			proposedOwner,
		),
	}
}
