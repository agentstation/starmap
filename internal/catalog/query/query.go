// Package query provides shared catalog list/detail query behavior.
package query

import (
	"encoding/json"
	stderrors "errors"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"

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

// ModelRecord identifies one exact provider-scoped model record. Embedding the
// model preserves the existing flat JSON shape while provider_id prevents
// equal model IDs from becoming ambiguous in unfiltered list results.
type ModelRecord struct {
	catalogs.Model
	ProviderID catalogs.ProviderID
}

// MarshalJSON preserves the historical flat model object while adding its
// provider-scoped identity. Model has a custom marshaler, so ordinary anonymous
// embedding would otherwise consume the outer provider_id field.
func (r ModelRecord) MarshalJSON() ([]byte, error) {
	modelJSON, err := json.Marshal(r.Model)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(modelJSON, &fields); err != nil {
		return nil, err
	}
	providerID, err := json.Marshal(r.ProviderID)
	if err != nil {
		return nil, err
	}
	fields["provider_id"] = providerID
	return json.Marshal(fields)
}

// MarshalYAML preserves the same flat provider-scoped shape as MarshalJSON.
func (r ModelRecord) MarshalYAML() (any, error) {
	modelValue, err := r.Model.MarshalYAML()
	if err != nil {
		return nil, err
	}
	fields, ok := modelValue.(yaml.MapSlice)
	if !ok {
		return nil, &pkgerrors.ValidationError{
			Field:   "model",
			Value:   modelValue,
			Message: "YAML representation must be a mapping",
		}
	}
	result := make(yaml.MapSlice, 0, len(fields)+1)
	result = append(result, yaml.MapItem{Key: "provider_id", Value: r.ProviderID})
	result = append(result, fields...)
	return result, nil
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

// CatalogModels returns exact provider model records from the catalog's
// provider index. With no provider filter, equal model IDs from different
// providers remain distinct and are ordered by provider ID, then model ID.
func CatalogModels(catalog *catalogs.Catalog, provider string) ([]ModelRecord, error) {
	if catalog == nil {
		return nil, &pkgerrors.ValidationError{
			Field:   "catalog",
			Message: "catalog reader cannot be nil",
		}
	}
	if provider == "" {
		var result []ModelRecord
		for _, catalogProvider := range catalog.Providers().List() {
			modelIDs := make([]string, 0, len(catalogProvider.Models))
			for modelID := range catalogProvider.Models {
				modelIDs = append(modelIDs, modelID)
			}
			slices.Sort(modelIDs)
			for _, modelID := range modelIDs {
				model := catalogProvider.Models[modelID]
				if model != nil {
					result = append(result, ModelRecord{
						Model:      catalogs.DeepCopyModel(*model),
						ProviderID: catalogProvider.ID,
					})
				}
			}
		}
		return result, nil
	}
	catalogProvider, err := catalog.Provider(catalogs.ProviderID(provider))
	if err != nil {
		var notFound *pkgerrors.NotFoundError
		if stderrors.As(err, &notFound) {
			return []ModelRecord{}, nil
		}
		return nil, err
	}
	result := make([]ModelRecord, 0, len(catalogProvider.Models))
	modelIDs := make([]string, 0, len(catalogProvider.Models))
	for modelID := range catalogProvider.Models {
		modelIDs = append(modelIDs, modelID)
	}
	slices.Sort(modelIDs)
	for _, modelID := range modelIDs {
		model := catalogProvider.Models[modelID]
		if model != nil {
			result = append(result, ModelRecord{
				Model:      catalogs.DeepCopyModel(*model),
				ProviderID: catalogProvider.ID,
			})
		}
	}
	return result, nil
}

// Models filters, sorts, and limits model results.
func Models(models []ModelRecord, opts ModelOptions) []ModelRecord {
	filtered := make([]ModelRecord, 0, len(models))
	for _, model := range models {
		if modelMatches(model.Model, opts) {
			filtered = append(filtered, model)
		}
	}

	slices.SortStableFunc(filtered, func(a, b ModelRecord) int {
		if result := strings.Compare(a.ID, b.ID); result != 0 {
			return result
		}
		return strings.Compare(string(a.ProviderID), string(b.ProviderID))
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
