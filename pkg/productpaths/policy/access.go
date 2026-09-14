// Package policy owns file-role access classes shared by runtime adapters and diagnostics.
// It does not inspect files, change permissions, or claim effective access.
package policy

import "github.com/agentstation/starmap/pkg/errors"

const (
	// OwnerOnly restricts managed state to the process account and permitted host administrators.
	OwnerOnly = "owner-only"
	// ServiceManaged permits shared reads of explicitly selected, trusted configuration.
	ServiceManaged = "service-managed"
	// DeploymentControlled leaves input sharing and editing rights to the deployment.
	DeploymentControlled = "deployment-controlled"
	// PublicRead permits explicit read access to verified public exports.
	PublicRead = "public-read"
	// ExternalSystem assigns access enforcement to the selected external system.
	ExternalSystem = "external-system"
)

// ForRole returns the canonical access class for a Starmap file role.
// Starport can reuse these catalog roles under its own product roots.
// Unknown roles return an error without selecting a default policy.
func ForRole(role string) (string, error) {
	switch role {
	case "configuration", "dotenv", "catalog-store", "catalog-migration-lock", "baseline-recovery",
		"runtime-owner", "runtime-lock", "runtime-seed", "runtime-evidence", "runtime-record-staging",
		"migration-pending", "migration-receipt", "migration-completed", "migration-retired", "migration-journal", "runtime-migration",
		"github-discovery", "workspace-preparing", "admin-identities", "admin-audit", "admin-operations",
		"download-staging", "file-logs", "managed-trust":
		return OwnerOnly, nil
	case "workspace", "workspace-receipt", "workspace-lock", "workspace-journal", "workspace-backup", "workspace-staging",
		"source-file", "source-http", "source-checkout", "release-artifacts", "shell-completion":
		return DeploymentControlled, nil
	case "baseline":
		return PublicRead, nil
	case "credentials", "tool-caches", "backup-and-usage":
		return ExternalSystem, nil
	default:
		return "", &errors.ConfigError{Component: "file policy", Message: "file role has no declared access policy: " + role}
	}
}

// Require checks that an adapter supports the role's declared access class before file access.
// A policy change requires a matching adapter instead of silently changing enforcement.
func Require(role, supported string) error {
	access, err := ForRole(role)
	if err != nil {
		return err
	}
	if access != supported {
		return &errors.ConfigError{Component: "file policy", Message: "file role requires a different access adapter: " + role}
	}
	return nil
}

// Configuration selects the primary input policy before the reader opens its bytes.
// An empty selection retains owner-only access. Service mode requires an explicit path.
func Configuration(selected string, explicitPath bool) (string, error) {
	switch selected {
	case "", OwnerOnly:
		return OwnerOnly, nil
	case ServiceManaged:
		if explicitPath {
			return ServiceManaged, nil
		}
		return "", &errors.ConfigError{Component: "configuration access", Message: "service-managed access requires an explicit configuration file path"}
	default:
		return "", &errors.ConfigError{Component: "configuration access", Message: "requires owner-only or service-managed"}
	}
}
