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
		if provider.Credentials != nil {
			if !validCredentialIdentifier(string(provider.ID)) {
				return identityValidationError(
					"provider.id", provider.ID, "must be a lowercase kebab-case ID",
				)
			}
			for index, alias := range provider.Aliases {
				if !validCredentialIdentifier(string(alias)) {
					return identityValidationError(
						fmt.Sprintf("provider[%s].aliases[%d]", provider.ID, index),
						alias,
						"must be a lowercase kebab-case ID",
					)
				}
			}
			if err := provider.Credentials.validate(); err != nil {
				return errors.WrapResource("validate", "provider credentials", string(provider.ID), err)
			}
		}
		if provider.Catalog != nil && provider.Catalog.Endpoint.AuthorMapping != nil {
			if err := provider.Catalog.Endpoint.AuthorMapping.Validate(); err != nil {
				return errors.WrapResource("validate", "provider author mapping", string(provider.ID), err)
			}
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
	if err := validateCatalogCredentialAliases(providers); err != nil {
		return err
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

func validateProviderAuthorMappingTargets(reader Reader) error {
	canonicalAuthors := make(map[AuthorID]struct{})
	for _, author := range reader.Authors().List() {
		canonicalAuthors[author.ID] = struct{}{}
	}
	for _, provider := range reader.Providers().List() {
		if provider.Catalog == nil || provider.Catalog.Endpoint.AuthorMapping == nil {
			continue
		}
		for source, target := range provider.Catalog.Endpoint.AuthorMapping.Normalized {
			if _, found := canonicalAuthors[target]; !found {
				return identityValidationError(
					"provider["+provider.ID.String()+"].catalog.endpoint.author_mapping.normalized",
					source,
					"target "+target.String()+" is not a canonical catalog author ID",
				)
			}
		}
	}
	return nil
}

func validateCatalogCredentialAliases(providers []Provider) error {
	conventionalOwners := make(map[string]string)
	derivedOwners := make(map[string]string)
	for _, provider := range providers {
		if provider.Credentials == nil {
			continue
		}
		for _, field := range provider.Credentials.Fields {
			owner := string(provider.ID) + "/" + string(field.ID)
			for _, environment := range field.Environment {
				if existing, exists := conventionalOwners[environment]; exists && existing != owner {
					return identityConflictError("credential environment", environment, existing, owner)
				}
				conventionalOwners[environment] = owner
			}
			alias := credentialEnvironmentSuffix(provider.ID, field.ID)
			if existing, exists := derivedOwners[alias]; exists && existing != owner {
				return identityConflictError("derived credential environment", alias, existing, owner)
			}
			derivedOwners[alias] = owner
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
