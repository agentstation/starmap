package catalogs

import (
	"fmt"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
)

// Provenance is a concurrent-safe container for provenance data.
// It follows the same pattern as Authors, Models, and Providers containers,
// using RWMutex for thread safety and returning deep copies to prevent external modification.
type Provenance struct {
	mu         sync.RWMutex
	provenance provenance.Map
}

// ProvenanceOption defines a function that configures a Provenance instance.
type ProvenanceOption func(*Provenance)

// NewProvenance creates a new Provenance container with optional configuration.
func NewProvenance(opts ...ProvenanceOption) *Provenance {
	p := &Provenance{
		provenance: make(provenance.Map),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Map returns a deep copy of the provenance map.
// This ensures thread safety by preventing external modification of internal state.
func (p *Provenance) Map() provenance.Map {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return deep copy
	result := make(provenance.Map)
	for k, v := range p.provenance {
		result[k] = append([]provenance.Entry{}, v...)
	}
	return result
}

// Set replaces the entire provenance map with new data.
// The input map is deep copied to prevent external modification.
func (p *Provenance) Set(m provenance.Map) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Deep copy the input
	p.provenance = make(provenance.Map)
	for k, v := range m {
		p.provenance[k] = append([]provenance.Entry{}, v...)
	}
}

// Merge adds new provenance entries to existing data.
// This appends to existing keys rather than replacing them.
func (p *Provenance) Merge(m provenance.Map) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for k, v := range m {
		// Append to existing entries
		p.provenance[k] = append(p.provenance[k], v...)
	}
}

// Clear removes all provenance data.
func (p *Provenance) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear existing map instead of allocating new one
	for k := range p.provenance {
		delete(p.provenance, k)
	}
}

// Len returns the number of provenance entries.
func (p *Provenance) Len() int {
	p.mu.RLock()
	length := len(p.provenance)
	p.mu.RUnlock()
	return length
}

// FindByField retrieves provenance for a specific field of a resource.
// Returns nil if no provenance is found.
func (p *Provenance) FindByField(resourceType evidence.ResourceType, resourceID string, field string) []provenance.Entry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := newKey(resourceType, resourceID, field)
	if entries, found := p.provenance[key]; found {
		// Return a copy to prevent external modification
		return append([]provenance.Entry{}, entries...)
	}
	return nil
}

// FindByResource retrieves all provenance for a resource.
// Returns a map of field names to their provenance entries.
func (p *Provenance) FindByResource(resourceType evidence.ResourceType, resourceID string) map[string][]provenance.Entry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string][]provenance.Entry)
	prefix := fmt.Sprintf("%s:%s:", resourceType, resourceID)

	for key, entries := range p.provenance {
		if field, found := strings.CutPrefix(key, prefix); found {
			// Return a copy to prevent external modification
			result[field] = append([]provenance.Entry{}, entries...)
		}
	}

	return result
}

// FindModelField retrieves provenance for one field of one provider model.
func (p *Provenance) FindModelField(providerID ProviderID, modelID, field string) []provenance.Entry {
	return p.FindByField(
		evidence.ResourceTypeModel,
		provenance.ModelResourceID(string(providerID), modelID),
		field,
	)
}

// FindModel retrieves all provenance for one provider model.
func (p *Provenance) FindModel(providerID ProviderID, modelID string) map[string][]provenance.Entry {
	return p.FindByResource(
		evidence.ResourceTypeModel,
		provenance.ModelResourceID(string(providerID), modelID),
	)
}

// FormatYAML returns the provenance data formatted as YAML.
// This follows the same pattern as Authors and Providers containers.
func (p *Provenance) FormatYAML() string {
	formatted, _ := p.EncodeYAML()
	return formatted
}

// EncodeYAML returns provenance YAML or a typed parse error when evidence
// values cannot be represented safely.
func (p *Provenance) EncodeYAML() (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Wrap in the provenance file structure for a consistent file format.
	pf := provenance.File{
		Provenance: p.provenance,
	}

	data, err := yaml.Marshal(pf)
	if err != nil {
		return "", errors.WrapParse("yaml", "provenance", err)
	}

	return string(data), nil
}

// newKey returns a unique key for provenance tracking.
// Format: "resourceType:resourceID:field".
func newKey(resourceType evidence.ResourceType, resourceID string, field string) string {
	return fmt.Sprintf("%s:%s:%s", resourceType, resourceID, field)
}
