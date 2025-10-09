package catalogs

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/errors"
)

// Providers is a concurrent safe map of providers.
type Providers struct {
	mu        sync.RWMutex
	providers map[ProviderID]*Provider
}

// ProvidersOption defines a function that configures a Providers instance.
type ProvidersOption func(*Providers)

// WithProvidersCapacity sets the initial capacity of the providers map.
func WithProvidersCapacity(capacity int) ProvidersOption {
	return func(p *Providers) {
		p.providers = make(map[ProviderID]*Provider, capacity)
	}
}

// WithProvidersMap initializes the map with existing providers.
func WithProvidersMap(providers map[ProviderID]*Provider) ProvidersOption {
	return func(p *Providers) {
		if providers != nil {
			p.providers = make(map[ProviderID]*Provider, len(providers))
			maps.Copy(p.providers, providers)
		}
	}
}

// NewProviders creates a new Providers map with optional configuration.
func NewProviders(opts ...ProvidersOption) *Providers {
	p := &Providers{
		providers: make(map[ProviderID]*Provider),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Get returns a provider by id and whether it exists.
func (p *Providers) Get(id ProviderID) (*Provider, bool) {
	p.mu.RLock()
	provider, ok := p.providers[id]
	p.mu.RUnlock()
	return provider, ok
}

// Set sets a provider by id. Returns an error if provider is nil.
func (p *Providers) Set(id ProviderID, provider *Provider) error {
	if provider == nil {
		return &errors.ValidationError{
			Field:   "provider",
			Message: "cannot be nil",
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.providers[id] = provider
	return nil
}

// Add adds a provider, returning an error if it already exists.
func (p *Providers) Add(provider *Provider) error {
	if provider == nil {
		return &errors.ValidationError{
			Field:   "provider",
			Message: "cannot be nil",
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.providers[provider.ID]; exists {
		return &errors.ValidationError{
			Field:   "provider.ID",
			Value:   provider.ID,
			Message: "already exists",
		}
	}

	p.providers[provider.ID] = provider
	return nil
}

// Delete removes a provider by id. Returns an error if the provider doesn't exist.
func (p *Providers) Delete(id ProviderID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.providers[id]; !exists {
		return &errors.NotFoundError{
			Resource: "provider",
			ID:       string(id),
		}
	}

	delete(p.providers, id)
	return nil
}

// Exists checks if a provider exists without returning it.
func (p *Providers) Exists(id ProviderID) bool {
	p.mu.RLock()
	_, exists := p.providers[id]
	p.mu.RUnlock()
	return exists
}

// Len returns the number of providers.
func (p *Providers) Len() int {
	p.mu.RLock()
	length := len(p.providers)
	p.mu.RUnlock()
	return length
}

// List returns a slice of all providers as values (copies).
func (p *Providers) List() []Provider {
	p.mu.RLock()
	providers := make([]Provider, 0, len(p.providers))
	for _, provider := range p.providers {
		// Return deep copies to prevent external modification
		providers = append(providers, DeepCopyProvider(*provider))
	}
	p.mu.RUnlock()

	// Sort by ID for deterministic ordering
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].ID < providers[j].ID
	})

	return providers
}

// Map returns a copy of all providers.
func (p *Providers) Map() map[ProviderID]*Provider {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[ProviderID]*Provider, len(p.providers))
	maps.Copy(result, p.providers)
	return result
}

// ForEach applies a function to each provider. The function should not modify the provider.
// If the function returns false, iteration stops early.
func (p *Providers) ForEach(fn func(id ProviderID, provider *Provider) bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for id, provider := range p.providers {
		if !fn(id, provider) {
			break
		}
	}
}

// Clear removes all providers.
func (p *Providers) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Clear existing map instead of allocating new one
	for k := range p.providers {
		delete(p.providers, k)
	}
}

// SetModel adds or updates a model in a provider.
func (p *Providers) SetModel(providerID ProviderID, model Model) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	provider, exists := p.providers[providerID]
	if !exists {
		return &errors.NotFoundError{
			Resource: "provider",
			ID:       string(providerID),
		}
	}

	// Ensure model has a name - use ID as fallback
	if model.Name == "" {
		model.Name = model.ID
	}

	// Create a copy of the provider to avoid modifying the original
	providerCopy := *provider
	if providerCopy.Models == nil {
		providerCopy.Models = make(map[string]*Model)
	} else {
		// Shallow copy the models map (we'll replace one entry)
		providerCopy.Models = ShallowCopyProviderModels(providerCopy.Models)
	}

	providerCopy.Models[model.ID] = &model
	p.providers[providerID] = &providerCopy

	return nil
}

// DeleteModel removes a model from a provider.
func (p *Providers) DeleteModel(providerID ProviderID, modelID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	provider, exists := p.providers[providerID]
	if !exists {
		return &errors.NotFoundError{
			Resource: "provider",
			ID:       string(providerID),
		}
	}

	if len(provider.Models) == 0 {
		return &errors.NotFoundError{
			Resource: "model",
			ID:       modelID,
		}
	}

	if _, exists := provider.Models[modelID]; !exists {
		return &errors.NotFoundError{
			Resource: "model",
			ID:       modelID,
		}
	}

	// Create a copy of the provider to avoid modifying the original
	providerCopy := *provider
	newModels := make(map[string]*Model, len(providerCopy.Models)-1)
	for k, v := range providerCopy.Models {
		if k != modelID {
			newModels[k] = v
		}
	}
	providerCopy.Models = newModels
	p.providers[providerID] = &providerCopy

	return nil
}

