// Package provenance provides field-level tracking of data sources and modifications.
package provenance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/errors"
)

// Entry records the origin and history of one field value.
type Entry struct {
	Source           evidence.SourceID            // Source that provided the value (e.g., "providers", "models_dev_git")
	Field            string                       // Field path
	Value            any                          // The actual value
	Timestamp        time.Time                    // When the value was set
	ObservationID    string                       // Stable identity of the winning source observation
	ObservedAt       time.Time                    // When the winning source was observed
	Revision         evidence.ObservationRevision // Exact revision of the winning observation
	EvidenceChecksum string                       // Digest binding the winning normalized evidence
	Rejections       []Rejection                  // Higher-authority observations rejected before selection
	Authority        float64                      // Authority score (0.0 to 1.0)
	Confidence       float64                      // Confidence in the value (0.0 to 1.0)
	Reason           string                       // Reason for selecting this value
	PreviousValue    any                          // Previous value if updated
}

// MarshalJSON makes interface-valued evidence independent of the concrete Go
// type used to construct it. This is required for immutable catalog payloads:
// after a payload is decoded, Value and PreviousValue contain generic JSON
// maps rather than the original source structs, but re-encoding must reproduce
// the exact generation bytes.
func (e Entry) MarshalJSON() ([]byte, error) {
	value, err := canonicalDynamicJSON(e.Value)
	if err != nil {
		return nil, fmt.Errorf("encode provenance value: %w", err)
	}
	previousValue, err := canonicalDynamicJSON(e.PreviousValue)
	if err != nil {
		return nil, fmt.Errorf("encode provenance previous value: %w", err)
	}
	type canonicalEntry struct {
		Source           evidence.SourceID
		Field            string
		Value            json.RawMessage
		Timestamp        time.Time
		ObservationID    string
		ObservedAt       time.Time
		Revision         evidence.ObservationRevision
		EvidenceChecksum string
		Rejections       []Rejection
		Authority        float64
		Confidence       float64
		Reason           string
		PreviousValue    json.RawMessage
	}
	rejections := make([]Rejection, 0, len(e.Rejections))
	if len(e.Rejections) > 0 {
		rejections = append(rejections, e.Rejections...)
	}
	return json.Marshal(canonicalEntry{
		Source: e.Source, Field: e.Field, Value: value, Timestamp: e.Timestamp,
		ObservationID: e.ObservationID, ObservedAt: e.ObservedAt, Revision: e.Revision,
		EvidenceChecksum: e.EvidenceChecksum, Rejections: rejections,
		Authority: e.Authority, Confidence: e.Confidence, Reason: e.Reason,
		PreviousValue: previousValue,
	})
}

// MarshalYAML uses the same canonical dynamic-value shape as MarshalJSON so a
// human workspace can reproduce the exact immutable catalog payload.
func (e Entry) MarshalYAML() ([]byte, error) {
	value, err := canonicalDynamicJSON(e.Value)
	if err != nil {
		return nil, fmt.Errorf("encode provenance value: %w", err)
	}
	previousValue, err := canonicalDynamicJSON(e.PreviousValue)
	if err != nil {
		return nil, fmt.Errorf("encode provenance previous value: %w", err)
	}
	type canonicalRejection struct {
		Source evidence.SourceID `json:"source"`
		Reason string            `json:"reason"`
	}
	rejections := make([]canonicalRejection, 0, len(e.Rejections))
	for _, rejection := range e.Rejections {
		rejections = append(rejections, canonicalRejection(rejection))
	}
	type canonicalEntry struct {
		Source           evidence.SourceID            `json:"source"`
		Field            string                       `json:"field"`
		Value            json.RawMessage              `json:"value"`
		Timestamp        time.Time                    `json:"timestamp"`
		ObservationID    string                       `json:"observationid"`
		ObservedAt       time.Time                    `json:"observedat"`
		Revision         evidence.ObservationRevision `json:"revision"`
		EvidenceChecksum string                       `json:"evidencechecksum"`
		Rejections       []canonicalRejection         `json:"rejections"`
		Authority        float64                      `json:"authority"`
		Confidence       float64                      `json:"confidence"`
		Reason           string                       `json:"reason"`
		PreviousValue    json.RawMessage              `json:"previousvalue"`
	}
	encoded, err := json.Marshal(canonicalEntry{
		Source: e.Source, Field: e.Field, Value: value, Timestamp: e.Timestamp,
		ObservationID: e.ObservationID, ObservedAt: e.ObservedAt, Revision: e.Revision,
		EvidenceChecksum: e.EvidenceChecksum, Rejections: rejections,
		Authority: e.Authority, Confidence: e.Confidence, Reason: e.Reason,
		PreviousValue: previousValue,
	})
	if err != nil {
		return nil, err
	}
	yamlData, err := yaml.JSONToYAML(encoded)
	if err != nil {
		return nil, err
	}
	return expandScientificYAMLNumbers(yamlData), nil
}

