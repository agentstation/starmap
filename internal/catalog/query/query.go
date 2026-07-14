// Package query provides shared catalog list/detail query behavior.
package query

import (
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

// PageResult contains paginated query results and metadata.
type PageResult[T any] struct {
	Items  []T
	Total  int
	Limit  int
	Offset int
	Count  int
}

// Paginate returns a stable page from a result set.
func Paginate[T any](items []T, limit int, offset int) PageResult[T] {
	total := len(items)
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = total
	}

	start := offset
	end := offset + limit
	if start >= total {
		return PageResult[T]{
			Items:  []T{},
			Total:  total,
			Limit:  limit,
			Offset: offset,
			Count:  0,
		}
	}
	if end > total {
		end = total
	}

	page := append([]T(nil), items[start:end]...)
	return PageResult[T]{
		Items:  page,
		Total:  total,
		Limit:  limit,
		Offset: offset,
		Count:  len(page),
	}
}

// ModelOptions controls model list filtering.
type ModelOptions struct {
	Author     string
	Capability string
	MinContext int64
	MaxPrice   float64
	Search     string
	Limit      int
}

// CatalogModels returns presentation models projected from canonical
// definitions and, when filtered, exact provider offerings.
func CatalogModels(catalog catalogs.Reader, provider string) ([]catalogs.Model, error) {
	if catalog == nil {
		return nil, &pkgerrors.ValidationError{
			Field:   "catalog",
			Message: "catalog reader cannot be nil",
		}
	}
	if provider == "" {
		definitions := catalog.Definitions()
		models := make([]catalogs.Model, 0, len(definitions))
		for _, definition := range definitions {
			models = append(models, presentationModel(catalog, definition, nil))
		}
		return models, nil
	}
	resolved, found := catalog.Providers().Resolve(catalogs.ProviderID(provider))
	if !found {
		return []catalogs.Model{}, nil
	}
	definitions := make(map[catalogs.ModelDefinitionID]catalogs.ModelDefinition)
	for _, definition := range catalog.Definitions() {
		definitions[definition.ID] = definition
	}
	models := make([]catalogs.Model, 0)
	for _, offering := range catalog.Offerings() {
		if offering.ProviderID != resolved.ID {
			continue
		}
		definition, ok := definitions[offering.DefinitionID]
		if !ok {
			return nil, &pkgerrors.NotFoundError{Resource: "model definition", ID: string(offering.DefinitionID)}
		}
		models = append(models, presentationModel(catalog, definition, &offering))
	}
	return models, nil
}

func presentationModel(catalog catalogs.Reader, definition catalogs.ModelDefinition, offering *catalogs.ProviderOffering) catalogs.Model {
	authors := make([]catalogs.Author, 0, len(definition.AuthorIDs))
	for _, authorID := range definition.AuthorIDs {
		if author, err := catalog.Author(authorID); err == nil {
			authors = append(authors, author)
		}
	}
	model := catalogs.Model{
		ID:          string(definition.ID),
		Name:        definition.Name,
		Authors:     authors,
		Description: definition.Description,
		Metadata: &catalogs.ModelMetadata{
			ReleaseDate: definition.Metadata.ReleaseDate, OpenWeights: definition.Weights.Open,
			KnowledgeCutoff: definition.Metadata.KnowledgeCutoff, Tags: definition.Metadata.Tags,
			Architecture: definition.Weights.Architecture,
		},
		Features: definition.Capabilities.Features, Attachments: definition.Capabilities.Attachments,
		Generation: definition.Capabilities.Generation, Reasoning: definition.Capabilities.Reasoning,
		ReasoningTokens: definition.Capabilities.ReasoningTokens, Verbosity: definition.Capabilities.Verbosity,
		Tools: definition.Capabilities.Tools, Delivery: definition.Capabilities.Delivery,
		CreatedAt: definition.CreatedAt, UpdatedAt: definition.UpdatedAt,
	}
	if offering != nil {
		model.ID = string(offering.ProviderModelID)
		model.Pricing = offering.Pricing
		model.Limits = offering.Limits
	}
	return model
}

// Models filters, sorts, and limits model results.
func Models(models []catalogs.Model, opts ModelOptions) []catalogs.Model {
	filtered := make([]catalogs.Model, 0, len(models))
	for _, model := range models {
		if modelMatches(model, opts) {
			filtered = append(filtered, model)
		}
	}

	slices.SortFunc(filtered, func(a, b catalogs.Model) int {
		return strings.Compare(a.ID, b.ID)
	})

	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
	}
	return filtered
}

