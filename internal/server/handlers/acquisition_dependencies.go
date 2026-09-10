package handlers

import (
	"errors"
	"slices"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// acquisitionDependencyDetail reports only recognized source and tool names.
// Error messages, executable paths, and installation commands remain private.
func acquisitionDependencyDetail(err error) map[string]any {
	var dependency *pkgerrors.DependencyError
	if !errors.As(err, &dependency) || !slices.Contains(sources.IDs(), sources.ID(dependency.Source)) {
		return nil
	}
	detail := map[string]any{"source": dependency.Source, "action": "check_source_dependencies"}
	switch dependency.Dependency {
	case "git", "bun":
		detail["dependency"] = dependency.Dependency
	}
	return detail
}
