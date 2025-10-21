// Package provenance provides field-level tracking of data sources and modifications.
package provenance

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/types"
)

// Provenance tracks the origin and history of a field value.
type Provenance struct {
	Source        types.SourceID // Source that provided the value (e.g., "providers", "models_dev_git")
	Field         string         // Field path
	Value         any            // The actual value
	Timestamp     time.Time      // When the value was set
	Authority     float64        // Authority score (0.0 to 1.0)
	Confidence    float64        // Confidence in the value (0.0 to 1.0)
	Reason        string         // Reason for selecting this value
	PreviousValue any            // Previous value if updated
}

// Map tracks provenance for multiple resources.
type Map map[string][]Provenance // key is "resourceType:resourceID:fieldPath"

// Tracker manages provenance tracking during reconciliation.
type Tracker interface {
	// Track records provenance for a field
	Track(resourceType types.ResourceType, resourceID string, field string, history Provenance)

	// Find retrieves provenance for a specific field
	FindByField(resourceType types.ResourceType, resourceID string, field string) []Provenance

	// FindByResource retrieves all provenance for a resource
	FindByResource(resourceType types.ResourceType, resourceID string) map[string][]Provenance

	// Map returns the complete provenance map
	Map() Map

	// Clear removes all provenance data
	Clear()
}

// tracker is the default implementation.
type tracker struct {
	provenance Map
	enabled    bool
}

// NewTracker creates a new provenance tracker.
func NewTracker(enabled bool) Tracker {
	return &tracker{
		provenance: make(Map),
		enabled:    enabled,
	}
}

// Track records provenance for a field.
func (p *tracker) Track(resourceType types.ResourceType, resourceID string, field string, history Provenance) {
	if !p.enabled {
		return
	}

	key := p.makeKey(string(resourceType), resourceID, field)

	// Set timestamp if not provided
	if history.Timestamp.IsZero() {
		history.Timestamp = time.Now()
	}

	p.provenance[key] = append(p.provenance[key], history)
}

// Find retrieves provenance for a specific field.
func (p *tracker) FindByField(resourceType types.ResourceType, resourceID string, field string) []Provenance {
	if !p.enabled {
		return nil
	}

	key := p.makeKey(string(resourceType), resourceID, field)
	return p.provenance[key]
}

// GetResourceProvenance retrieves all provenance for a resource.
func (p *tracker) FindByResource(resourceType types.ResourceType, resourceID string) map[string][]Provenance {
	if !p.enabled {
		return nil
	}

	result := make(map[string][]Provenance)
	prefix := fmt.Sprintf("%s:%s:", string(resourceType), resourceID)

	for key, info := range p.provenance {
		if field, found := strings.CutPrefix(key, prefix); found {
			result[field] = info
		}
	}

	return result
}

// Map returns the complete provenance map.
func (p *tracker) Map() Map {
	if !p.enabled {
		return nil
	}

	// Return a copy to prevent external modification
	result := make(Map)
	for k, v := range p.provenance {
		result[k] = append([]Provenance{}, v...)
	}
	return result
}

// Clear removes all provenance data.
func (p *tracker) Clear() {
	p.provenance = make(Map)
}

// makeKey creates a unique key for provenance tracking.
func (p *tracker) makeKey(resourceType string, resourceID string, field string) string {
	return fmt.Sprintf("%s:%s:%s", resourceType, resourceID, field)
}

// Report generates a human-readable provenance report.
type Report struct {
	Resources map[string]ResourceProvenance // key is "resourceType:resourceID"
}

// ResourceProvenance contains provenance for a single resource.
type ResourceProvenance struct {
	Type   types.ResourceType // Resource type (e.g., "model", "provider", "author")
	ID     string
	Fields map[string]Field
}

// Field contains provenance history for a single field.
type Field struct {
	Current   Provenance     // Current value and its source
	History   []Provenance   // Historical values
	Conflicts []ConflictInfo // Any conflicts that were resolved
}

// ConflictInfo describes a conflict that was resolved.
type ConflictInfo struct {
	Sources        []types.SourceID // Sources that had conflicting values
	Values         []any            // The conflicting values
	Resolution     string           // How the conflict was resolved
	SelectedSource types.SourceID   // Which source was selected
}

// GenerateReport creates a provenance report from a Map.
func GenerateReport(provenance Map) *Report {
	report := &Report{
		Resources: make(map[string]ResourceProvenance),
	}

	// Group by resource
	for key, infos := range provenance {
		parts := strings.SplitN(key, ":", 3)
		if len(parts) != 3 {
			continue
		}

		resourceType := parts[0]
		resourceID := parts[1]
		field := parts[2]

		resourceKey := fmt.Sprintf("%s:%s", resourceType, resourceID)

		// Get or create resource provenance
		resource, exists := report.Resources[resourceKey]
		if !exists {
			resource = ResourceProvenance{
				Type:   types.ResourceType(resourceType),
				ID:     resourceID,
				Fields: make(map[string]Field),
			}
		}

		// Sort infos by timestamp (newest first)
		sort.Slice(infos, func(i, j int) bool {
			return infos[i].Timestamp.After(infos[j].Timestamp)
		})

		// Create field provenance
		fieldProv := Field{
			History: infos,
		}

		if len(infos) > 0 {
			fieldProv.Current = infos[0]
		}

		// Detect conflicts
		fieldProv.Conflicts = detectConflicts(infos)

		resource.Fields[field] = fieldProv
		report.Resources[resourceKey] = resource
	}

	return report
}

