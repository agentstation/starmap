// Package deps provides commands for managing external dependencies required by data sources.
package deps

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/deps"
	"github.com/agentstation/starmap/internal/sources/local"
	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// NewCheckCommand creates the deps check subcommand using app context.
func NewCheckCommand(app application.Application) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check dependency status for all sources",
		Long: `Check the availability of external dependencies required by data sources.

This command verifies that external tools (like 'bun', 'git', etc.) are installed
and accessible in your PATH. For each dependency, it shows:

  - Whether it's installed
  - The installed version (if detectable)
  - Installation path
  - Installation instructions if missing

Sources are marked as Optional if they can be skipped when dependencies are missing.`,
		Example: `  starmap deps check                # Check all dependencies
  starmap deps check --output json  # JSON output
  starmap deps check -v             # Verbose output`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCheck(cmd, app)
		},
	}
}

// SourceDepStatus holds dependency status for one source.
type SourceDepStatus struct {
	SourceID     sources.ID         `json:"source_id" yaml:"source_id"`
	SourceName   string             `json:"source_name" yaml:"source_name"`
	IsOptional   bool               `json:"is_optional" yaml:"is_optional"`
	Dependencies []DependencyDetail `json:"dependencies" yaml:"dependencies"`
}

// DependencyDetail combines dependency definition with status.
type DependencyDetail struct {
	Dependency sources.Dependency       `json:"dependency" yaml:"dependency"`
	Status     sources.DependencyStatus `json:"status" yaml:"status"`
}

// CheckResults aggregates all statuses.
type CheckResults struct {
	Sources           []SourceDepStatus `json:"sources" yaml:"sources"`
	TotalDeps         int               `json:"total_deps" yaml:"total_deps"`
	AvailableDeps     int               `json:"available_deps" yaml:"available_deps"`
	MissingDeps       int               `json:"missing_deps" yaml:"missing_deps"`
	SourcesWithNoDeps int               `json:"sources_with_no_deps" yaml:"sources_with_no_deps"`
}

// runCheck executes the dependency check command.
func runCheck(cmd *cobra.Command, app application.Application) error {
	ctx := context.Background()

	// Get all sources
	allSources := getAllSources()

	// Collect dependency statuses
	results := collectDependencyStatuses(ctx, allSources)

	// Get global flags for output format
	globalFlags, err := globals.Parse(cmd)
	if err != nil {
		return err
	}

	// Display results based on format
	if err := displayResults(results, globalFlags, cmd, app); err != nil {
		return err
	}

	// Return error if required dependencies are missing
	if results.MissingDeps > 0 {
		// Check if any missing deps are from required sources
		hasRequiredMissing := false
		for _, srcStatus := range results.Sources {
			if !srcStatus.IsOptional {
				for _, dep := range srcStatus.Dependencies {
					if !dep.Status.Available {
						hasRequiredMissing = true
						break
					}
				}
			}
			if hasRequiredMissing {
				break
			}
		}

		if hasRequiredMissing {
			return &errors.ValidationError{
				Field:   "dependencies",
				Message: "required dependencies are missing",
			}
		}
	}

	return nil
}

// getAllSources creates all available sources.
func getAllSources() []sources.Source {
	return []sources.Source{
		local.New(),
		providers.New(),
		modelsdev.NewGitSource(),
		modelsdev.NewHTTPSource(),
	}
}

// collectDependencyStatuses checks all sources and collects dependency statuses.
func collectDependencyStatuses(ctx context.Context, srcs []sources.Source) *CheckResults {
	results := &CheckResults{
		Sources: make([]SourceDepStatus, 0, len(srcs)),
	}

	for _, src := range srcs {
		sourceDeps := src.Dependencies()

		if len(sourceDeps) == 0 {
			results.SourcesWithNoDeps++
			results.Sources = append(results.Sources, SourceDepStatus{
				SourceID:     src.ID(),
				SourceName:   src.Name(),
				IsOptional:   src.IsOptional(),
				Dependencies: []DependencyDetail{},
			})
			continue
		}

		// Check each dependency
		statuses := deps.CheckAll(ctx, src)
		details := make([]DependencyDetail, 0, len(sourceDeps))

		for _, dep := range sourceDeps {
			status := statuses[dep.Name]
			details = append(details, DependencyDetail{
				Dependency: dep,
				Status:     status,
			})

			results.TotalDeps++
			if status.Available {
				results.AvailableDeps++
			} else {
				results.MissingDeps++
			}
		}

		results.Sources = append(results.Sources, SourceDepStatus{
			SourceID:     src.ID(),
			SourceName:   src.Name(),
			IsOptional:   src.IsOptional(),
			Dependencies: details,
		})
	}

	return results
}
