// Package productpaths resolves application roots and leaf paths without creating files.
// The host supplies configuration layers and an explicit platform-default lookup.
package productpaths

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// Root identifies one application directory role.
type Root string

const (
	// Home groups the four roots under one directory.
	Home Root = "home"
	// Config contains application configuration.
	Config Root = "config"
	// Data contains durable application data.
	Data Root = "data"
	// State contains process state.
	State Root = "state"
	// Cache contains disposable application data.
	Cache Root = "cache"
)

// Path reports a resolved absolute path and the authority that selected it.
// Anchor names the parent used for a relative leaf or grouped root.
type Path struct {
	Path   string `json:"path"`
	Origin string `json:"origin"`
	Anchor string `json:"anchor,omitempty"`
}

// Layer supplies explicit root values. Earlier layers take precedence at equal specificity.
// A specific root replaces that child of Home, regardless of Home's layer.
type Layer struct {
	Name   string
	Values map[Root]string
}

// Lookup returns an absolute platform default for one root.
// Resolve calls it only when no configured root or grouped home supplies the path.
type Lookup func(Root) (string, error)

// Roots lists the four resolved application roots.
type Roots map[Root]Path

// Resolve validates selected roots and preserves explicit presence.
// It never creates a directory or reads application configuration.
func Resolve(defaults Lookup, layers ...Layer) (Roots, error) {
	result := make(Roots, 4)
	for _, root := range []Root{Config, Data, State, Cache} {
		path, err := ResolveRoot(root, defaults, layers...)
		if err != nil {
			return nil, err
		}
		result[root] = path
	}
	return result, nil
}

// ResolveRoot selects one root before later configuration needs the other roots.
// It rejects unknown names but validates only the selected path value.
func ResolveRoot(root Root, defaults Lookup, layers ...Layer) (Path, error) {
	if !slices.Contains([]Root{Config, Data, State, Cache}, root) {
		return Path{}, invalidRoot(root)
	}
	names := make(map[string]bool)
	for _, layer := range layers {
		if layer.Name == "" || names[layer.Name] {
			return Path{}, &errors.ValidationError{Field: "paths.layer", Message: "must have a unique nonempty name"}
		}
		names[layer.Name] = true
		for name := range layer.Values {
			if !slices.Contains([]Root{Home, Config, Data, State, Cache}, name) {
				return Path{}, invalidRoot(name)
			}
		}
	}
	validate := func(value string) error {
		if !filepath.IsAbs(value) || strings.ContainsRune(value, '\x00') {
			return &errors.ValidationError{Field: string(root), Message: "must be a nonempty absolute directory"}
		}
		return nil
	}
	for _, layer := range layers {
		if value, present := layer.Values[root]; present {
			if err := validate(value); err != nil {
				return Path{}, err
			}
			return Path{Path: filepath.Clean(value), Origin: layer.Name}, nil
		}
	}
	for _, layer := range layers {
		if value, present := layer.Values[Home]; present {
			if err := validate(value); err != nil {
				return Path{}, err
			}
			return Path{Path: filepath.Join(value, string(root)), Origin: layer.Name, Anchor: filepath.Clean(value)}, nil
		}
	}
	if defaults == nil {
		return Path{}, &errors.ConfigError{Component: "product paths", Message: "a platform default or explicit root is required"}
	}
	value, err := defaults(root)
	if err != nil {
		return Path{}, err
	}
	if err := validate(value); err != nil {
		return Path{}, err
	}
	return Path{Path: filepath.Clean(value), Origin: "platform-default"}, nil
}

// Leaf anchors a required relative leaf under the resolved configuration root.
// An explicit absolute leaf keeps its exact location.
func Leaf(config Path, value, origin string) (Path, error) {
	if value == "" || strings.ContainsRune(value, '\x00') {
		return Path{}, &errors.ValidationError{Field: "paths.leaf", Message: "must not be empty or contain NUL"}
	}
	if filepath.IsAbs(value) {
		return Path{Path: filepath.Clean(value), Origin: origin}, nil
	}
	if !filepath.IsAbs(config.Path) {
		return Path{}, &errors.ValidationError{Field: "paths.config", Message: "must be an absolute anchor"}
	}
	// Drive-relative and rooted-without-volume Windows paths are not portable relative leaves.
	if filepath.VolumeName(value) != "" || strings.HasPrefix(value, "\\") {
		return Path{}, &errors.ValidationError{Field: "paths.leaf", Message: "must be absolute or relative to the configuration root"}
	}
	return Path{Path: filepath.Join(config.Path, value), Origin: origin, Anchor: config.Path}, nil
}

// ValidateInstanceID requires a portable lowercase component with at most 63 bytes.
func ValidateInstanceID(id string) error {
	invalid := func() error {
		return &errors.ValidationError{Field: "instance_id", Message: "must be a safe lowercase instance name with at most 63 bytes"}
	}
	if len(id) == 0 || len(id) > 63 {
		return invalid()
	}
	for index, ch := range []byte(id) {
		if ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' {
			continue
		}
		if index > 0 && (ch == '-' || ch == '_') {
			continue
		}
		return invalid()
	}
	if slices.Contains([]string{"con", "prn", "aux", "nul"}, id) {
		return invalid()
	}
	if len(id) == 4 && (strings.HasPrefix(id, "com") || strings.HasPrefix(id, "lpt")) && id[3] >= '1' && id[3] <= '9' {
		return invalid()
	}
	return nil
}
