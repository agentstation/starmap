package pipeline

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/reconciler"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestEmbeddedRevisionUpdatesGeneratedFieldsAndPreservesHumanEdit(t *testing.T) {
	t.Parallel()

	embeddedE1 := embeddedRevisionCatalog(t, "Embedded Name E1", "Embedded Description E1", 8192, 0)
	e1Observation := completeObservation(
		t,
		sources.EmbeddedCatalogID,
		embeddedE1,
		time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC),
	)
	e1Result, err := reconcile(context.Background(), asSnapshot(catalogs.NewEmpty()), []sources.Observation{e1Observation})
	if err != nil {
		t.Fatalf("reconcile E1: %v", err)
	}
	workspacePath := t.TempDir()
	if err := e1Result.Catalog.SaveTo(workspacePath); err != nil {
		t.Fatalf("save E1 workspace: %v", err)
	}

	human, err := loadHumanWorkspace(workspacePath)
	if err != nil {
		t.Fatalf("load human workspace: %v", err)
	}
	provider, err := human.Provider("provider-a")
	if err != nil {
		t.Fatalf("human provider: %v", err)
	}
	provider.Models["model-a"].Name = "Human Name"
	if err := human.SetProvider(provider); err != nil {
		t.Fatalf("set human provider: %v", err)
	}
	if err := human.SaveTo(workspacePath); err != nil {
		t.Fatalf("save human edit: %v", err)
	}

	embeddedE2 := embeddedRevisionCatalog(t, "Embedded Name E2", "Embedded Description E2", 16384, 4096)
	store := &pipelineTestStore{catalog: buildCatalog(t, e1Result.Catalog)}
	runner := New(store)
	runner.loadEmbedded = func() (*catalogs.Builder, error) {
		return catalogs.NewBuilderFrom(embeddedE2)
	}

	result, err := runner.Sync(
		context.Background(),
		pkgsync.WithCatalogPath(workspacePath),
		pkgsync.WithSources(sources.LocalCatalogID),
	)
	if err != nil {
		t.Fatalf("Sync E2: %v", err)
	}
	if store.applyCalls != 1 || store.appliedCatalog == nil {
		t.Fatalf("E2 apply = %d, catalog %#v", store.applyCalls, store.appliedCatalog)
	}
	if got := result.Sources; len(got) != 2 ||
		!slices.Contains(got, sources.LocalCatalogID) ||
		!slices.Contains(got, sources.EmbeddedCatalogID) {
		t.Fatalf("E2 sources = %v, want local and embedded", got)
	}

	upgraded := buildCatalog(t, store.appliedCatalog)
	upgradedProvider, err := upgraded.Provider("provider-a")
	if err != nil {
		t.Fatalf("upgraded provider: %v", err)
	}
	model := upgradedProvider.Models["model-a"]
	if model == nil {
		t.Fatal("upgraded model is missing")
	}
	if model.Name != "Human Name" {
		t.Fatalf("human name = %q, want preserved", model.Name)
	}
	if model.Description != "Embedded Description E2" {
		t.Fatalf("description = %q, want E2", model.Description)
	}
	if model.Limits == nil ||
		model.Limits.ContextWindow != 16384 ||
		model.Limits.OutputTokens != 4096 {
		t.Fatalf("limits = %#v, want E2 update and gap fill", model.Limits)
	}
	assertUpgradeEvidenceSource(t, upgraded, "Name", sources.LocalCatalogID)
	assertUpgradeEvidenceSource(t, upgraded, "Description", sources.EmbeddedCatalogID)
	assertUpgradeEvidenceSource(t, upgraded, "limits.context_window", sources.EmbeddedCatalogID)
	assertUpgradeEvidenceSource(t, upgraded, "limits.output_tokens", sources.EmbeddedCatalogID)

	payload, err := catalogs.EncodeCatalogPayload(upgraded)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	identity := workspace.Identity{
		GenerationID:    "embedded-e2-test",
		PayloadChecksum: catalogs.DescribeCatalogPayload(payload).Checksum,
	}
	receipt, err := workspace.ProjectExpected(
		context.Background(),
		workspacePath,
		upgraded,
		identity,
		store.workspaceInput,
	)
	if err != nil {
		t.Fatalf("project upgraded workspace: %v", err)
	}
	if receipt.WorkspaceChecksum == "" {
		t.Fatal("upgraded workspace receipt omitted semantic checksum")
	}
	reloaded, err := loadHumanWorkspace(workspacePath)
	if err != nil {
		t.Fatalf("reload upgraded workspace: %v", err)
	}
	reloadedCatalog := buildCatalog(t, reloaded)
	reloadedProvider, err := reloadedCatalog.Provider("provider-a")
	if err != nil {
		t.Fatalf("reloaded provider: %v", err)
	}
	reloadedModel := reloadedProvider.Models["model-a"]
	if reloadedModel == nil ||
		reloadedModel.Name != "Human Name" ||
		reloadedModel.Description != "Embedded Description E2" ||
		reloadedModel.Limits == nil ||
		reloadedModel.Limits.ContextWindow != 16384 ||
		reloadedModel.Limits.OutputTokens != 4096 {
		t.Fatalf("reloaded upgraded model = %#v", reloadedModel)
	}
	assertUpgradeEvidenceSource(t, reloadedCatalog, "Name", sources.LocalCatalogID)
	assertUpgradeEvidenceSource(t, reloadedCatalog, "Description", sources.EmbeddedCatalogID)
}