func expandScientificYAMLNumbers(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	for index, line := range lines {
		trimmed := bytes.TrimSpace(line)
		scalarOffset := 0
		switch {
		case bytes.HasPrefix(trimmed, []byte("- ")):
			scalarOffset = 2
		case bytes.Contains(trimmed, []byte(": ")):
			scalarOffset = bytes.Index(trimmed, []byte(": ")) + 2
		}
		scalar := trimmed[scalarOffset:]
		if !bytes.ContainsAny(scalar, "eE") {
			continue
		}
		value, err := strconv.ParseFloat(string(scalar), 64)
		if err != nil {
			continue
		}
		plain := strconv.FormatFloat(value, 'f', -1, 64)
		start := len(line) - len(trimmed) + scalarOffset
		rewritten := make([]byte, 0, start+len(plain))
		rewritten = append(rewritten, line[:start]...)
		rewritten = append(rewritten, plain...)
		lines[index] = rewritten
	}
	return bytes.Join(lines, []byte("\n"))
}

func canonicalDynamicJSON(value any) (json.RawMessage, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

// Rejection records why a higher-authority field observation did not win.
type Rejection struct {
	Source evidence.SourceID // Source whose field observation was rejected
	Reason string            // Stable human-readable validation or applicability reason
}

// Map tracks provenance for multiple resources.
type Map map[string][]Entry // key is "resourceType:resourceID:fieldPath"

// Tracker records and queries field-level provenance.
type Tracker struct {
	provenance Map
	enabled    bool
}

// NewTracker creates a new provenance tracker.
func NewTracker(enabled bool) *Tracker {
	return &Tracker{
		provenance: make(Map),
		enabled:    enabled,
	}
}

// Track records provenance for a field.
func (p *Tracker) Track(resourceType evidence.ResourceType, resourceID string, field string, history Entry) {
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

// FindByField retrieves provenance for a specific field.
func (p *Tracker) FindByField(resourceType evidence.ResourceType, resourceID string, field string) []Entry {
	if !p.enabled {
		return nil
	}

	key := p.makeKey(string(resourceType), resourceID, field)
	return cloneProvenance(p.provenance[key])
}

// FindByResource retrieves all provenance for a resource.
func (p *Tracker) FindByResource(resourceType evidence.ResourceType, resourceID string) map[string][]Entry {
	if !p.enabled {
		return nil
	}

	result := make(map[string][]Entry)
	prefix := fmt.Sprintf("%s:%s:", string(resourceType), resourceID)

	for key, info := range p.provenance {
		if field, found := strings.CutPrefix(key, prefix); found {
			result[field] = cloneProvenance(info)
		}
	}

	return result
}

// Map returns the complete provenance map.
func (p *Tracker) Map() Map {
	if !p.enabled {
		return nil
	}

	// Return a copy to prevent external modification
	result := make(Map)
	for k, v := range p.provenance {
		result[k] = cloneProvenance(v)
	}
	return result
}

func cloneProvenance(source []Entry) []Entry {
	result := make([]Entry, len(source))
	copy(result, source)
	for index := range result {
		result[index].Rejections = append([]Rejection(nil), source[index].Rejections...)
	}
	return result
}

// Clear removes all provenance data.
func (p *Tracker) Clear() {
	p.provenance = make(Map)
}

// makeKey creates a unique key for provenance tracking.
func (p *Tracker) makeKey(resourceType string, resourceID string, field string) string {
	return fmt.Sprintf("%s:%s:%s", resourceType, resourceID, field)
}

// Report generates a human-readable provenance report.
type Report struct {
	Resources map[string]ResourceProvenance // key is "resourceType:resourceID"
}

// ResourceProvenance contains provenance for a single resource.
type ResourceProvenance struct {
	Type   evidence.ResourceType // Resource type (e.g., "model", "provider", "author")
	ID     string
	Fields map[string]Field
}

// Field contains provenance history for a single field.
type Field struct {
	Current   Entry          // Current value and its source
	History   []Entry        // Historical values
	Conflicts []ConflictInfo // Any conflicts that were resolved
}

// ConflictInfo describes a conflict that was resolved.
type ConflictInfo struct {
	Sources        []evidence.SourceID // Sources that had conflicting values
	Values         []any               // The conflicting values
	Resolution     string              // How the conflict was resolved
	SelectedSource evidence.SourceID   // Which source was selected
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
				Type:   evidence.ResourceType(resourceType),
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
func detectConflicts(infos []Entry) []ConflictInfo {
	conflicts := []ConflictInfo{}

	// Group by timestamp to find simultaneous values
	byTime := make(map[int64][]Entry)
	for _, info := range infos {
		timeKey := info.Timestamp.Unix()
		byTime[timeKey] = append(byTime[timeKey], info)
	}

	// Check each time group for conflicts
	for _, group := range byTime {
		if len(group) > 1 {
			conflict := ConflictInfo{
				Sources: []evidence.SourceID{},
				Values:  []any{},
			}

			// Find the selected source (highest authority)
			var selected Entry
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
		fmt.Fprintf(&sb, "%s: %s\n", resource.Type, resource.ID)
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteString("\n")

		// Sort fields for consistent output
		fieldKeys := make([]string, 0, len(resource.Fields))
		for field := range resource.Fields {
			fieldKeys = append(fieldKeys, field)
		}
		sort.Strings(fieldKeys)

		for _, field := range fieldKeys {
			fieldProv := resource.Fields[field]
			fmt.Fprintf(&sb, "  %s:\n", field)
			fmt.Fprintf(&sb, "    Current: %v (from %s)\n",
				fieldProv.Current.Value, fieldProv.Current.Source)

			if len(fieldProv.Conflicts) > 0 {
				sb.WriteString("    Conflicts:\n")
				for _, conflict := range fieldProv.Conflicts {
					fmt.Fprintf(&sb, "      - Sources: %v\n", conflict.Sources)
					fmt.Fprintf(&sb, "        Selected: %s\n", conflict.SelectedSource)
					fmt.Fprintf(&sb, "        Reason: %s\n", conflict.Resolution)
				}
			}

			if len(fieldProv.History) > 1 {
				sb.WriteString("    History:\n")
				for i, info := range fieldProv.History {
					if i > 3 { // Limit history display
						fmt.Fprintf(&sb, "      ... and %d more\n", len(fieldProv.History)-i)
						break
					}
					fmt.Fprintf(&sb, "      - %v from %s at %s\n",
						info.Value, info.Source, info.Timestamp.Format("15:04:05"))
				}
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
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

// File represents provenance stored on disk.
type File struct {
	Provenance Map `yaml:"provenance"`
}

// Load reads provenance data from a YAML file.
// Returns nil, nil if the file doesn't exist (not an error).
func Load(path string) (*File, error) {
	// Path is from catalog configuration, not user input
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errors.WrapIO("read", path, err)
	}

	var pf File
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, errors.WrapParse("yaml", path, err)
	}

	return &pf, nil
}
