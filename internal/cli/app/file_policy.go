package app

import (
	"slices"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths"
	filepolicy "github.com/agentstation/starmap/pkg/productpaths/policy"
)

func applyFilePolicies(report *productpaths.FileManifest) error {
	for i := range report.Files {
		policy, err := managedFilePolicy(report.Files[i].ID)
		if err != nil {
			return err
		}
		report.Files[i].Policy = policy
	}
	for i := range report.External {
		policy, err := externalFilePolicy(report.External[i].ID)
		if err != nil {
			return err
		}
		report.External[i].Policy = policy
	}
	return nil
}

func managedFilePolicy(id string) (productpaths.FilePolicy, error) {
	access, err := filepolicy.ForRole(id)
	if err != nil {
		return productpaths.FilePolicy{}, err
	}
	config := []string{"--config", "CONFIG", "STARMAP_CONFIG_DIR", "STARMAP_HOME"}
	data := []string{"STARMAP_DATA_DIR", "STARMAP_HOME"}
	state := []string{"STARMAP_STATE_ROOT", "STARMAP_HOME"}
	cache := []string{"STARMAP_CACHE_DIR", "STARMAP_HOME"}
	runtime := append(slices.Clone(state), catalogconfig.StateDirectory, "STARMAP_INSTANCE_ID")
	workspace := append(slices.Clone(data), catalogconfig.WorkspacePath, "--catalog-path")
	makePolicy := func(selectors []string, applicability, retention, removal string) productpaths.FilePolicy {
		return productpaths.FilePolicy{Selectors: selectors, Applicability: applicability, Access: access, Retention: retention, Removal: removal}
	}
	var policy productpaths.FilePolicy
	switch id {
	case "configuration":
		policy = makePolicy(config, "Selected configuration, including private credential settings.", "operator", "Remove only after preserving required settings and secret access.")
	case "source-file":
		policy = makePolicy([]string{catalogconfig.Source, catalogconfig.SourceURL}, "Explicit file catalog source.", "operator", "Preserve until all consumers select another source or retain a permitted generation.")
	case "baseline":
		policy = makePolicy(data, "Verified embedded catalog export on persistent startup.", "reproducible", "Remove only if the same binary can reproduce the baseline. Never remove an active export.")
	case "catalog-store":
		policy = makePolicy(append(slices.Clone(state), "STARMAP_CATALOG_STORE_PATH"), "Filesystem catalog store selected by the standalone application.", "durable", "Preserve the current pointer and referenced generations. Use a consistent backup before replacement.")
	case "runtime-owner", "runtime-lock", "runtime-seed":
		policy = makePolicy(runtime, "Persistent identity and exclusive ownership for one runtime.", "identity", "Keep while the runtime or its recovery procedure needs this identity. A stopped process does not make deletion safe.")
	case "migration-pending", "migration-receipt", "migration-completed", "migration-retired":
		policy = makePolicy(runtime, "Runtime migration and legacy-root acknowledgement.", "recovery", "Keep until the migration procedure permits removal. Retirement records must still fence old runtime starts.")
	case "runtime-evidence":
		policy = makePolicy(runtime, "Retained catalog evidence and operator recovery records.", "durable", "Preserve evidence required for retained startup and authority enforcement. This state is not a disposable source cache.")
	case "runtime-record-staging":
		policy = makePolicy(runtime, "Temporary owner and migration record writes.", "recovery", "Remove only after proving operation ownership and completing or abandoning the interrupted write.")
	case "github-discovery":
		policy = makePolicy(runtime, "Public or GitHub catalog discovery with retained replay protection.", "durable", "Preserve replay floors through restart and backup. Disabling downloads does not authorize resetting this state.")
	case "source-http":
		policy = makePolicy(cache, "Explicit models.dev HTTP acquisition.", "rebuildable", "Remove only when no acquisition uses the files and permitted source access can rebuild them.")
	case "source-checkout":
		policy = makePolicy(cache, "Explicit models.dev Git acquisition, including caller-selected checkouts.", "operator", "Preserve local edits. Remove a managed checkout only after excluding active acquisition and operator-owned content.")
	case "workspace", "workspace-receipt":
		policy = makePolicy(workspace, "Selected optional YAML authoring workspace.", "operator", "Preserve operator files and their projection receipt together. Catalog refresh does not authorize deleting unrelated files.")
	case "workspace-lock":
		policy = makePolicy(workspace, "Shared workspace reads and exclusive writes after the first writer.", "identity", "Retain for the lifetime of the workspace. A lock file alone does not prove an active writer.")
	case "catalog-migration-lock":
		policy = makePolicy(workspace, "Windows migration from the legacy catalog layout.", "recovery", "Remove an abandoned alias only after all store writers and migrations stop. Preserve the store commit lock.")
	case "workspace-preparing":
		policy = makePolicy(workspace, "Private container for workspace rendering and access restoration.", "recovery", "Retain until verified operation ownership permits cleanup.")
	case "workspace-journal", "workspace-backup", "workspace-staging":
		policy = makePolicy(workspace, "Workspace projection, validation, or interrupted replacement.", "recovery", "Retain until verified recovery permits cleanup. Preserve changed files and candidates referenced by a replacement journal.")
	case "migration-journal":
		policy = makePolicy(append(slices.Clone(state), "migrate runtime --journal-root"), "Explicit runtime migration operations.", "recovery", "Preserve until migration completion and deployment recovery no longer require the operation records.")
	case "admin-identities":
		policy = makePolicy(state, "Planned standalone administration setup.", "identity", "A future administration procedure must preserve identity and recovery access before removal.")
	case "admin-audit", "admin-operations":
		policy = makePolicy(state, "Planned standalone administrative audit and operation records.", "audit", "A future qualified retention procedure must satisfy the deployment audit policy before removal.")
	case "download-staging":
		policy = makePolicy(cache, "Planned catalog download staging.", "recovery", "A future cleanup procedure must exclude active downloads and unprocessed recovery records.")
	case "file-logs":
		policy = makePolicy(state, "Planned optional file logging.", "audit", "A future rotation procedure must preserve the required retention window. Current logging uses streams.")
	case "managed-trust":
		policy = makePolicy([]string{"STARMAP_CONFIG_DIR", "STARMAP_HOME"}, "Planned explicit managed trust import.", "operator", "Remove only through an explicit trust change that preserves access to accepted authorities.")
	default:
		return productpaths.FilePolicy{}, unknownFilePolicy(id)
	}
	return policy, nil
}

