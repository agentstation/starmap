// Package matcher provides a unified interface for pattern matching using glob and regex patterns.
// It supports file path matching, string filtering, and batch operations with compile-time optimization.
package matcher

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// PatternType represents the type of pattern matching to use.
type PatternType int

const (
	// Glob uses shell-style glob patterns (*, ?, []).
	Glob PatternType = iota
	// Regex uses regular expressions.
	Regex
	// Auto attempts to detect the pattern type.
	Auto
)

// Matcher is the main interface for pattern matching operations.
type Matcher interface {
	// Match checks if the input matches the pattern
	Match(input string) bool
	// MatchAll checks multiple inputs and returns matches.
	MatchAll(inputs ...string) []string
	// MatchFirst returns the first matching input or empty string.
	MatchFirst(inputs ...string) string
	// MatchCount returns the number of matching inputs.
	MatchCount(inputs ...string) int
	// Pattern returns the original pattern string.
	Pattern() string
	// Type returns the pattern type being used.
	Type() PatternType
}

// matcher is the concrete implementation of the Matcher interface.
type matcher struct {
	pattern         string
	patternType     PatternType
	compiled        *regexp.Regexp
	globPattern     string
	caseInsensitive bool
	mu              sync.RWMutex
}

// Options configures the matcher behavior.
type Options struct {
	// CaseInsensitive makes matching case-insensitive
	CaseInsensitive bool
	// FilePath treats the pattern as a file path (affects glob behavior)
	FilePath bool
	// Anchored adds ^ and $ to regex patterns if not present
	Anchored bool
}

// DefaultOptions returns the default options.
func DefaultOptions() *Options {
	return &Options{
		CaseInsensitive: false,
		FilePath:        false,
		Anchored:        false,
	}
}

// New creates a new Matcher with the specified pattern and type.
func New(patternType PatternType, pattern string, opts ...*Options) (Matcher, error) {
	var options *Options
	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	} else {
		options = DefaultOptions()
	}

	m := &matcher{
		pattern:     pattern,
		patternType: patternType,
	}

	// Auto-detect pattern type if needed
	if patternType == Auto {
		m.patternType = detectPatternType(pattern)
	}

	// Apply options and compile pattern
	if err := m.compile(options); err != nil {
		return nil, fmt.Errorf("failed to compile pattern: %w", err)
	}

	return m, nil
}

// MustNew creates a new Matcher and panics if there's an error.
func MustNew(patternType PatternType, pattern string, opts ...*Options) Matcher {
	m, err := New(patternType, pattern, opts...)
	if err != nil {
		panic(err)
	}
	return m
}

// compile prepares the pattern for matching.
func (m *matcher) compile(opts *Options) error {
	m.caseInsensitive = opts.CaseInsensitive

	switch m.patternType {
	case Glob:
		m.globPattern = m.pattern
		if opts.CaseInsensitive {
			m.globPattern = strings.ToLower(m.globPattern)
		}
		// Validate glob pattern
		if _, err := filepath.Match(m.globPattern, ""); err != nil {
			return fmt.Errorf("invalid glob pattern: %w", err)
		}
	case Regex:
		pattern := m.pattern

		// Add anchors if requested
		if opts.Anchored {
			if !strings.HasPrefix(pattern, "^") {
				pattern = "^" + pattern
			}
			if !strings.HasSuffix(pattern, "$") {
				pattern = pattern + "$"
			}
		}

		// Add case-insensitive flag if needed
		if opts.CaseInsensitive && !strings.HasPrefix(pattern, "(?i)") {
			pattern = "(?i)" + pattern
		}

		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
		m.compiled = compiled
	default:
		return fmt.Errorf("unsupported pattern type: %v", m.patternType)
	}
	return nil
}

// Match checks if the input matches the pattern.
func (m *matcher) Match(input string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch m.patternType {
	case Glob:
		// Handle case-insensitive matching for glob
		compareInput := input
		if m.caseInsensitive {
			compareInput = strings.ToLower(input)
		}
		matched, _ := filepath.Match(m.globPattern, compareInput)
		return matched
	case Regex:
		return m.compiled.MatchString(input)
	default:
		return false
	}
}

// MatchAll checks multiple inputs and returns matches.
func (m *matcher) MatchAll(inputs ...string) []string {
	results := make([]string, 0)
	for _, input := range inputs {
		if m.Match(input) {
			results = append(results, input)
		}
	}
	return results
}

// MatchFirst returns the first matching input or empty string.
func (m *matcher) MatchFirst(inputs ...string) string {
	for _, input := range inputs {
		if m.Match(input) {
			return input
		}
	}
	return ""
}

// MatchCount returns the number of matching inputs.
func (m *matcher) MatchCount(inputs ...string) int {
	count := 0
	for _, input := range inputs {
		if m.Match(input) {
			count++
		}
	}
	return count
}

// Pattern returns the original pattern string.
func (m *matcher) Pattern() string {
	return m.pattern
}

// Type returns the pattern type being used.
func (m *matcher) Type() PatternType {
	return m.patternType
}

