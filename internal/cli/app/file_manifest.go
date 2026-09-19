package app

import (
	"path/filepath"

	"github.com/agentstation/starmap/pkg/productpaths"
	filepolicy "github.com/agentstation/starmap/pkg/productpaths/policy"
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
	add("credential-policy", paths.CredentialPolicy, "tree", "available", "Persistent catalog startup before catalog writes.", "Preserve accepted credential policy during upgrades and recovery. Records contain no credential values.", "policy.json", "provider-*.json", ".policy-*", ".record-publications/.owner.lock", ".record-publications/*.jsonl")
	if paths.SourceFile.Path != "" {
		add("source-file", paths.SourceFile, "file", "available", "Operator-supplied file catalog source.", "Preserve the selected catalog payload and its access policy.")
	}
	add("baseline", paths.Baselines, "tree", "available", "Persistent application startup.", "Preserve or reproduce from the same binary.", "*/manifest.json", "*/catalog.json", ".baseline-*/**")
	add("baseline-recovery", child(paths.Baselines, ".starmap-baseline"), "tree", "available", "Baseline export and interrupted-stage recovery.", "Preserve journals and the writer lock until verified recovery completes.", ".owner.lock", "*.json", ".record-*")
	add("catalog-store", paths.CatalogStore, "tree", "available", "Catalog publication, generation pins, and retention.", "Preserve the current pointer, generations, read locks, and retirement records through coordinated recovery and consistent backups.",
		"current", ".commit.lock", "generations/*/manifest.json", "generations/*/catalog.json", "generations/*/authority.json", "generations/*/.authority-*", "generations/*/.read.lock",
		"generations/.retirement-*.json", "generations/.retirement-write-*", "generations/.record-publications/.owner.lock", "generations/.record-publications/*.jsonl",
		"generations/.candidate-*/**", ".current-*")
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
	add("runtime-evidence", child(paths.Runtime, "catalog-runtime"), "tree", "available", "Source refresh, provider acquisition, manual publication, or generation pin acceptance.", "Preserve accepted history, operator removals, pin receipts, and recovery inputs with the catalog state.", "source.json", "source.json.tmp", "manual.json", "publication.json", "removals.json", "generation-pin.json", ".layer-*", "providers/*.json", "providers/*.json.tmp", "providers/.layer-*", "providers/bindings/*.json", "providers/bindings/.layer-*", "publication-inputs/*.json", "publication-inputs/.layer-*", "publication-inputs/.input-*", ".record-publications/.owner.lock", ".record-publications/*.jsonl", "providers/.record-publications/.owner.lock", "providers/.record-publications/*.jsonl", "providers/bindings/.record-publications/.owner.lock", "providers/bindings/.record-publications/*.jsonl", "publication-inputs/.record-publications/.owner.lock", "publication-inputs/.record-publications/*.jsonl")
	add("runtime-record-staging", paths.Runtime, "patterns", "available", "Atomic owner or migration record writes.", "Remove only after ownership and interrupted-write checks.", ".owner-*")
	add("github-discovery", child(paths.Runtime, "github-catalog-source"), "tree", "available", "Configured GitHub source initialization and refresh.", "Preserve replay floors and accepted release references.", "*.json", ".state-*", ".record-publications/.owner.lock", ".record-publications/*.jsonl")
	add("source-http", paths.SourceCache, "tree", "available", "Explicit models.dev HTTP acquisition.", "Rebuild through permitted source access. Accepted evidence lives elsewhere.", "api.json", "api.json.metadata.json", ".starmap-cache-*")
	add("source-checkout", paths.SourceCheckout, "tree", "available", "Explicit pinned models.dev Git acquisition.", "Preserve operator-selected content. Rebuild managed input only through permitted acquisition.", "**")
	if err := addWorkspaceFiles(&report, paths); err != nil {
		return productpaths.FileManifest{}, err
	}
	add("migration-journal", child(paths.Roots[productpaths.State], "migrations"), "tree", "available", "An explicit runtime migration, unless its journal root is overridden.", "Preserve until the deployment recovery procedure permits removal.", "*/manifest.json", "*/stage-initialization.json", "*/journal.ndjson", "*/journal.partial-*", "*/.owner.lock", "*/.owner-*")
	add("admin-identities", child(paths.Roots[productpaths.State], "admin", "identities.json"), "file", "available", "Explicit local administrator initialization.", "Preserve identities, authority audience, and revision in an offline backup.")
	add("admin-audit", child(paths.Roots[productpaths.State], "admin", "audit"), "tree", "available", "Administrative intent and outcome writes.", "Preserve the complete audit history. Invalid or full history blocks new mutations.", "events.ndjson", ".audit-*", ".record-publications/**")
	add("admin-operations", child(paths.Roots[productpaths.State], "admin", "operations"), "tree", "available", "Durable administrative receipts.", "Preserve with audit history. Recovery never repeats an unknown effect.", "*.json", ".operation-*", ".record-publications/**")
	add("admin-owner", child(paths.Roots[productpaths.State], "admin"), "patterns", "available", "Exclusive administration writer and private publication recovery.", "Preserve ownership and publication receipts while state is in use.", ".owner.lock", ".identities-*", ".record-publications/**")
	for _, item := range []struct {
		id       string
		root     productpaths.Root
		parts    []string
		creation string
	}{
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
	access, err := filepolicy.Configuration(a.config.ConfigAccess, a.config.ConfigFile != "")
	if err != nil {
		return productpaths.FileManifest{}, err
	}
	for i := range report.Files {
		if report.Files[i].ID == "configuration" {
			report.Files[i].Policy.Access = access
			report.Files[i].Policy.Selectors = append(report.Files[i].Policy.Selectors, "--config-access", "STARMAP_CONFIG_ACCESS")
		}
	}
	return report, nil
}

func addWorkspaceFiles(report *productpaths.FileManifest, paths ProductPaths) error {
	entries, err := productpaths.WorkspaceFiles(paths.Workspace)
	if err != nil {
		return err
	}
	report.Files = append(report.Files, entries...)
	return nil
}