// detectConflicts identifies conflicts in provenance history.
func detectConflicts(infos []Provenance) []ConflictInfo {
	conflicts := []ConflictInfo{}

	// Group by timestamp to find simultaneous values
	byTime := make(map[int64][]Provenance)
	for _, info := range infos {
		timeKey := info.Timestamp.Unix()
		byTime[timeKey] = append(byTime[timeKey], info)
	}

	// Check each time group for conflicts
	for _, group := range byTime {
		if len(group) > 1 {
			conflict := ConflictInfo{
				Sources: []types.SourceID{},
				Values:  []any{},
			}

			// Find the selected source (highest authority)
			var selected Provenance
			maxAuthority := 0.0

			for _, info := range group {
				conflict.Sources = append(conflict.Sources, info.Source)
				conflict.Values = append(conflict.Values, info.Value)

				if info.Authority > maxAuthority {
					maxAuthority = info.Authority
					selected = info
				}
			}

			conflict.SelectedSource = selected.Source
			conflict.Resolution = selected.Reason

			conflicts = append(conflicts, conflict)
		}
	}

	return conflicts
}

// String generates a string representation of the provenance report.
func (r *Report) String() string {
	var sb strings.Builder

	sb.WriteString("Provenance Report\n")
	sb.WriteString("=================\n\n")

	// Sort resources for consistent output
	resourceKeys := make([]string, 0, len(r.Resources))
	for key := range r.Resources {
		resourceKeys = append(resourceKeys, key)
	}
	sort.Strings(resourceKeys)

	for _, key := range resourceKeys {
		resource := r.Resources[key]
		sb.WriteString(fmt.Sprintf("%s: %s\n", resource.Type, resource.ID))
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteString("\n")

		// Sort fields for consistent output
		var fieldKeys []string
		for field := range resource.Fields {
			fieldKeys = append(fieldKeys, field)
		}
		sort.Strings(fieldKeys)

		for _, field := range fieldKeys {
			fieldProv := resource.Fields[field]
			sb.WriteString(fmt.Sprintf("  %s:\n", field))
			sb.WriteString(fmt.Sprintf("    Current: %v (from %s)\n",
				fieldProv.Current.Value, fieldProv.Current.Source))

			if len(fieldProv.Conflicts) > 0 {
				sb.WriteString("    Conflicts:\n")
				for _, conflict := range fieldProv.Conflicts {
					sb.WriteString(fmt.Sprintf("      - Sources: %v\n", conflict.Sources))
					sb.WriteString(fmt.Sprintf("        Selected: %s\n", conflict.SelectedSource))
					sb.WriteString(fmt.Sprintf("        Reason: %s\n", conflict.Resolution))
				}
			}

			if len(fieldProv.History) > 1 {
				sb.WriteString("    History:\n")
				for i, info := range fieldProv.History {
					if i > 3 { // Limit history display
						sb.WriteString(fmt.Sprintf("      ... and %d more\n", len(fieldProv.History)-i))
						break
					}
					sb.WriteString(fmt.Sprintf("      - %v from %s at %s\n",
						info.Value, info.Source, info.Timestamp.Format("15:04:05")))
				}
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// Auditor validates provenance tracking.
type Auditor interface {
	// Audit checks provenance for completeness and consistency
	Audit(provenance Map) *AuditResult

	// ValidateAuthority ensures authority scores are valid
	ValidateAuthority(provenance Map) []string

	// CheckCoverage verifies all required fields have provenance
	CheckCoverage(provenance Map, requiredFields []string) []string
}

// AuditResult contains audit findings.
type AuditResult struct {
	Valid       bool
	Issues      []string
	Warnings    []string
	Coverage    float64  // Percentage of fields with provenance
	Conflicts   int      // Number of unresolved conflicts
	MissingData []string // Fields without provenance
}

// ProvenanceFile represents a provenance file stored on disk.
//nolint:revive // Name is intentionally descriptive for external clarity
type ProvenanceFile struct {
	Provenance Map `yaml:"provenance"`
}

// Load reads provenance data from a YAML file.
// Returns nil, nil if the file doesn't exist (not an error).
func Load(path string) (*ProvenanceFile, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	// Path is from catalog configuration, not user input
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to read provenance file: %w", err)
	}

	var pf ProvenanceFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("failed to parse provenance file: %w", err)
	}

	return &pf, nil
}
