package config

import (
	"maps"
	"slices"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime"
)

// Layer supplies one named configuration authority. Earlier layers take precedence.
// The caller selects eligible authorities before resolution.
type Layer struct {
	Name   string
	Values map[string]string
}

// Ignored identifies a supplied setting that another authority replaced.
// Diagnostics never contain configured values.
type Ignored struct {
	Name   string
	Origin string
	Reason string
}

// IgnoredSharedAuthority reports a local deployment-scope value that the shared authority owns.
const IgnoredSharedAuthority = "shared-authority"

// Resolution binds parsed settings to their winning and ignored origins.
// Authority names the shared layer from ResolveAuthority and is empty for Resolve.
type Resolution struct {
	Config    Config
	Origins   map[string]string
	Ignored   []Ignored
	Authority string
}

// ResolveAuthority resolves settings under one shared deployment configuration authority.
// The shared layer supplies every deployment-scope value and selects the source group.
// A node-scope name, an unknown name, or the authority origin in the shared layer fails.
// It reports each local deployment-scope value as ignored with IgnoredSharedAuthority.
// Node-scope values and the host authority origin resolve from the local layers as in Resolve.
func ResolveAuthority(shared Layer, local ...Layer) (Resolution, error) {
	descriptors := Descriptors()
	shareable := make(map[string]bool, len(descriptors))
	for _, descriptor := range descriptors {
		shareable[descriptor.Name] = descriptor.Scope == DeploymentScope && descriptor.Name != AuthorityOrigin
	}
	for _, name := range slices.Sorted(maps.Keys(shared.Values)) {
		allowed, known := shareable[name]
		if !known {
			return Resolution{}, &errors.ValidationError{Field: name, Message: "is not a supported catalog setting"}
		}
		if !allowed {
			return Resolution{}, &errors.ValidationError{Field: name, Message: "cannot come from the shared configuration authority"}
		}
	}
	layers := append(make([]Layer, 0, len(local)+1), shared)
	var ignored []Ignored
	for _, layer := range local {
		if err := validateNames(layer.Values); err != nil {
			return Resolution{}, err
		}
		values := make(map[string]string)
		for _, descriptor := range descriptors {
			value, present := layer.Values[descriptor.Name]
			if !present {
				continue
			}
			if shareable[descriptor.Name] {
				ignored = append(ignored, Ignored{Name: descriptor.Name, Origin: layer.Name, Reason: IgnoredSharedAuthority})
				continue
			}
			values[descriptor.Name] = value
		}
		layers = append(layers, Layer{Name: layer.Name, Values: values})
	}
	result, err := Resolve(layers...)
	if err != nil {
		return Resolution{}, err
	}
	result.Ignored = append(result.Ignored, ignored...)
	result.Authority = shared.Name
	return result, nil
}

// Resolve preserves explicit presence and binds transport credentials to one source.
// An explicit source identity selects a complete source group from that layer or higher.
// Lower source credentials and identities do not enter the new source group.
func Resolve(layers ...Layer) (Resolution, error) {
	result := Resolution{Origins: make(map[string]string)}
	values := make(map[string]string)
	sourceLayer := len(layers)
	seen := make(map[string]bool)
	for index, layer := range layers {
		if layer.Name == "" || seen[layer.Name] {
			return Resolution{}, &errors.ValidationError{Field: "settings.layer", Message: "must have a unique nonempty name"}
		}
		seen[layer.Name] = true
		if err := validateNames(layer.Values); err != nil {
			return Resolution{}, err
		}
		for _, descriptor := range Descriptors() {
			if descriptor.SourceBinding == "" || descriptor.Sensitive {
				continue
			}
			if _, present := layer.Values[descriptor.Name]; present && index < sourceLayer {
				sourceLayer = index
			}
		}
	}
	for index, layer := range layers {
		for _, descriptor := range Descriptors() {
			value, present := layer.Values[descriptor.Name]
			if !present {
				continue
			}
			reason := ""
			if _, selected := values[descriptor.Name]; selected {
				reason = "higher-precedence-value"
			}
			if descriptor.SourceBinding != "" && index > sourceLayer {
				reason = "source-replaced"
			}
			if reason != "" {
				result.Ignored = append(result.Ignored, Ignored{Name: descriptor.Name, Origin: layer.Name, Reason: reason})
				continue
			}
			values[descriptor.Name], result.Origins[descriptor.Name] = value, layer.Name
		}
	}
	parsed, err := Parse(values)
	if err != nil {
		return Resolution{}, err
	}
	if err := validateSourceBinding(parsed); err != nil {
		return Resolution{}, err
	}
	if sourceLayer < len(layers) {
		// Clear previously composed source defaults and credentials before applying
		// the selected group. Independent policy values apply after this reset.
		entries := table()
		options := make([]runtime.Option, 0, len(entries)+len(parsed.options))
		for _, entry := range entries {
			descriptor := describe(entry)
			if descriptor.SourceBinding == "" {
				continue
			}
			option, err := entry.apply(descriptor.Default)
			if err != nil {
				return Resolution{}, err
			}
			options = append(options, option)
		}
		parsed.options = append(options, parsed.options...)
	}
	result.Config = parsed
	return result, nil
}

// CanonicalValues accepts descriptor keys, semantic IDs or canonical environment names.
// Conflicting aliases fail before any application configuration takes effect.
func CanonicalValues(values map[string]string) (map[string]string, error) {
	names := make(map[string]string)
	for _, descriptor := range Descriptors() {
		for _, name := range append([]string{descriptor.ID, descriptor.Key}, descriptor.Environment...) {
			names[name] = descriptor.Name
		}
	}
	result := make(map[string]string)
	for _, key := range slices.Sorted(maps.Keys(values)) {
		name, known := names[key]
		if !known {
			return nil, &errors.ValidationError{Field: key, Message: "is not a supported catalog setting"}
		}
		value := values[key]
		if prior, present := result[name]; present && prior != value {
			return nil, &errors.ValidationError{Field: name, Message: "has conflicting configuration aliases"}
		}
		result[name] = value
	}
	return result, nil
}

// validateSourceBinding refuses a selected credential or endpoint for another source kind.
func validateSourceBinding(parsed Config) error {
	for _, descriptor := range Descriptors() {
		value, present := parsed.Value(descriptor.Name)
		if !present || value == "" || descriptor.SourceBinding == "" {
			continue
		}
		if !slices.Contains(descriptor.Applicability, "all") && !slices.Contains(descriptor.Applicability, string(parsed.SourceKind)) {
			return &errors.ValidationError{Field: descriptor.Name, Message: "does not apply to the selected source kind"}
		}
	}
	if (parsed.SourceKind == runtime.SourceStarmap || parsed.SourceKind == runtime.SourceFile) && parsed.SourceURL == "" {
		return &errors.ValidationError{Field: SourceURL, Message: "is required for the selected source kind"}
	}
	return nil
}
