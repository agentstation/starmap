package reconcile

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ProvenanceInfo tracks the origin and history of a field value
type ProvenanceInfo struct {
	Source      SourceName    // Source that provided the value
	Field       string        // Field path
	Value       interface{}   // The actual value
	Timestamp   time.Time     // When the value was set
	Authority   float64       // Authority score (0.0 to 1.0)
	Confidence  float64       // Confidence in the value (0.0 to 1.0)
	Reason      string        // Reason for selecting this value
	PreviousValue interface{} // Previous value if updated
}

// ProvenanceMap tracks provenance for multiple resources
type ProvenanceMap map[string][]ProvenanceInfo // key is "resourceType:resourceID:fieldPath"

// ProvenanceTracker manages provenance tracking during reconciliation
type ProvenanceTracker interface {
	// Track records provenance for a field
	Track(resourceType ResourceType, resourceID string, field string, info ProvenanceInfo)

	// GetProvenance retrieves provenance for a specific field
	GetProvenance(resourceType ResourceType, resourceID string, field string) []ProvenanceInfo

	// GetResourceProvenance retrieves all provenance for a resource
	GetResourceProvenance(resourceType ResourceType, resourceID string) map[string][]ProvenanceInfo

	// Export returns the complete provenance map
	Export() ProvenanceMap

	// Clear removes all provenance data
	Clear()
}

// provenanceTracker is the default implementation
type provenanceTracker struct {
	provenance ProvenanceMap
	enabled    bool
}

// NewProvenanceTracker creates a new provenance tracker
func NewProvenanceTracker(enabled bool) ProvenanceTracker {
	return &provenanceTracker{
		provenance: make(ProvenanceMap),
		enabled:    enabled,
	}
}

// Track records provenance for a field
func (p *provenanceTracker) Track(resourceType ResourceType, resourceID string, field string, info ProvenanceInfo) {
	if !p.enabled {
		return
	}

	key := p.makeKey(resourceType, resourceID, field)
	
	// Set timestamp if not provided
	if info.Timestamp.IsZero() {
		info.Timestamp = time.Now()
	}
	
	p.provenance[key] = append(p.provenance[key], info)
}

// GetProvenance retrieves provenance for a specific field
func (p *provenanceTracker) GetProvenance(resourceType ResourceType, resourceID string, field string) []ProvenanceInfo {
	if !p.enabled {
		return nil
	}

	key := p.makeKey(resourceType, resourceID, field)
	return p.provenance[key]
}

// GetResourceProvenance retrieves all provenance for a resource
func (p *provenanceTracker) GetResourceProvenance(resourceType ResourceType, resourceID string) map[string][]ProvenanceInfo {
	if !p.enabled {
		return nil
	}

	result := make(map[string][]ProvenanceInfo)
	prefix := fmt.Sprintf("%s:%s:", resourceType, resourceID)
	
	for key, info := range p.provenance {
		if strings.HasPrefix(key, prefix) {
			field := strings.TrimPrefix(key, prefix)
			result[field] = info
		}
	}
	
	return result
}

// Export returns the complete provenance map
func (p *provenanceTracker) Export() ProvenanceMap {
	if !p.enabled {
		return nil
	}
	
	// Return a copy to prevent external modification
	result := make(ProvenanceMap)
	for k, v := range p.provenance {
		result[k] = append([]ProvenanceInfo{}, v...)
	}
	return result
}

// Clear removes all provenance data
func (p *provenanceTracker) Clear() {
	p.provenance = make(ProvenanceMap)
}

// makeKey creates a unique key for provenance tracking
func (p *provenanceTracker) makeKey(resourceType ResourceType, resourceID string, field string) string {
	return fmt.Sprintf("%s:%s:%s", resourceType, resourceID, field)
}

// ProvenanceReport generates a human-readable provenance report
type ProvenanceReport struct {
	Resources map[string]ResourceProvenance // key is "resourceType:resourceID"
}

// ResourceProvenance contains provenance for a single resource
type ResourceProvenance struct {
	Type   ResourceType
	ID     string
	Fields map[string]FieldProvenance
}

// FieldProvenance contains provenance history for a single field
type FieldProvenance struct {
	Current  ProvenanceInfo   // Current value and its source
	History  []ProvenanceInfo // Historical values
	Conflicts []ConflictInfo   // Any conflicts that were resolved
}

// ConflictInfo describes a conflict that was resolved
type ConflictInfo struct {
	Sources     []SourceName  // Sources that had conflicting values
	Values      []interface{} // The conflicting values
	Resolution  string        // How the conflict was resolved
	SelectedSource SourceName // Which source was selected
}

// GenerateReport creates a provenance report from a ProvenanceMap
func GenerateReport(provenance ProvenanceMap) *ProvenanceReport {
	report := &ProvenanceReport{
		Resources: make(map[string]ResourceProvenance),
	}

	// Group by resource
	for key, infos := range provenance {
		parts := strings.SplitN(key, ":", 3)
		if len(parts) != 3 {
			continue
		}
		
		resourceType := ResourceType(parts[0])
		resourceID := parts[1]
		field := parts[2]
		
		resourceKey := fmt.Sprintf("%s:%s", resourceType, resourceID)
		
		// Get or create resource provenance
		resource, exists := report.Resources[resourceKey]
		if !exists {
			resource = ResourceProvenance{
				Type:   resourceType,
				ID:     resourceID,
				Fields: make(map[string]FieldProvenance),
			}
		}
		
		// Sort infos by timestamp (newest first)
		sort.Slice(infos, func(i, j int) bool {
			return infos[i].Timestamp.After(infos[j].Timestamp)
		})
		
		// Create field provenance
		fieldProv := FieldProvenance{
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

// detectConflicts identifies conflicts in provenance history
func detectConflicts(infos []ProvenanceInfo) []ConflictInfo {
	conflicts := []ConflictInfo{}
	
	// Group by timestamp to find simultaneous values
	byTime := make(map[int64][]ProvenanceInfo)
	for _, info := range infos {
		timeKey := info.Timestamp.Unix()
		byTime[timeKey] = append(byTime[timeKey], info)
	}
	
	// Check each time group for conflicts
	for _, group := range byTime {
		if len(group) > 1 {
			conflict := ConflictInfo{
				Sources: []SourceName{},
				Values:  []interface{}{},
			}
			
			// Find the selected source (highest authority)
			var selected ProvenanceInfo
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

// String generates a string representation of the provenance report
func (r *ProvenanceReport) String() string {
	var sb strings.Builder
	
	sb.WriteString("Provenance Report\n")
	sb.WriteString("=================\n\n")
	
	// Sort resources for consistent output
	var resourceKeys []string
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

// ProvenanceAuditor validates provenance tracking
type ProvenanceAuditor interface {
	// Audit checks provenance for completeness and consistency
	Audit(provenance ProvenanceMap) *ProvenanceAuditResult
	
	// ValidateAuthority ensures authority scores are valid
	ValidateAuthority(provenance ProvenanceMap) []string
	
	// CheckCoverage verifies all required fields have provenance
	CheckCoverage(provenance ProvenanceMap, requiredFields []string) []string
}

// ProvenanceAuditResult contains audit findings
type ProvenanceAuditResult struct {
	Valid       bool
	Issues      []string
	Warnings    []string
	Coverage    float64 // Percentage of fields with provenance
	Conflicts   int     // Number of unresolved conflicts
	MissingData []string // Fields without provenance
}