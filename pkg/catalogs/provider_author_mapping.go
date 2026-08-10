package catalogs

import (
	"path"
	"sort"
	"strings"
)

// Resolve returns the configured author for one exact provider field value.
// Exact and case-insensitive matches precede the most-specific glob pattern.
func (m AuthorMapping) Resolve(value string) (AuthorID, bool) {
	if value == "" || len(m.Normalized) == 0 {
		return AuthorIDUnknown, false
	}
	if authorID, found := m.Normalized[value]; found {
		return authorID, true
	}

	valueLower := strings.ToLower(value)
	patterns := make([]string, 0, len(m.Normalized))
	for candidate := range m.Normalized {
		if strings.ToLower(candidate) == valueLower {
			return m.Normalized[candidate], true
		}
		if strings.ContainsAny(candidate, "*?[") {
			patterns = append(patterns, candidate)
		}
	}
	sort.Slice(patterns, func(left, right int) bool {
		if len(patterns[left]) == len(patterns[right]) {
			return patterns[left] < patterns[right]
		}
		return len(patterns[left]) > len(patterns[right])
	})
	for _, pattern := range patterns {
		matched, err := path.Match(strings.ToLower(pattern), valueLower)
		if err == nil && matched {
			return m.Normalized[pattern], true
		}
	}
	return AuthorIDUnknown, false
}