func TestEmbeddedRevisionLoadFailurePreservesExistingWorkspace(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	human := catalogs.NewEmpty()
	if err := human.SetProvider(catalogs.Provider{
		ID: "provider-a",
		Models: map[string]*catalogs.Model{
			"model-a": {ID: "model-a", Name: "Human Model"},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	if err := human.SaveTo(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	store := &pipelineTestStore{catalog: buildCatalog(t, human)}
	runner := New(store)
	injected := stderrors.New("embedded revision unavailable")
	runner.loadEmbedded = func() (*catalogs.Builder, error) {
		return nil, injected
	}

	if _, err := runner.Sync(
		context.Background(),
		pkgsync.WithCatalogPath(path),
		pkgsync.WithSources(sources.LocalCatalogID),
	); !stderrors.Is(err, injected) {
		t.Fatalf("Sync error = %v, want injected failure", err)
	}
	if store.applyCalls != 0 {
		t.Fatalf("apply calls = %d, want zero", store.applyCalls)
	}
	reloaded, err := loadHumanWorkspace(path)
	if err != nil {
		t.Fatalf("reload workspace: %v", err)
	}
	provider, err := buildCatalog(t, reloaded).Provider("provider-a")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if model := provider.Models["model-a"]; model == nil || model.Name != "Human Model" {
		t.Fatalf("workspace model = %#v, want unchanged", model)
	}
}

func TestWorkspaceFormattingChangesDoNotBecomeLocalEvidence(t *testing.T) {
	t.Parallel()

	embedded := embeddedRevisionCatalog(t, "Embedded Name", "Embedded Description", 8192, 4096)
	observation := completeObservation(
		t,
		sources.EmbeddedCatalogID,
		embedded,
		time.Date(2026, time.July, 28, 13, 0, 0, 0, time.UTC),
	)
	generated, err := reconcile(
		context.Background(),
		asSnapshot(catalogs.NewEmpty()),
		[]sources.Observation{observation},
	)
	if err != nil {
		t.Fatalf("reconcile embedded catalog: %v", err)
	}
	path := t.TempDir()
	if err := generated.Catalog.SaveTo(path); err != nil {
		t.Fatalf("save generated workspace: %v", err)
	}

	modelPath := filepath.Join(path, "providers", "provider-a", "models", "model-a.yaml")
	data, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	const generatedHeader = "id: model-a\nmodel: test-author/provider-a--model-a\nname: Embedded Name\n"
	if !strings.Contains(string(data), generatedHeader) {
		t.Fatalf("generated model omitted expected header:\n%s", data)
	}
	formatted := "# retained operator comment\n" + strings.Replace(
		string(data),
		generatedHeader,
		"name: 'Embedded Name'\nmodel: test-author/provider-a--model-a\nid: model-a\n",
		1,
	)
	if err := os.WriteFile(modelPath, []byte(formatted), constants.FilePermissions); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	store := &pipelineTestStore{catalog: buildCatalog(t, generated.Catalog)}
	runner := New(store)
	runner.loadEmbedded = func() (*catalogs.Builder, error) {
		return catalogs.NewBuilderFrom(embedded)
	}
	var candidate *reconciler.Result
	runner.reconcile = func(
		ctx context.Context,
		baseline *catalogs.Catalog,
		observations []sources.Observation,
	) (*reconciler.Result, error) {
		result, reconcileErr := reconcile(ctx, baseline, observations)
		candidate = result
		return result, reconcileErr
	}

	result, err := runner.Sync(
		context.Background(),
		pkgsync.WithCatalogPath(path),
		pkgsync.WithSources(sources.LocalCatalogID),
	)
	if err != nil {
		t.Fatalf("Sync formatting-only workspace: %v", err)
	}
	if result.HasChanges() || store.applyCalls != 0 {
		t.Fatalf("formatting-only sync changed catalog: result %#v, applies %d", result, store.applyCalls)
	}
	if candidate == nil {
		t.Fatal("reconciliation candidate was not captured")
	}
	reconciled := buildCatalog(t, candidate.Catalog)
	assertUpgradeEvidenceSource(t, reconciled, "Name", sources.EmbeddedCatalogID)
	assertUpgradeEvidenceSource(t, reconciled, "Description", sources.EmbeddedCatalogID)
}

func TestWorkspaceChangesRequireExplicitReloadAndPublishOnce(t *testing.T) {
	t.Parallel()

	embedded := embeddedRevisionCatalog(t, "Embedded Name", "Embedded Description", 8192, 4096)
	observation := completeObservation(
		t,
		sources.EmbeddedCatalogID,
		embedded,
		time.Date(2026, time.July, 28, 14, 0, 0, 0, time.UTC),
	)
	generated, err := reconcile(
		context.Background(),
		asSnapshot(catalogs.NewEmpty()),
		[]sources.Observation{observation},
	)
	if err != nil {
		t.Fatalf("reconcile embedded catalog: %v", err)
	}
	path := t.TempDir()
	if err := generated.Catalog.SaveTo(path); err != nil {
		t.Fatalf("save generated workspace: %v", err)
	}

	human, err := loadHumanWorkspace(path)
	if err != nil {
		t.Fatalf("load human workspace: %v", err)
	}
	provider, err := human.Provider("provider-a")
	if err != nil {
		t.Fatalf("human provider: %v", err)
	}
	provider.Models["model-a"].Name = "Human Reload Name"
	if err := human.SetProvider(provider); err != nil {
		t.Fatalf("set human provider: %v", err)
	}
	if err := human.SaveTo(path); err != nil {
		t.Fatalf("save human edit: %v", err)
	}

	baseline := buildCatalog(t, generated.Catalog)
	store := &pipelineTestStore{catalog: baseline}
	if got, err := store.catalog.Provider("provider-a"); err != nil {
		t.Fatalf("baseline provider: %v", err)
	} else if got.Models["model-a"].Name != "Embedded Name" {
		t.Fatalf("workspace edit was implicitly published: %#v", got.Models["model-a"])
	}

	runner := New(store)
	runner.loadEmbedded = func() (*catalogs.Builder, error) {
		return catalogs.NewBuilderFrom(embedded)
	}
	result, err := runner.Sync(
		context.Background(),
		pkgsync.WithCatalogPath(path),
		pkgsync.WithSources(sources.LocalCatalogID),
	)
	if err != nil {
		t.Fatalf("explicit local Sync: %v", err)
	}
	if !result.HasChanges() || store.applyCalls != 1 || store.appliedCatalog == nil {
		t.Fatalf("semantic reload result = %#v, apply calls %d", result, store.applyCalls)
	}
	if store.workspaceInput.Checksum == "" {
		t.Fatal("semantic reload did not bind its pre-candidate workspace digest")
	}
	published := buildCatalog(t, store.appliedCatalog)
	if got, err := published.Provider("provider-a"); err != nil {
		t.Fatalf("published provider: %v", err)
	} else if got.Models["model-a"].Name != "Human Reload Name" {
		t.Fatalf("published model = %#v, want human edit", got.Models["model-a"])
	}
	payload, err := catalogs.EncodeCatalogPayload(published)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	if _, err := workspace.ProjectExpected(
		context.Background(),
		path,
		published,
		workspace.Identity{
			GenerationID:    "explicit-reload",
			PayloadChecksum: catalogs.DescribeCatalogPayload(payload).Checksum,
		},
		store.workspaceInput,
	); err != nil {
		t.Fatalf("project explicit reload: %v", err)
	}
	store.catalog = published

	repeated, err := runner.Sync(
		context.Background(),
		pkgsync.WithCatalogPath(path),
		pkgsync.WithSources(sources.LocalCatalogID),
	)
	if err != nil {
		t.Fatalf("repeat local Sync: %v", err)
	}
	if repeated.HasChanges() || store.applyCalls != 1 {
		t.Fatalf("unchanged reload republished: result %#v, apply calls %d", repeated, store.applyCalls)
	}
}

func embeddedRevisionCatalog(
	t testing.TB,
	name, description string,
	contextWindow, outputTokens int64,
) *catalogs.Catalog {
	t.Helper()

	model := &catalogs.Model{
		ID:          "model-a",
		Name:        name,
		Description: description,
		Limits: &catalogs.ModelLimits{
			ContextWindow: contextWindow,
		},
	}
	if outputTokens > 0 {
		model.Limits.OutputTokens = outputTokens
	}
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{
		ID:     "provider-a",
		Name:   "Provider A",
		Models: map[string]*catalogs.Model{model.ID: model},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	return buildCatalog(t, builder)
}

func completeObservation(
	t testing.TB,
	sourceID sources.ID,
	catalog *catalogs.Catalog,
	observedAt time.Time,
) sources.Observation {
	t.Helper()

	observation, err := sources.NewObservation(sourceID, catalog, sources.ObservationMetadata{
		ObservedAt:   observedAt,
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
		Records:      sources.ObservationRecordCounts{Accepted: 1},
	})
	if err != nil {
		t.Fatalf("NewObservation(%s): %v", sourceID, err)
	}
	return observation
}

func buildCatalog(t testing.TB, builder *catalogs.Builder) *catalogs.Catalog {
	t.Helper()

	if err := completePipelineTestCatalog(builder); err != nil {
		t.Fatalf("Complete test catalog: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return catalog
}

func assertUpgradeEvidenceSource(
	t testing.TB,
	catalog *catalogs.Catalog,
	field string,
	want sources.ID,
) {
	t.Helper()

	entries := catalog.Provenance().FindModelField("provider-a", "model-a", field)
	if len(entries) == 0 {
		t.Fatalf("%s evidence is missing", field)
	}
	current := entries[0]
	for _, entry := range entries[1:] {
		if entry.Timestamp.After(current.Timestamp) {
			current = entry
		}
	}
	if current.Source != want {
		t.Fatalf("%s evidence source = %q, want %q: %#v", field, current.Source, want, entries)
	}
}
