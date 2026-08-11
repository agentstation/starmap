package catalogs

import (
	"path"
	"sort"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// Validate checks the transport-independent author-normalization contract.
// Transport adapters separately validate which source fields they support.
func (m AuthorMapping) Validate() error {
	if strings.TrimSpace(m.Field) == "" {
		return &errors.ValidationError{
			Field: "author_mapping.field", Value: m.Field, Message: "is required",
		}
	}
	if len(m.Normalized) == 0 {
		return &errors.ValidationError{
			Field: "author_mapping.normalized", Message: "must contain at least one exact or glob mapping",
		}
	}
	caseFolded := make(map[string]string, len(m.Normalized))
	for candidate, authorID := range m.Normalized {
		if strings.TrimSpace(candidate) == "" {
			return &errors.ValidationError{
				Field: "author_mapping.normalized", Value: candidate, Message: "mapping key must not be empty",
			}
		}
		if strings.TrimSpace(string(authorID)) == "" {
			return &errors.ValidationError{
				Field: "author_mapping.normalized", Value: candidate, Message: "canonical author target must not be empty",
			}
		}
		folded := strings.ToLower(candidate)
		if existing, found := caseFolded[folded]; found {
			return &errors.ValidationError{
				Field: "author_mapping.normalized", Value: candidate,
				Message: "case-insensitive duplicate of " + existing,
			}
		}
		caseFolded[folded] = candidate
		if strings.ContainsAny(candidate, "*?[") {
			if _, err := path.Match(candidate, ""); err != nil {
				return &errors.ValidationError{
					Field: "author_mapping.normalized", Value: candidate, Message: "invalid glob: " + err.Error(),
				}
			}
		}
	}
	return nil
}

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
