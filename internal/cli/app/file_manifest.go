package app

import (
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/pkg/productpaths"
)

// FileManifest reports selected locations without opening catalog state or creating files.
// Planned paths identify remaining features and do not imply current file support.
func (a *App) FileManifest() (productpaths.FileManifest, error) {
	paths, err := a.ResolvedPaths()
	if err != nil {
		return productpaths.FileManifest{}, err
	}
	report := productpaths.FileManifest{SchemaVersion: 1, Product: productpaths.Starmap, BuildVersion: a.version, DeploymentID: paths.DeploymentID, InstanceID: paths.InstanceID, Roots: paths.Roots}
	report.RelativePathBase, report.RelativePathBaseOrigin = paths.RelativePathBase, paths.RelativePathBaseOrigin
	child := func(base productpaths.Path, parts ...string) productpaths.Path {
		if base.Path == "" {
			return productpaths.Path{Origin: base.Origin}
		}
		base.Path = filepath.Join(append([]string{base.Path}, parts...)...)
		return base
	}
	add := func(id string, location productpaths.Path, kind, availability, creation, recovery string, patterns ...string) {
		report.Files = append(report.Files, productpaths.FileEntry{ID: id, Location: location, Kind: kind, Patterns: patterns, Availability: availability, Creation: creation, Recovery: recovery})
	}
	add("configuration", paths.Configuration, "file", "available", "Operator-selected configuration.", "Preserve settings and required secret access.")
	if paths.SourceFile.Path != "" {
		add("source-file", paths.SourceFile, "file", "available", "Operator-supplied file catalog source.", "Preserve the selected catalog payload and its access policy.")
	}
	add("baseline", paths.Baselines, "tree", "available", "Persistent application startup.", "Preserve or reproduce from the same binary.", "*/manifest.json", "*/catalog.json", ".baseline-*/**")
	add("catalog-store", paths.CatalogStore, "tree", "available", "Accepted catalog publication.", "Preserve generations and the current pointer through a consistent backup.", "current", ".commit.lock", "generations/*/manifest.json", "generations/*/catalog.json", "generations/.candidate-*/**", ".current-*")
	for _, item := range []struct{ id, name, creation, recovery string }{
		{"runtime-owner", "owner.json", "Persistent runtime initialization.", "Preserve the product, deployment, and instance binding."},
		{"runtime-lock", ".owner.lock", "Runtime directory ownership.", "A lock file alone does not prove active ownership."},
		{"runtime-seed", "instance-seed", "Persistent runtime initialization.", "Restore one identity to one fenced replacement only."},
		{"migration-pending", ".migration-pending.json", "Migration staging and publication.", "Keep through publication. A pending target cannot start."},
		{"migration-receipt", ".migration-receipt.json", "Runtime migration publication.", "Preserve with the runtime and its operation journal."},
		{"migration-completed", ".migration-completed.json", "Completed runtime selection.", "Preserve while legacy-root acknowledgement is required."},
		{"migration-retired", ".migration-retired.json", "A later migration retires this runtime.", "Keep older binaries stopped and preserve the source files."},
	} {
		add(item.id, child(paths.Runtime, item.name), "file", "available", item.creation, item.recovery)
	}
	add("runtime-evidence", child(paths.Runtime, "catalog-runtime"), "tree", "available", "Source refresh or provider acquisition.", "Preserve permitted evidence with the catalog state.", "source.json", "source.json.tmp", ".layer-*", "providers/*.json", "providers/*.json.tmp", "providers/.layer-*")
	add("runtime-record-staging", paths.Runtime, "patterns", "available", "Atomic owner or migration record writes.", "Remove only after ownership and interrupted-write checks.", ".owner-*")
	add("github-discovery", child(paths.Runtime, "github-catalog-source"), "tree", "available", "Configured GitHub source initialization and refresh.", "Preserve replay floors and accepted release references.", "*.json", ".state-*")
	add("source-http", paths.SourceCache, "tree", "available", "Explicit models.dev HTTP acquisition.", "Rebuild through permitted source access. Accepted evidence lives elsewhere.", "api.json", "api.json.metadata.json", ".starmap-cache-*")
	add("source-checkout", paths.SourceCheckout, "tree", "available", "Explicit pinned models.dev Git acquisition.", "Preserve operator-selected content. Rebuild managed input only through permitted acquisition.", "**")
	addWorkspaceFiles(&report, paths)
	add("migration-journal", child(paths.Roots[productpaths.State], "migrations"), "tree", "available", "An explicit runtime migration, unless its journal root is overridden.", "Preserve until the deployment recovery procedure permits removal.", "*/manifest.json", "*/journal.ndjson", "*/journal.partial-*", "*/.owner.lock", "*/.owner-*")
	for _, item := range []struct {
		id       string
		root     productpaths.Root
		parts    []string
		creation string
	}{
		{"admin-identities", productpaths.State, []string{"admin", "identities.json"}, "Planned standalone administration setup."},
		{"admin-audit", productpaths.State, []string{"admin", "audit", "events.ndjson"}, "Planned standalone administrative mutations."},
		{"admin-operations", productpaths.State, []string{"admin", "operations"}, "Planned standalone administrative receipts."},
		{"download-staging", productpaths.Cache, []string{"catalog", "downloads"}, "Planned catalog download staging."},
		{"file-logs", productpaths.State, []string{"logs", "starmap.log"}, "Planned optional file logging. Current logging uses streams."},
		{"managed-trust", productpaths.Config, []string{"trust"}, "Planned explicit managed trust import."},
	} {
		add(item.id, child(paths.Roots[item.root], item.parts...), "reserved", "planned", item.creation, "No implemented writer uses this reserved path.")
	}
	report.External = []productpaths.ExternalFiles{
		{ID: "dotenv", Selection: "Only explicitly selected --env-file paths.", Recovery: "Preserve required values privately. No working-directory discovery occurs."},
		{ID: "credentials", Selection: "Explicit credential references and external secret managers.", Recovery: "Preserve access and provider credential roles. Values are excluded from this report."},
		{ID: "runtime-migration", Selection: "Explicit source, target, and optional journal root arguments.", Recovery: "The operation manifest records those paths and checksums. Stages are siblings of the explicit target."},
		{ID: "release-artifacts", Selection: "An explicit release output directory.", Recovery: "Preserve artifact bytes, attestation, checksum, and release identity."},
		{ID: "shell-completion", Selection: "Explicit shell completion installation through the shell adapter.", Recovery: "Regenerate from the installed binary and preserve shell configuration."},
		{ID: "tool-caches", Selection: "Git, Bun, and other external tools select their own caches.", Recovery: "External tool caches are outside the product file manifest."},
		{ID: "backup-and-usage", Selection: "Planned explicit backup or usage-export destination. No default directory.", Recovery: "Backup and export qualification remain separate production requirements."},
	}
	if err := applyFilePolicies(&report); err != nil {
		return productpaths.FileManifest{}, err
	}
	return report, nil
}

