package sync

import (
	"fmt"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

var modelProvenanceFieldSuffixes = []string{
	"limits.context_window",
	"limits.input_tokens",
	"limits.output_tokens",
	"lineage.family",
	"lineage.root",
	"lineage.parent",
	"ReasoningTokens",
	"Description",
	"Attachments",
	"Generation",
	"Reasoning",
	"Verbosity",
	"Features",
	"Pricing",
	"Metadata",
	"Delivery",
	"Tools",
	"pricing",
	"metadata",
	"modes",
}

// Result represents the complete result of a sync operation.
type Result struct {
	// Overall statistics
	TotalChanges     int                                     // Total number of changes across all providers
	ProvidersChanged int                                     // Number of providers with changes
	ProviderResults  map[catalogs.ProviderID]*ProviderResult // Results per provider

	// Operation metadata
	DryRun    bool   // Whether this was a dry run
	Fresh     bool   // Whether this was a fresh sync
	OutputDir string // Where files were written (empty means default)
	Sources   []sources.ID
	// SourceObservations contains caller-owned freshness/audit projections from
	// every source used by this attempt, including no-change synchronizations.
	SourceObservations []catalogs.SourceObservationLink
	GenerationID       string // Durable generation activated by a non-dry sync
	SyncRunID          string // Correlation ID for the synchronization attempt
}

// ProviderResult represents sync results for a single provider.
type ProviderResult struct {
	ProviderID catalogs.ProviderID  // The provider that was synced
	Added      []catalogs.Model     // New models not in catalog
	Updated    []differ.ModelUpdate // Existing models with changes
	Removed    []catalogs.Model     // Models in catalog but not in API (informational only)

	// Summary counts
	AddedCount   int // Number of models added
	UpdatedCount int // Number of models updated
	RemovedCount int // Number of models removed from API (not deleted from catalog)

	// Metadata
	APIModelsCount      int // Total models fetched from API
	ExistingModelsCount int // Total models that existed in catalog
	EnhancedCount       int // Number of models enhanced with models.dev data
}

// HasChanges returns true if the sync result contains any changes.
func (sr *Result) HasChanges() bool {
	return sr.TotalChanges > 0
}

// HasChanges returns true if the provider result contains any changes.
func (spr *ProviderResult) HasChanges() bool {
	return spr.AddedCount > 0 || spr.UpdatedCount > 0 || spr.RemovedCount > 0
}

// Summary returns a human-readable summary of the sync result.
func (sr *Result) Summary() string {
	if !sr.HasChanges() {
		return "No changes detected"
	}

	var parts []string
	if sr.DryRun {
		parts = append(parts, "(Dry run)")
	}
	if sr.Fresh {
		parts = append(parts, "(Fresh sync)")
	}

	summary := fmt.Sprintf("%d total changes across %d providers", sr.TotalChanges, sr.ProvidersChanged)
	if len(parts) > 0 {
		summary += " " + strings.Join(parts, " ")
	}

	return summary
}

// Summary returns a human-readable summary of the provider result.
func (spr *ProviderResult) Summary() string {
	if !spr.HasChanges() {
		return fmt.Sprintf("%s: No changes", spr.ProviderID)
	}

	return fmt.Sprintf("%s: %d added, %d updated, %d removed",
		spr.ProviderID, spr.AddedCount, spr.UpdatedCount, spr.RemovedCount)
}

// ChangesetToResult converts a reconcile.Changeset to a SyncResult.
func ChangesetToResult(changeset *differ.Changeset, dryRun bool, outputDir string, providerAPICounts map[catalogs.ProviderID]int, modelProviderMap map[string]catalogs.ProviderID, activeSources ...sources.ID) *Result {
	return ChangesetToResultWithProvenance(
		changeset,
		dryRun,
		outputDir,
		providerAPICounts,
		modelProviderMap,
		nil,
		activeSources...,
	)
}

// ChangesetToResultWithProvenance converts a reconcile.Changeset to a SyncResult
// and uses field-level provenance to report models.dev enrichment counts.
func ChangesetToResultWithProvenance(changeset *differ.Changeset, dryRun bool, outputDir string, providerAPICounts map[catalogs.ProviderID]int, modelProviderMap map[string]catalogs.ProviderID, fieldProvenance provenance.Map, activeSources ...sources.ID) *Result {
	changeset = normalizeChangeset(changeset)

	result := &Result{
		TotalChanges:    changeset.Summary.TotalChanges,
		DryRun:          dryRun,
		OutputDir:       outputDir,
		Sources:         append([]sources.ID(nil), activeSources...),
		ProviderResults: make(map[catalogs.ProviderID]*ProviderResult),
	}

	// Group models by provider for the provider results
	providerAdded := make(map[catalogs.ProviderID][]catalogs.Model)
	providerUpdated := make(map[catalogs.ProviderID][]differ.ModelUpdate)
	providerRemoved := make(map[catalogs.ProviderID][]catalogs.Model)

	if len(changeset.Models.AddedScoped) > 0 {
		for _, change := range changeset.Models.AddedScoped {
			providerID := getModelChangeProvider(change, modelProviderMap)
			providerAdded[providerID] = append(providerAdded[providerID], change.Model)
		}
	} else {
		for _, model := range changeset.Models.Added {
			providerID := getModelProvider(model, modelProviderMap)
			providerAdded[providerID] = append(providerAdded[providerID], model)
		}
	}

	for _, update := range changeset.Models.Updated {
		providerID := getModelUpdateProvider(update, modelProviderMap)
		providerUpdated[providerID] = append(providerUpdated[providerID], update)
	}

	if len(changeset.Models.RemovedScoped) > 0 {
		for _, change := range changeset.Models.RemovedScoped {
			providerID := getModelChangeProvider(change, modelProviderMap)
			providerRemoved[providerID] = append(providerRemoved[providerID], change.Model)
		}
	} else {
		for _, model := range changeset.Models.Removed {
			providerID := getModelProvider(model, modelProviderMap)
			providerRemoved[providerID] = append(providerRemoved[providerID], model)
		}
	}

	// Collect all providers that have changes
	allProviders := make(map[catalogs.ProviderID]bool)
	for providerID := range providerAdded {
		allProviders[providerID] = true
	}
	for providerID := range providerUpdated {
		allProviders[providerID] = true
	}
	for providerID := range providerRemoved {
		allProviders[providerID] = true
	}

	provenanceIndex := indexModelProvenance(fieldProvenance, allProviders)

	// Create provider results
	for providerID := range allProviders {
		providerResult := &ProviderResult{
			ProviderID:     providerID,
			Added:          providerAdded[providerID],
			Updated:        providerUpdated[providerID],
			Removed:        providerRemoved[providerID],
			AddedCount:     len(providerAdded[providerID]),
			UpdatedCount:   len(providerUpdated[providerID]),
			RemovedCount:   len(providerRemoved[providerID]),
			APIModelsCount: providerAPICounts[providerID], // Now properly set from actual API fetch
		}
		if hasModelsDevSource(activeSources) {
			providerResult.EnhancedCount = countEnrichmentUpdates(providerUpdated[providerID], provenanceIndex)
		}
		result.ProviderResults[providerID] = providerResult
		result.ProvidersChanged++
	}

	return result
}

func hasModelsDevSource(activeSources []sources.ID) bool {
	for _, source := range activeSources {
		if source == sources.ModelsDevHTTPID || source == sources.ModelsDevGitID {
			return true
		}
	}
	return false
}

func countEnrichmentUpdates(updates []differ.ModelUpdate, provenanceIndex map[catalogs.ProviderID]map[string]map[string][]provenance.Provenance) int {
	count := 0
	for _, update := range updates {
		provenanceByField := provenanceIndex[update.ProviderID][update.ID]
		if len(provenanceByField) == 0 {
			provenanceByField = provenanceIndex[""][update.ID]
		}
		if hasEnrichmentChange(update, provenanceByField) {
			count++
		}
	}
	return count
}

func hasEnrichmentChange(update differ.ModelUpdate, provenanceByField map[string][]provenance.Provenance) bool {
	for _, change := range update.Changes {
		if isModelsDevSource(change.Source) && isEnrichmentField(change.Path) {
			return true
		}
		if provenanceIsModelsDev(provenanceByField, change.Path) && isEnrichmentField(change.Path) {
			return true
		}
	}
	return false
}

func indexModelProvenance(fieldProvenance provenance.Map, providerIDs map[catalogs.ProviderID]bool) map[catalogs.ProviderID]map[string]map[string][]provenance.Provenance {
	index := make(map[catalogs.ProviderID]map[string]map[string][]provenance.Provenance)
	if len(fieldProvenance) == 0 {
		return index
	}

	for key, entries := range fieldProvenance {
		providerID, modelID, field, ok := splitModelProvenanceKey(key, providerIDs)
		if !ok {
			continue
		}
		if index[providerID] == nil {
			index[providerID] = make(map[string]map[string][]provenance.Provenance)
		}
		if index[providerID][modelID] == nil {
			index[providerID][modelID] = make(map[string][]provenance.Provenance)
		}
		index[providerID][modelID][field] = entries
	}
	return index
}

func splitModelProvenanceKey(key string, providerIDs map[catalogs.ProviderID]bool) (catalogs.ProviderID, string, string, bool) {
	if rest, ok := strings.CutPrefix(key, "model:"); ok {
		modelID, field, found := strings.Cut(rest, ":")
		return "", modelID, field, found && modelID != "" && field != ""
	}

	rest, ok := strings.CutPrefix(key, "models.")
	if !ok {
		return "", "", "", false
	}
	for _, field := range modelProvenanceFieldSuffixes {
		suffix := "." + field
		if strings.HasSuffix(rest, suffix) {
			modelKey := strings.TrimSuffix(rest, suffix)
			providerID, modelID := splitProviderScopedModelKey(modelKey, providerIDs)
			return providerID, modelID, field, modelID != ""
		}
	}
	return "", "", "", false
}

func splitProviderScopedModelKey(modelKey string, providerIDs map[catalogs.ProviderID]bool) (catalogs.ProviderID, string) {
	provider, modelID, ok := strings.Cut(modelKey, ".")
	providerID := catalogs.ProviderID(provider)
	if ok && providerIDs[providerID] {
		return providerID, modelID
	}
	return "", modelKey
}

func provenanceIsModelsDev(fields map[string][]provenance.Provenance, changePath string) bool {
	if len(fields) == 0 {
		return false
	}
	for field, entries := range fields {
		if !provenanceFieldMatchesChange(field, changePath) || len(entries) == 0 {
			continue
		}
		if isModelsDevSource(entries[0].Source) {
			return true
		}
	}
	return false
}

func provenanceFieldMatchesChange(field, changePath string) bool {
	field = normalizeProvenancePath(field)
	changePath = normalizeProvenancePath(changePath)
	return field == changePath ||
		strings.HasPrefix(changePath, field+".") ||
		strings.HasPrefix(field, changePath+".")
}

func normalizeProvenancePath(path string) string {
	switch path {
	case "Delivery":
		return "response"
	case "ReasoningTokens":
		return "reasoning_tokens"
	default:
		return strings.ToLower(path)
	}
}

func isEnrichmentField(path string) bool {
	return path == "description" ||
		strings.HasPrefix(path, "features") ||
		strings.HasPrefix(path, "pricing") ||
		strings.HasPrefix(path, "limits") ||
		strings.HasPrefix(path, "metadata") ||
		path == "attachments" ||
		path == "generation" ||
		path == "reasoning" ||
		path == "reasoning_tokens" ||
		path == "verbosity" ||
		path == "tools" ||
		path == "response"
}

func isModelsDevSource(source sources.ID) bool {
	return source == sources.ModelsDevHTTPID || source == sources.ModelsDevGitID
}

func normalizeChangeset(changeset *differ.Changeset) *differ.Changeset {
	if changeset == nil {
		changeset = &differ.Changeset{}
	}
	if changeset.Models == nil {
		changeset.Models = &differ.ModelChangeset{}
	}
	if changeset.Providers == nil {
		changeset.Providers = &differ.ProviderChangeset{}
	}
	if changeset.Authors == nil {
		changeset.Authors = &differ.AuthorChangeset{}
	}
	return changeset
}

func getModelUpdateProvider(update differ.ModelUpdate, modelProviderMap map[string]catalogs.ProviderID) catalogs.ProviderID {
	if update.ProviderID != "" {
		return update.ProviderID
	}
	return getModelProvider(update.New, modelProviderMap)
}

func getModelChangeProvider(change differ.ModelChange, modelProviderMap map[string]catalogs.ProviderID) catalogs.ProviderID {
	if change.ProviderID != "" {
		return change.ProviderID
	}
	return getModelProvider(change.Model, modelProviderMap)
}

// getModelProvider extracts the provider ID from a model using the provider map.
func getModelProvider(model catalogs.Model, modelProviderMap map[string]catalogs.ProviderID) catalogs.ProviderID {
	// Use the model-to-provider map (should always succeed now)
	if providerID, ok := modelProviderMap[model.ID]; ok {
		return providerID
	}

	// This should never happen since the map is built from the final catalog
	// If it does, it indicates a bug in the reconciler
	return "unknown"
}
