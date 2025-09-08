// Package hints provides actionable user guidance for CLI operations.
package hints

import (
	"fmt"
	"strings"
)

// Hint represents actionable user guidance.
type Hint struct {
	Message string   // Human-readable guidance message
	Command string   // Optional specific command to run
	URL     string   // Optional documentation link
	Tags    []string // For context-aware filtering
}

// New creates a new hint with the given message.
func New(message string) *Hint {
	return &Hint{
		Message: message,
	}
}

// NewCommand creates a new hint with a specific command.
func NewCommand(message, command string) *Hint {
	return &Hint{
		Message: message,
		Command: command,
	}
}

// NewURL creates a new hint with a documentation URL.
func NewURL(message, url string) *Hint {
	return &Hint{
		Message: message,
		URL:     url,
	}
}

// WithCommand adds a command to the hint.
func (h *Hint) WithCommand(command string) *Hint {
	h.Command = command
	return h
}

// WithURL adds a URL to the hint.
func (h *Hint) WithURL(url string) *Hint {
	h.URL = url
	return h
}

// WithTags adds tags to the hint for context-aware filtering.
func (h *Hint) WithTags(tags ...string) *Hint {
	h.Tags = append(h.Tags, tags...)
	return h
}

// HasTag checks if the hint has a specific tag.
func (h *Hint) HasTag(tag string) bool {
	for _, t := range h.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// String returns a string representation of the hint.
func (h *Hint) String() string {
	var parts []string
	
	// Start with the message
	message := fmt.Sprintf("💡 %s", h.Message)
	parts = append(parts, message)
	
	// Add command if present
	if h.Command != "" {
		parts = append(parts, fmt.Sprintf("   Run: %s", h.Command))
	}
	
	// Add URL if present
	if h.URL != "" {
		parts = append(parts, fmt.Sprintf("   See: %s", h.URL))
	}
	
	return strings.Join(parts, "\n")
}

// Context provides information for generating contextual hints.
type Context struct {
	Command     string            // Current command being executed
	Subcommand  string            // Subcommand if applicable
	Succeeded   bool              // Whether the operation succeeded
	ErrorType   string            // Type of error if failed
	Args        []string          // Command arguments
	Flags       map[string]string // Command flags
	UserState   UserState         // Current user state
	Environment Environment       // Runtime environment
}

// UserState represents the current state of user configuration.
type UserState struct {
	AuthProviders    []string // Configured authentication providers
	HasConfig        bool     // Whether user has configuration file
	IsFirstRun       bool     // Whether this is the first time running
	LastCommand      string   // Last successful command
	ConfiguredOutput string   // User's preferred output format
}

// Environment represents the runtime environment.
type Environment struct {
	IsTerminal    bool   // Whether running in a terminal
	IsCI          bool   // Whether running in CI/CD
	Shell         string // Shell type (bash, zsh, etc.)
	OS            string // Operating system
	WorkingDir    string // Current working directory
	IsGitRepo     bool   // Whether in a git repository
}

// Provider generates contextual hints based on the current context.
type Provider interface {
	GetHints(ctx Context) []*Hint
	Name() string
}

// ProviderFunc is an adapter to allow functions to be used as Providers.
type ProviderFunc func(Context) []*Hint

// GetHints calls the function.
func (f ProviderFunc) GetHints(ctx Context) []*Hint {
	return f(ctx)
}

// Name returns the function name (generic).
func (f ProviderFunc) Name() string {
	return "func"
}

// Registry manages hint providers and generates contextual hints.
type Registry struct {
	providers []Provider
	config    RegistryConfig
}

// RegistryConfig configures hint generation behavior.
type RegistryConfig struct {
	MaxHints    int      // Maximum number of hints to return
	FilterTags  []string // Only include hints with these tags
	ExcludeTags []string // Exclude hints with these tags
	Enabled     bool     // Whether hints are enabled
}

// NewRegistry creates a new hint registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make([]Provider, 0),
		config: RegistryConfig{
			MaxHints: 3,
			Enabled:  true,
		},
	}
}

// WithConfig sets the registry configuration.
func (r *Registry) WithConfig(config RegistryConfig) *Registry {
	r.config = config
	return r
}

// Register adds a hint provider to the registry.
func (r *Registry) Register(provider Provider) {
	r.providers = append(r.providers, provider)
}

// RegisterFunc registers a function as a hint provider.
func (r *Registry) RegisterFunc(name string, fn func(Context) []*Hint) {
	provider := &namedProvider{
		name: name,
		fn:   ProviderFunc(fn),
	}
	r.Register(provider)
}

// GetHints generates hints for the given context.
func (r *Registry) GetHints(ctx Context) []*Hint {
	if !r.config.Enabled {
		return nil
	}
	
	var allHints []*Hint
	
	// Collect hints from all providers
	for _, provider := range r.providers {
		hints := provider.GetHints(ctx)
		allHints = append(allHints, hints...)
	}
	
	// Filter hints based on configuration
	filteredHints := r.filterHints(allHints)
	
	// Limit number of hints
	if r.config.MaxHints > 0 && len(filteredHints) > r.config.MaxHints {
		filteredHints = filteredHints[:r.config.MaxHints]
	}
	
	return filteredHints
}

// filterHints applies tag-based filtering to hints.
func (r *Registry) filterHints(hints []*Hint) []*Hint {
	var filtered []*Hint
	
	for _, hint := range hints {
		// Skip if hint has excluded tags
		excluded := false
		for _, excludeTag := range r.config.ExcludeTags {
			if hint.HasTag(excludeTag) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		
		// Include if no filter tags specified, or if hint has any filter tag
		if len(r.config.FilterTags) == 0 {
			filtered = append(filtered, hint)
			continue
		}
		
		for _, filterTag := range r.config.FilterTags {
			if hint.HasTag(filterTag) {
				filtered = append(filtered, hint)
				break
			}
		}
	}
	
	return filtered
}

// namedProvider wraps a ProviderFunc with a name.
type namedProvider struct {
	name string
	fn   ProviderFunc
}

func (p *namedProvider) GetHints(ctx Context) []*Hint {
	return p.fn.GetHints(ctx)
}

func (p *namedProvider) Name() string {
	return p.name
}