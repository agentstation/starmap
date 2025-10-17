// Package deps provides dependency checking and management for sources.
package deps

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/agentstation/starmap/pkg/sources"
)

// Check verifies if a dependency is available on the system.
// It tries all CheckCommands in order and returns the first one that succeeds.
func Check(ctx context.Context, dep sources.Dependency) sources.DependencyStatus {
	status := sources.DependencyStatus{
		Available: false,
	}

	// Try each command in order
	for _, cmd := range dep.CheckCommands {
		path, err := exec.LookPath(cmd)
		if err != nil {
			// Command not found, try next one
			continue
		}

		// Found the command
		status.Available = true
		status.Path = path

		// Try to get version if MinVersion is specified
		if dep.MinVersion != "" {
			version, err := getVersion(ctx, cmd)
			if err != nil {
				status.CheckError = fmt.Errorf("found %s but could not detect version: %w", cmd, err)
			} else {
				status.Version = version
				// Check if version meets minimum requirement
				if !meetsMinVersion(version, dep.MinVersion) {
					status.CheckError = fmt.Errorf("found %s version %s but requires %s or later", cmd, version, dep.MinVersion)
				}
			}
		}

		return status
	}

	// None of the commands were found
	if len(dep.CheckCommands) > 0 {
		status.CheckError = fmt.Errorf("%s not found in PATH (tried: %s)", dep.DisplayName, strings.Join(dep.CheckCommands, ", "))
	}

	return status
}

// CheckAll checks all dependencies for a source.
// Returns a map of dependency name to status.
func CheckAll(ctx context.Context, src sources.Source) map[string]sources.DependencyStatus {
	deps := src.Dependencies()
	if len(deps) == 0 {
		return nil
	}

	results := make(map[string]sources.DependencyStatus, len(deps))
	for _, dep := range deps {
		results[dep.Name] = Check(ctx, dep)
	}

	return results
}

// HasMissingDeps returns true if any dependencies are missing.
func HasMissingDeps(statuses map[string]sources.DependencyStatus) bool {
	for _, status := range statuses {
		if !status.Available {
			return true
		}
	}
	return false
}

// GetMissingDeps returns a list of dependencies that are missing.
func GetMissingDeps(deps []sources.Dependency, statuses map[string]sources.DependencyStatus) []sources.Dependency {
	var missing []sources.Dependency
	for _, dep := range deps {
		if status, ok := statuses[dep.Name]; ok && !status.Available {
			missing = append(missing, dep)
		}
	}
	return missing
}

// getVersion attempts to get the version of a command.
// This is a best-effort attempt - different tools have different version flags.
func getVersion(ctx context.Context, cmdName string) (string, error) {
	// Try common version flags in order
	versionFlags := []string{"--version", "-v", "version"}

	for _, flag := range versionFlags {
		//nolint:gosec // cmdName comes from Dependency.CheckCommands (trusted source)
		cmd := exec.CommandContext(ctx, cmdName, flag)
		output, err := cmd.CombinedOutput()
		if err != nil {
			continue // Try next flag
		}

		// Extract version number from output
		version := extractVersion(string(output))
		if version != "" {
			return version, nil
		}
	}

	return "", fmt.Errorf("could not determine version")
}

// extractVersion tries to extract a semantic version number from output.
// Looks for patterns like "1.2.3", "v1.2.3", "version 1.2.3", etc.
func extractVersion(output string) string {
	// Common version patterns
	patterns := []string{
		`v?(\d+\.\d+\.\d+)`,           // 1.2.3 or v1.2.3
		`version\s+v?(\d+\.\d+\.\d+)`, // version 1.2.3
		`(\d+\.\d+\.\d+)`,             // Just the numbers
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(output)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// meetsMinVersion checks if the detected version meets the minimum requirement.
// This is a simplified version comparison that works for semantic versioning.
func meetsMinVersion(detected, required string) bool {
	// Simple string comparison for now
	// In a production system, you'd want a proper semver library
	detectedParts := strings.Split(strings.TrimPrefix(detected, "v"), ".")
	requiredParts := strings.Split(strings.TrimPrefix(required, "v"), ".")

	for i := 0; i < len(requiredParts) && i < len(detectedParts); i++ {
		// Simple numeric comparison
		if detectedParts[i] < requiredParts[i] {
			return false
		}
		if detectedParts[i] > requiredParts[i] {
			return true
		}
	}

	// All parts equal or detected has more parts
	return len(detectedParts) >= len(requiredParts)
}