func addWorkspaceFiles(report *productpaths.FileManifest, paths ProductPaths) {
	availability := "available"
	if paths.Workspace.Path == "" {
		availability = "disabled"
	}
	report.Files = append(report.Files, productpaths.FileEntry{ID: "workspace", Location: paths.Workspace, Kind: "tree", Availability: availability, Patterns: []string{"**"}, Creation: "Explicit catalog authoring or projection.", Recovery: "Preserve operator content and projection receipts."})
	if availability == "disabled" {
		return
	}
	parent := paths.Workspace
	parent.Path = filepath.Dir(parent.Path)
	name := filepath.Base(paths.Workspace.Path)
	for _, item := range []struct{ id, suffix, creation, recovery string }{
		{"workspace-receipt", ".starmap-projection.json", "Successful workspace projection.", "Preserve with the human catalog workspace."},
		{"workspace-journal", ".starmap-replacement.json", "Windows workspace replacement records intent before either directory moves.", "Preserve with the candidate and backup. Projection repair validates and resumes the operation."},
		{"workspace-lock", ".starmap-write.lock", "First writer creates the lock. Shared reads and exclusive writes reuse it.", "Retain while the workspace is in use. A lock file alone does not prove active ownership."},
	} {
		location := parent
		location.Path = filepath.Join(parent.Path, "."+name+item.suffix)
		report.Files = append(report.Files, productpaths.FileEntry{ID: item.id, Location: location, Kind: "file", Availability: availability, Creation: item.creation, Recovery: item.recovery})
	}
	patternName := strings.NewReplacer("\\", "\\\\", "*", "\\*", "?", "\\?", "[", "\\[", "]", "\\]").Replace(name)
	report.Files = append(report.Files,
		productpaths.FileEntry{ID: "catalog-migration-lock", Location: parent, Kind: "patterns", Availability: availability, Patterns: []string{"." + patternName + ".starmap-migration-lock-*"}, Creation: "Windows migration creates a hard link to the existing private store commit lock.", Recovery: "Completed operations remove their own alias. Stop all store writers and migrations before removing an abandoned alias."},
		productpaths.FileEntry{ID: "workspace-preparing", Location: parent, Kind: "patterns", Availability: availability, Patterns: []string{"." + patternName + ".preparing-*"}, Creation: "Private workspace rendering and access restoration.", Recovery: "Preserve interrupted preparation until ownership checks permit cleanup."},
		productpaths.FileEntry{ID: "workspace-staging", Location: parent, Kind: "patterns", Availability: availability, Patterns: []string{"." + patternName + ".preparing-*/**", "." + patternName + ".candidate-*/**", ".." + patternName + ".candidate-*.verify-*/**", ".." + patternName + ".starmap-projection.json.*", ".." + patternName + ".starmap-replacement.json.*"}, Creation: "Workspace copy, verification, receipt, and journal writes.", Recovery: "Preserve interrupted work until ownership and recovery checks permit cleanup."},
		productpaths.FileEntry{ID: "workspace-backup", Location: parent, Kind: "patterns", Availability: availability, Patterns: []string{"." + patternName + ".backup-*/**"}, Creation: "Windows replacement retains the previous workspace until the new receipt is saved.", Recovery: "Retain with the replacement journal. Recovery refuses changed or unrecognized backup files."},
	)
}