// detectPatternType attempts to detect if a pattern is glob or regex.
func detectPatternType(pattern string) PatternType {
	// Check for common regex metacharacters not used in glob
	regexIndicators := []string{
		"^", "$", "\\d", "\\w", "\\s", "\\D", "\\W", "\\S",
		"(?:", "(?i)", "(?m)", "(?s)",
		"{", "}", "+", "|", "(", ")",
	}

	for _, indicator := range regexIndicators {
		if strings.Contains(pattern, indicator) {
			return Regex
		}
	}

	// Check for glob-specific patterns
	if strings.ContainsAny(pattern, "*?[]") {
		return Glob
	}

	// Default to glob for simple strings
	return Glob
}

// String returns a string representation of the PatternType.
func (pt PatternType) String() string {
	switch pt {
	case Glob:
		return "glob"
	case Regex:
		return "regex"
	case Auto:
		return "auto"
	default:
		return "unknown"
	}
}

// MultiMatcher handles multiple patterns simultaneously.
type MultiMatcher struct {
	matchers []Matcher
	mu       sync.RWMutex
}

// NewMultiMatcher creates a matcher with multiple patterns.
func NewMultiMatcher(patterns []string, patternType PatternType, opts ...*Options) (*MultiMatcher, error) {
	mm := &MultiMatcher{
		matchers: make([]Matcher, 0, len(patterns)),
	}

	for _, pattern := range patterns {
		m, err := New(patternType, pattern, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create matcher for pattern %q: %w", pattern, err)
		}
		mm.matchers = append(mm.matchers, m)
	}

	return mm, nil
}

// Match returns true if any pattern matches.
func (mm *MultiMatcher) Match(input string) bool {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, m := range mm.matchers {
		if m.Match(input) {
			return true
		}
	}
	return false
}

// MatchAll returns all inputs that match any pattern.
func (mm *MultiMatcher) MatchAll(inputs ...string) []string {
	results := make([]string, 0)
	seen := make(map[string]bool)

	for _, input := range inputs {
		if !seen[input] && mm.Match(input) {
			results = append(results, input)
			seen[input] = true
		}
	}

	return results
}

// MatchAny returns true if any of the inputs match any pattern.
func (mm *MultiMatcher) MatchAny(inputs ...string) bool {
	for _, input := range inputs {
		if mm.Match(input) {
			return true
		}
	}
	return false
}

// AddMatcher adds a new matcher to the collection.
func (mm *MultiMatcher) AddMatcher(m Matcher) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.matchers = append(mm.matchers, m)
}

// Helper functions for common use cases.

// MatchFile checks if a file path matches the pattern.
func MatchFile(path, pattern string) (bool, error) {
	opts := &Options{FilePath: true}
	m, err := New(Auto, pattern, opts)
	if err != nil {
		return false, err
	}
	return m.Match(path), nil
}

// FilterStrings filters a slice of strings based on the pattern.
func FilterStrings(patternType PatternType, pattern string, items ...string) ([]string, error) {
	m, err := New(patternType, pattern)
	if err != nil {
		return nil, err
	}
	return m.MatchAll(items...), nil
}

// IsGlobPattern checks if a string contains glob metacharacters.
func IsGlobPattern(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[]")
}

// IsRegexPattern attempts to detect if a pattern is a regex.
func IsRegexPattern(pattern string) bool {
	return detectPatternType(pattern) == Regex
}

// GlobToRegex converts a glob pattern to a regex pattern.
func GlobToRegex(glob string) string {
	var regex strings.Builder
	regex.WriteString("^")

	for i := 0; i < len(glob); i++ {
		switch glob[i] {
		case '*':
			regex.WriteString(".*")
		case '?':
			regex.WriteString(".")
		case '[':
			// Handle character classes
			j := i + 1
			if j < len(glob) && (glob[j] == '!' || glob[j] == '^') {
				regex.WriteString("[^")
				j++
			} else {
				regex.WriteString("[")
			}

			for ; j < len(glob) && glob[j] != ']'; j++ {
				if glob[j] == '\\' {
					regex.WriteByte(glob[j])
					j++
					if j < len(glob) {
						regex.WriteByte(glob[j])
					}
				} else {
					regex.WriteByte(glob[j])
				}
			}

			if j < len(glob) {
				regex.WriteString("]")
				i = j
			}
		case '\\':
			if i+1 < len(glob) {
				i++
				regex.WriteString(regexp.QuoteMeta(string(glob[i])))
			}
		default:
			regex.WriteString(regexp.QuoteMeta(string(glob[i])))
		}
	}

	regex.WriteString("$")
	return regex.String()
}

// CompilePatterns pre-compiles multiple patterns for efficient matching.
func CompilePatterns(patterns map[string]PatternType, opts ...*Options) (map[string]Matcher, error) {
	compiled := make(map[string]Matcher, len(patterns))

	for pattern, patternType := range patterns {
		m, err := New(patternType, pattern, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pattern %q: %w", pattern, err)
		}
		compiled[pattern] = m
	}

	return compiled, nil
}