func modelMatches(model catalogs.Model, opts ModelOptions) bool {
	if opts.Author != "" && !modelMatchesAuthor(model, opts.Author) {
		return false
	}
	if opts.Capability != "" && !modelMatchesCapability(model, opts.Capability) {
		return false
	}
	if opts.MinContext > 0 && (model.Limits == nil || model.Limits.ContextWindow < opts.MinContext) {
		return false
	}
	if opts.MaxPrice > 0 && !modelMatchesMaxPrice(model, opts.MaxPrice) {
		return false
	}
	if opts.Search != "" && !modelMatchesSearch(model, opts.Search) {
		return false
	}
	return true
}

func modelMatchesAuthor(model catalogs.Model, authorQuery string) bool {
	for _, author := range model.Authors {
		if strings.EqualFold(string(author.ID), authorQuery) ||
			strings.EqualFold(author.Name, authorQuery) {
			return true
		}
	}
	return false
}

func modelMatchesCapability(model catalogs.Model, capability string) bool {
	if model.Features == nil {
		return false
	}

	switch strings.ToLower(capability) {
	case featureToolCalls, featureTools:
		return model.Features.ToolCalls || model.Features.Tools
	case featureReasoning:
		return model.Features.Reasoning
	case featureStreaming:
		return model.Features.Streaming
	case "vision", "image":
		return slices.Contains(model.Features.Modalities.Input, catalogs.ModelModalityImage)
	default:
		return false
	}
}

func modelMatchesMaxPrice(model catalogs.Model, maxPrice float64) bool {
	if model.Pricing == nil || model.Pricing.Tokens == nil || model.Pricing.Tokens.Input == nil {
		return true
	}
	return model.Pricing.Tokens.Input.Per1M <= maxPrice
}

func modelMatchesSearch(model catalogs.Model, query string) bool {
	search := strings.ToLower(query)
	if strings.Contains(strings.ToLower(model.ID), search) {
		return true
	}
	if strings.Contains(strings.ToLower(model.Name), search) {
		return true
	}
	if strings.Contains(strings.ToLower(model.Description), search) {
		return true
	}
	for _, author := range model.Authors {
		if strings.Contains(strings.ToLower(author.Name), search) {
			return true
		}
	}
	return false
}

// ProviderOptions controls provider list filtering.
type ProviderOptions struct {
	Search string
	Limit  int
}

// Providers filters, sorts, and limits provider results.
func Providers(providers []catalogs.Provider, opts ProviderOptions) []catalogs.Provider {
	filtered := make([]catalogs.Provider, 0, len(providers))
	for _, provider := range providers {
		if providerMatches(provider, opts.Search) {
			filtered = append(filtered, provider)
		}
	}

	slices.SortFunc(filtered, func(a, b catalogs.Provider) int {
		return strings.Compare(string(a.ID), string(b.ID))
	})

	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
	}
	return filtered
}

func providerMatches(provider catalogs.Provider, query string) bool {
	if query == "" {
		return true
	}
	search := strings.ToLower(query)
	if strings.Contains(strings.ToLower(string(provider.ID)), search) {
		return true
	}
	if strings.Contains(strings.ToLower(provider.Name), search) {
		return true
	}
	return provider.Headquarters != nil && strings.Contains(strings.ToLower(*provider.Headquarters), search)
}

// AuthorOptions controls author list filtering.
type AuthorOptions struct {
	Search string
	Limit  int
}

// Authors filters, sorts, and limits author results.
func Authors(authors []catalogs.Author, opts AuthorOptions) []catalogs.Author {
	filtered := make([]catalogs.Author, 0, len(authors))
	for _, author := range authors {
		if authorMatches(author, opts.Search) {
			filtered = append(filtered, author)
		}
	}

	slices.SortFunc(filtered, func(a, b catalogs.Author) int {
		return strings.Compare(string(a.ID), string(b.ID))
	})

	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
	}
	return filtered
}

func authorMatches(author catalogs.Author, query string) bool {
	if query == "" {
		return true
	}
	search := strings.ToLower(query)
	if strings.Contains(strings.ToLower(string(author.ID)), search) {
		return true
	}
	if strings.Contains(strings.ToLower(author.Name), search) {
		return true
	}
	return author.Description != nil && strings.Contains(strings.ToLower(*author.Description), search)
}