// AddBatch adds multiple providers in a single operation.
// Only adds providers that don't already exist - fails if a provider ID already exists.
// Returns a map of provider IDs to errors for any failed additions.
func (p *Providers) AddBatch(providers []*Provider) map[ProviderID]error {
	if len(providers) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	errs := make(map[ProviderID]error)

	// First pass: validate all providers
	for _, provider := range providers {
		if provider == nil {
			continue // Skip nil providers
		}
		if _, exists := p.providers[provider.ID]; exists {
			errs[provider.ID] = &errors.ValidationError{
				Field:   "provider.ID",
				Value:   provider.ID,
				Message: "already exists",
			}
		}
	}

	// Second pass: add valid providers
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		if _, hasError := errs[provider.ID]; !hasError {
			p.providers[provider.ID] = provider
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

// SetBatch sets multiple providers in a single operation.
// Overwrites existing providers or adds new ones (upsert behavior).
// Returns an error if any provider is nil.
func (p *Providers) SetBatch(providers map[ProviderID]*Provider) error {
	if len(providers) == 0 {
		return nil
	}

	// Validate all providers first
	for id, provider := range providers {
		if provider == nil {
			return &errors.ValidationError{
				Field:   "providers[" + string(id) + "]",
				Message: "cannot be nil",
			}
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for id, provider := range providers {
		p.providers[id] = provider
	}

	return nil
}

// DeleteBatch removes multiple providers by their IDs.
// Returns a map of errors for providers that couldn't be deleted (not found).
func (p *Providers) DeleteBatch(ids []ProviderID) map[ProviderID]error {
	if len(ids) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	errs := make(map[ProviderID]error)
	for _, id := range ids {
		if _, exists := p.providers[id]; !exists {
			errs[id] = &errors.NotFoundError{
				Resource: "provider",
				ID:       string(id),
			}
		} else {
			delete(p.providers, id)
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

// FormatYAML returns the providers as formatted YAML with enhanced formatting, comments, and structure.
func (p *Providers) FormatYAML() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Convert map to slice for consistent ordering
	providers := make([]Provider, 0, len(p.providers))

	// Get all provider IDs and sort them for deterministic output
	ids := make([]ProviderID, 0, len(p.providers))
	for id := range p.providers {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return string(ids[i]) < string(ids[j])
	})

	// Create ordered slice of providers
	for _, id := range ids {
		if provider := p.providers[id]; provider != nil {
			// Create a copy to avoid modifying the original
			providerCopy := *provider
			providers = append(providers, providerCopy)
		}
	}

	return formatProvidersYAML(providers)
}

// formatProvidersYAML formats a slice of providers with proper formatting, comments, and spacing.
func formatProvidersYAML(providers []Provider) string {
	// Create comment map for provider section headers and duration comments
	commentMap := yaml.CommentMap{}

	for i, provider := range providers {
		// Add provider section header comment using HeadComment with space formatting
		providerPath := fmt.Sprintf("$[%d]", i)
		commentMap[providerPath] = []*yaml.Comment{
			yaml.HeadComment(fmt.Sprintf(" %s", provider.Name)), // Space prefix for proper formatting
		}

		// Add duration comments
		if provider.RetentionPolicy != nil && provider.RetentionPolicy.Duration != nil {
			retentionPath := fmt.Sprintf("$[%d].retention_policy.duration", i)
			duration := provider.RetentionPolicy.Duration.String()
			var comment string
			switch duration {
			case "720h0m0s", "720h":
				comment = "30 days"
			case "48h0m0s", "48h":
				comment = "2 days"
			case "0s":
				comment = "immediate"
			default:
				// Handle other common durations
				if d := *provider.RetentionPolicy.Duration; d > 0 {
					hours := int(d.Hours())
					if hours >= 24 && hours%24 == 0 {
						days := hours / 24
						comment = fmt.Sprintf("%d days", days)
					}
				}
			}

			if comment != "" {
				commentMap[retentionPath] = []*yaml.Comment{
					yaml.LineComment(comment),
				}
			}
		}
	}

	// Let the library handle the formatting properly
	yamlData, err := yaml.MarshalWithOptions(providers,
		yaml.Indent(2),               // 2-space indentation
		yaml.IndentSequence(false),   // Keep root array flush left (no indentation)
		yaml.WithComment(commentMap), // Add comments
	)
	if err != nil {
		// Fallback to basic YAML if enhanced formatting fails
		basicYaml, _ := yaml.Marshal(providers)
		return string(basicYaml)
	}

	// Post-process to add spacing between providers
	return addBlankLinesBetweenProviders(string(yamlData))
}

// addBlankLinesBetweenProviders adds spacing between provider sections.
func addBlankLinesBetweenProviders(yamlContent string) string {
	lines := strings.Split(yamlContent, "\n")
	result := make([]string, 0, len(lines)+len(lines)/10) // Add extra space for blank lines

	for i, line := range lines {
		// Add blank line before each provider comment (except the first one)
		if strings.HasPrefix(line, "#") && i > 0 {
			result = append(result, "")
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}
