package table

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/provenance"
)

// ProvenanceToTableData converts provenance history to table format.
// Shows all fields and their history in a single unified table.
func ProvenanceToTableData(fieldProvenance map[string][]provenance.Provenance) Data {
	var rows [][]string

	// Sort fields alphabetically
	fields := make([]string, 0, len(fieldProvenance))
	for field := range fieldProvenance {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	// Build unified table with all fields
	for _, field := range fields {
		history := fieldProvenance[field]
		if len(history) == 0 {
			continue
		}

		// Sort by timestamp (newest first)
		sortedHistory := make([]provenance.Provenance, len(history))
		copy(sortedHistory, history)
		sort.Slice(sortedHistory, func(i, j int) bool {
			return sortedHistory[i].Timestamp.After(sortedHistory[j].Timestamp)
		})

		// Add each history entry for this field
		for i, entry := range sortedHistory {
			// Field name only on first row, blank for subsequent entries
			fieldName := ""
			if i == 0 {
				fieldName = field
			}

			// Current indicator (→ for newest entry)
			currentIndicator := ""
			if i == 0 {
				currentIndicator = "→"
			}

			// Format value as YAML for readability
			valueStr := formatValueAsYAML(entry.Value)

			// Format authority and confidence
			authorityStr := formatAuthority(entry.Authority)
			confidenceStr := fmt.Sprintf("%.0f%%", entry.Confidence*100)

			// Format timestamp
			whenStr := formatTimestamp(entry.Timestamp)

			// Reason (can be empty)
			reasonStr := entry.Reason

			// Add row
			rows = append(rows, []string{
				fieldName,
				currentIndicator,
				valueStr,
				string(entry.Source),
				authorityStr,
				confidenceStr,
				whenStr,
				reasonStr,
			})
		}
	}

	return Data{
		Headers: []string{"Field", "Curr", "Value", "Source", "Authority", "Confidence", "When", "Reason"},
		Rows:    rows,
		ColumnAlignment: []Align{
			AlignLeft,   // Field
			AlignCenter, // Curr
			AlignLeft,   // Value
			AlignLeft,   // Source
			AlignRight,  // Authority
			AlignRight,  // Confidence
			AlignLeft,   // When
			AlignLeft,   // Reason
		},
	}
}

// MatchField checks if a field matches any of the provided patterns.
// Supports wildcard matching (e.g., "pricing.*" matches "Pricing.Tokens.Input").
// Matching is case-insensitive for better user experience.
func MatchField(field string, patterns []string) bool {
	if len(patterns) == 0 {
		return true // No patterns means match all
	}

	// Convert field to lowercase for case-insensitive matching
	fieldLower := strings.ToLower(field)

	for _, pattern := range patterns {
		// Convert pattern to lowercase for case-insensitive matching
		patternLower := strings.ToLower(pattern)

		matched, err := filepath.Match(patternLower, fieldLower)
		if err == nil && matched {
			return true
		}

		// Also support prefix matching for patterns like "pricing.*"
		if strings.HasSuffix(patternLower, ".*") {
			prefix := strings.TrimSuffix(patternLower, ".*")
			if strings.HasPrefix(fieldLower, prefix+".") || fieldLower == prefix {
				return true
			}
		}
	}

	return false
}

// formatValueAsYAML formats a provenance value as YAML for display.
// Complex values (maps, slices, structs) are formatted as multi-line YAML.
// Simple values (strings, numbers, bools) are kept as-is.
func formatValueAsYAML(val any) string {
	if val == nil {
		return "<nil>"
	}

	// Handle simple types directly
	switch v := val.(type) {
	case string:
		if v == "" {
			return "<empty>"
		}
		return v
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		// Format numbers nicely
		if fval, ok := v.(float64); ok {
			if fval == float64(int64(fval)) {
				return fmt.Sprintf("%d", int64(fval))
			}
			return fmt.Sprintf("%.2f", fval)
		}
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%t", v)
	}

	// For complex types, use YAML formatting
	yamlBytes, err := yaml.Marshal(val)
	if err != nil {
		// Fall back to simple string representation
		return fmt.Sprintf("%v", val)
	}

	// Convert to string and remove trailing newline
	yamlStr := strings.TrimSuffix(string(yamlBytes), "\n")

	// If it's a simple single-line value, return as-is
	if !strings.Contains(yamlStr, "\n") {
		return yamlStr
	}

	// For multi-line values, return with proper formatting
	return yamlStr
}

// formatAuthority formats an authority score as a percentage.
func formatAuthority(authority float64) string {
	return fmt.Sprintf("%.0f%%", authority*100)
}

// formatTimestamp formats a timestamp for display.
func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "-"
	}

	// Show relative time for recent timestamps
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		mins := int(diff.Minutes())
		return fmt.Sprintf("%d min ago", mins)
	}
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return fmt.Sprintf("%d hr ago", hours)
	}
	if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	}

	// For older timestamps, show the date
	return t.Format("2006-01-02 15:04")
}
