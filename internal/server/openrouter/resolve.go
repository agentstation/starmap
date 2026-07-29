package openrouter

import (
	"errors"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

type resolvedModel struct {
	definition catalogs.ModelDefinition
	offerings  []catalogs.ProviderOffering
	variant    string
}

func resolve(
	catalog *catalogs.Catalog,
	authorID catalogs.AuthorID,
	slug string,
) (resolvedModel, error) {
	if catalog == nil {
		return resolvedModel{}, &pkgerrors.ValidationError{
			Field: "openrouter.catalog", Message: "is required",
		}
	}
	author, found := catalog.Authors().Resolve(authorID)
	if !found || author == nil {
		return resolvedModel{}, notFound(authorID, slug)
	}

	baseSlug, variant := splitVariant(slug)
	if baseSlug == "" || (strings.Contains(slug, ":") && variant == "") {
		return resolvedModel{}, notFound(authorID, slug)
	}
	definition, err := resolveDefinition(catalog, author.ID, baseSlug)
	if err != nil {
		return resolvedModel{}, err
	}
	offerings, err := catalog.DefinitionOfferings(definition.ID)
	if err != nil {
		return resolvedModel{}, err
	}
	offerings = eligibleOfferings(offerings)
	if variant != "" {
		offerings = variantOfferings(offerings, variant)
		if len(offerings) == 0 {
			return resolvedModel{}, &pkgerrors.NotFoundError{
				Resource: "model variant",
				ID:       string(definition.ID) + ":" + variant,
			}
		}
	}
	return resolvedModel{
		definition: definition,
		offerings:  offerings,
		variant:    variant,
	}, nil
}

func resolveDefinition(
	catalog *catalogs.Catalog,
	authorID catalogs.AuthorID,
	slug string,
) (catalogs.ModelDefinition, error) {
	definition, err := catalog.AuthorModel(authorID, slug)
	if err == nil {
		return definition, nil
	}
	if !pkgerrors.IsNotFound(err) {
		return catalogs.ModelDefinition{}, err
	}

	for _, alias := range []string{string(authorID) + "/" + slug, slug} {
		definition, err = catalog.FindModel(alias)
		if err == nil {
			if definitionHasAuthor(definition, authorID) {
				return definition, nil
			}
			return catalogs.ModelDefinition{}, &pkgerrors.NotFoundError{
				Resource: "model", ID: string(authorID) + "/" + slug,
			}
		}
		if !pkgerrors.IsNotFound(err) {
			return catalogs.ModelDefinition{}, err
		}
	}
	return catalogs.ModelDefinition{}, notFound(authorID, slug)
}

func definitionHasAuthor(
	definition catalogs.ModelDefinition,
	authorID catalogs.AuthorID,
) bool {
	for _, candidate := range definition.AuthorIDs {
		if candidate == authorID {
			return true
		}
	}
	return false
}

func splitVariant(slug string) (string, string) {
	base, variant, found := strings.Cut(slug, ":")
	if !found {
		return slug, ""
	}
	if strings.Contains(variant, ":") {
		return "", ""
	}
	return base, variant
}

func eligibleOfferings(
	offerings []catalogs.ProviderOffering,
) []catalogs.ProviderOffering {
	eligible := make([]catalogs.ProviderOffering, 0, len(offerings))
	for _, offering := range offerings {
		if offering.Availability == catalogs.OfferingAvailabilityUnavailable ||
			offering.Lifecycle == catalogs.OfferingLifecycleRetired {
			continue
		}
		eligible = append(eligible, offering)
	}
	return eligible
}

func variantOfferings(
	offerings []catalogs.ProviderOffering,
	variant string,
) []catalogs.ProviderOffering {
	selected := make([]catalogs.ProviderOffering, 0, len(offerings))
	for _, offering := range offerings {
		mode, found := offering.Modes[variant]
		if !found {
			continue
		}
		if mode.Pricing != nil {
			offering.Pricing = mode.Pricing
		}
		offering.Modes = nil
		selected = append(selected, offering)
	}
	return selected
}

func notFound(authorID catalogs.AuthorID, slug string) error {
	return &pkgerrors.NotFoundError{
		Resource: "model", ID: string(authorID) + "/" + slug,
	}
}

func isNotFoundOrConflict(err error) bool {
	var conflict *pkgerrors.ConflictError
	return pkgerrors.IsNotFound(err) || errors.As(err, &conflict)
}
