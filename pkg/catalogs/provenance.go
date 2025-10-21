package catalogs

import (
	"fmt"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/types"
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
		result[k] = append([]provenance.Provenance{}, v...)
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
		p.provenance[k] = append([]provenance.Provenance{}, v...)
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
func (p *Provenance) FindByField(resourceType types.ResourceType, resourceID string, field string) []provenance.Provenance {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := newKey(resourceType, resourceID, field)
	if entries, found := p.provenance[key]; found {
		// Return a copy to prevent external modification
		return append([]provenance.Provenance{}, entries...)
	}
	return nil
}

// FindByResource retrieves all provenance for a resource.
// Returns a map of field names to their provenance entries.
func (p *Provenance) FindByResource(resourceType types.ResourceType, resourceID string) map[string][]provenance.Provenance {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string][]provenance.Provenance)
	prefix := fmt.Sprintf("%s:%s:", resourceType, resourceID)

	for key, entries := range p.provenance {
		if field, found := strings.CutPrefix(key, prefix); found {
			// Return a copy to prevent external modification
			result[field] = append([]provenance.Provenance{}, entries...)
		}
	}

	return result
}

// FormatYAML returns the provenance data formatted as YAML.
// This follows the same pattern as Authors and Providers containers.
func (p *Provenance) FormatYAML() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Wrap in ProvenanceFile structure for consistent file format
	pf := provenance.ProvenanceFile{
		Provenance: p.provenance,
	}

	data, err := yaml.Marshal(pf)
	if err != nil {
		// In practice this should never happen with valid provenance data
		// Return empty string rather than panicking
		return ""
	}

	return string(data)
}

// newKey returns a unique key for provenance tracking.
// Format: "resourceType:resourceID:field".
func newKey(resourceType types.ResourceType, resourceID string, field string) string {
	return fmt.Sprintf("%s:%s:%s", resourceType, resourceID, field)
}
