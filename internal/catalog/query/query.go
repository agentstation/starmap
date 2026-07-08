// Package query provides shared catalog list/detail query behavior.
package query

import (
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
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
	Provider           string
	ProviderModelIndex ProviderModelIndex
	Author             string
	Capability         string
	MinContext         int64
	MaxPrice           float64
	Search             string
	Limit              int
}

// ProviderModelIndex maps provider IDs and aliases to the model IDs they serve.
type ProviderModelIndex map[catalogs.ProviderID]map[string]struct{}

// NewProviderModelIndex builds a provider membership index from catalog providers.
func NewProviderModelIndex(providers []catalogs.Provider) ProviderModelIndex {
	index := make(ProviderModelIndex, len(providers))
	for _, provider := range providers {
		modelIDs := make(map[string]struct{}, len(provider.Models))
		for modelID := range provider.Models {
			modelIDs[modelID] = struct{}{}
		}
		index[provider.ID] = modelIDs
		for _, alias := range provider.Aliases {
			index[alias] = modelIDs
		}
	}
	return index
}

// Contains reports whether the provider or one of its aliases serves modelID.
func (i ProviderModelIndex) Contains(provider catalogs.ProviderID, modelID string) bool {
	if i == nil {
		return false
	}
	models, ok := i[provider]
	if !ok {
		return false
	}
	_, ok = models[modelID]
	return ok
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
	if opts.Provider != "" && !opts.ProviderModelIndex.Contains(catalogs.ProviderID(opts.Provider), model.ID) {
		return false
	}
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
	case "tool_calls", "tools":
		return model.Features.ToolCalls || model.Features.Tools
	case "reasoning":
		return model.Features.Reasoning
	case "streaming":
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
