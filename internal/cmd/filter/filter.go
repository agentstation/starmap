package filter

import (
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// ModelFilter applies filters to model lists
type ModelFilter struct {
	Provider    string
	Author      string
	Capability  string
	MinContext  int64
	MaxPrice    float64
	Search      string // General search term
}

// Apply filters a slice of models
func (f *ModelFilter) Apply(models []catalogs.Model) []catalogs.Model {
	if f == nil || f.isEmpty() {
		return models
	}
	
	var filtered []catalogs.Model
	
	for _, model := range models {
		if f.matches(model) {
			filtered = append(filtered, model)
		}
	}
	
	return filtered
}

func (f *ModelFilter) isEmpty() bool {
	return f.Provider == "" && 
		f.Author == "" && 
		f.Capability == "" && 
		f.MinContext == 0 && 
		f.MaxPrice == 0 && 
		f.Search == ""
}

func (f *ModelFilter) matches(model catalogs.Model) bool {
	// Provider filter
	if f.Provider != "" && !f.matchesProvider(model) {
		return false
	}
	
	// Author filter
	if f.Author != "" && !f.matchesAuthor(model) {
		return false
	}
	
	// Capability filter
	if f.Capability != "" && !f.matchesCapability(model) {
		return false
	}
	
	// Context window filter
	if f.MinContext > 0 && !f.matchesContext(model) {
		return false
	}
	
	// Price filter
	if f.MaxPrice > 0 && !f.matchesPrice(model) {
		return false
	}
	
	// Search filter
	if f.Search != "" && !f.matchesSearch(model) {
		return false
	}
	
	return true
}

func (f *ModelFilter) matchesProvider(model catalogs.Model) bool {
	// Check if model has a provider association
	// This might need adjustment based on how providers are stored
	return true // Placeholder - needs implementation based on model structure
}

func (f *ModelFilter) matchesAuthor(model catalogs.Model) bool {
	for _, author := range model.Authors {
		if strings.EqualFold(string(author.ID), f.Author) || 
		   strings.EqualFold(author.Name, f.Author) {
			return true
		}
	}
	return false
}

func (f *ModelFilter) matchesCapability(model catalogs.Model) bool {
	if model.Features == nil {
		return false
	}
	
	// Check specific capability flags
	switch strings.ToLower(f.Capability) {
	case "tool_calls", "tools":
		return model.Features.ToolCalls || model.Features.Tools
	case "reasoning":
		return model.Features.Reasoning
	case "streaming":
		return model.Features.Streaming
	case "vision", "image":
		if model.Features.Modalities.Input != nil {
			for _, modality := range model.Features.Modalities.Input {
				if modality == "image" {
					return true
				}
			}
		}
	}
	
	return false
}

func (f *ModelFilter) matchesContext(model catalogs.Model) bool {
	if model.Limits == nil {
		return false
	}
	return model.Limits.ContextWindow >= f.MinContext
}

func (f *ModelFilter) matchesPrice(model catalogs.Model) bool {
	if model.Pricing == nil || model.Pricing.Tokens == nil || model.Pricing.Tokens.Input == nil {
		return true // No price info means we include it
	}
	return model.Pricing.Tokens.Input.Per1M <= f.MaxPrice
}

func (f *ModelFilter) matchesSearch(model catalogs.Model) bool {
	search := strings.ToLower(f.Search)
	
	// Search in ID
	if strings.Contains(strings.ToLower(model.ID), search) {
		return true
	}
	
	// Search in name
	if strings.Contains(strings.ToLower(model.Name), search) {
		return true
	}
	
	// Search in description
	if strings.Contains(strings.ToLower(model.Description), search) {
		return true
	}
	
	// Search in authors
	for _, author := range model.Authors {
		if strings.Contains(strings.ToLower(author.Name), search) {
			return true
		}
	}
	
	return false
}

// ProviderFilter applies filters to provider lists
type ProviderFilter struct {
	HasClient  bool
	Configured bool
	Search     string
}

// Apply filters a slice of providers
func (f *ProviderFilter) Apply(providers []catalogs.Provider) []catalogs.Provider {
	if f == nil || f.isEmpty() {
		return providers
	}
	
	var filtered []catalogs.Provider
	
	for _, provider := range providers {
		if f.matches(provider) {
			filtered = append(filtered, provider)
		}
	}
	
	return filtered
}

func (f *ProviderFilter) isEmpty() bool {
	return !f.HasClient && !f.Configured && f.Search == ""
}

func (f *ProviderFilter) matches(provider catalogs.Provider) bool {
	// Search filter
	if f.Search != "" && !f.matchesSearch(provider) {
		return false
	}
	
	// Additional filters can be added here
	
	return true
}

func (f *ProviderFilter) matchesSearch(provider catalogs.Provider) bool {
	search := strings.ToLower(f.Search)
	
	// Search in ID
	if strings.Contains(strings.ToLower(string(provider.ID)), search) {
		return true
	}
	
	// Search in name
	if strings.Contains(strings.ToLower(provider.Name), search) {
		return true
	}
	
	// Search in headquarters
	if provider.Headquarters != nil && strings.Contains(strings.ToLower(*provider.Headquarters), search) {
		return true
	}
	
	return false
}

// AuthorFilter applies filters to author lists
type AuthorFilter struct {
	Search string
}

// Apply filters a slice of authors
func (f *AuthorFilter) Apply(authors []catalogs.Author) []catalogs.Author {
	if f == nil || f.isEmpty() {
		return authors
	}
	
	var filtered []catalogs.Author
	
	for _, author := range authors {
		if f.matches(author) {
			filtered = append(filtered, author)
		}
	}
	
	return filtered
}

func (f *AuthorFilter) isEmpty() bool {
	return f.Search == ""
}

func (f *AuthorFilter) matches(author catalogs.Author) bool {
	if f.Search == "" {
		return true
	}
	
	search := strings.ToLower(f.Search)
	
	// Search in ID
	if strings.Contains(strings.ToLower(string(author.ID)), search) {
		return true
	}
	
	// Search in name
	if strings.Contains(strings.ToLower(author.Name), search) {
		return true
	}
	
	// Search in description
	if author.Description != nil && strings.Contains(strings.ToLower(*author.Description), search) {
		return true
	}
	
	return false
}