func externalFilePolicy(id string) (productpaths.FilePolicy, error) {
	access, err := filepolicy.ForRole(id)
	if err != nil {
		return productpaths.FilePolicy{}, err
	}
	var p productpaths.FilePolicy
	switch id {
	case "dotenv":
		p = productpaths.FilePolicy{Selectors: []string{"--env-file"}, Applicability: "Explicit environment files only.", Access: access, Retention: "operator", Removal: "Preserve required values privately before removing an explicit input file."}
	case "credentials":
		p = productpaths.FilePolicy{Selectors: []string{"credential_sources"}, Applicability: "Explicit credential references or external secret managers.", Access: access, Retention: "operator", Removal: "Rotate or retire credentials through their owning system and preserve separate acquisition and inference roles."}
	case "runtime-migration":
		p = productpaths.FilePolicy{Selectors: []string{"migrate runtime --from", "migrate runtime --to", "migrate runtime --journal-root"}, Applicability: "Explicit migration destinations outside default roots.", Access: access, Retention: "recovery", Removal: "Keep source, target, stage, and journals until the migration procedure permits removal."}
	case "release-artifacts":
		p = productpaths.FilePolicy{Selectors: []string{"release output directory"}, Applicability: "Explicit release assembly.", Access: access, Retention: "durable", Removal: "Preserve published bytes, attestations, and release identity for the supported release lifetime."}
	case "shell-completion":
		p = productpaths.FilePolicy{Selectors: []string{"shell completion destination"}, Applicability: "Explicit shell adapter installation.", Access: access, Retention: "reproducible", Removal: "Regenerate from the selected binary and preserve the shell configuration."}
	case "tool-caches":
		p = productpaths.FilePolicy{Selectors: []string{"external tool configuration"}, Applicability: "Caches owned by external tools.", Access: access, Retention: "operator", Removal: "Use the owning tool's cache policy. Product cleanup does not own these paths."}
	case "backup-and-usage":
		p = productpaths.FilePolicy{Selectors: []string{"explicit backup or export destination"}, Applicability: "Planned backup and usage export operations.", Access: access, Retention: "audit", Removal: "Follow the deployment retention policy and prove backup recovery before removing retained exports."}
	default:
		return productpaths.FilePolicy{}, unknownFilePolicy(id)
	}
	return p, nil
}

func unknownFilePolicy(id string) error {
	return &errors.ConfigError{Component: "file inventory", Message: "file role has no declared access and retention policy: " + id}
}
