package workspace

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestProjectAtomicallyReplacesValidatedWorkspace(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "catalog")
	oldCatalog, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(context.Background(), path, oldCatalog, oldIdentity); err != nil {
		t.Fatalf("Project old catalog: %v", err)
	}
	notesPath := filepath.Join(path, "notes.txt")
	if err := os.WriteFile(notesPath, []byte("operator notes\n"), constants.FilePermissions); err != nil {
		t.Fatalf("Write unmanaged file: %v", err)
	}

	newCatalog, newIdentity := testCatalog(t, "new", "New Model")
	receipt, err := Project(context.Background(), path, newCatalog, newIdentity)
	if err != nil {
		t.Fatalf("Project new catalog: %v", err)
	}
	if receipt.GenerationID != newIdentity.GenerationID {
		t.Fatalf("GenerationID = %q, want %q", receipt.GenerationID, newIdentity.GenerationID)
	}
	if receipt.WorkspaceChecksum != newIdentity.PayloadChecksum {
		t.Fatalf("WorkspaceChecksum = %q, want %q", receipt.WorkspaceChecksum, newIdentity.PayloadChecksum)
	}
	assertWorkspaceModel(t, path, "new", "New Model")
	assertWorkspaceModelMissing(t, path, "old")
	if data, err := os.ReadFile(notesPath); err != nil || string(data) != "operator notes\n" {
		t.Fatalf("Unmanaged file = %q, %v; want preserved", data, err)
	}

	marker, err := readProjectionMarker(path)
	if err != nil {
		t.Fatalf("Read marker: %v", err)
	}
	markerPath := projectionMarkerPath(path)
	if pathsOverlap(path, markerPath) {
		t.Fatalf("machine projection marker %q is inside human workspace %q", markerPath, path)
	}
	if marker.GenerationID != newIdentity.GenerationID ||
		marker.PayloadChecksum != newIdentity.PayloadChecksum ||
		marker.WorkspaceChecksum != receipt.WorkspaceChecksum {
		t.Fatalf("Marker = %#v, want identity %#v and receipt %#v", marker, newIdentity, receipt)
	}
	assertNoProjectionStaging(t, path)
}

func TestProjectFailureBeforePromotePreservesOldWorkspace(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "catalog")
	oldCatalog, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(context.Background(), path, oldCatalog, oldIdentity); err != nil {
		t.Fatalf("Project old catalog: %v", err)
	}

	injected := stderrors.New("injected promotion failure")
	newCatalog, newIdentity := testCatalog(t, "new", "New Model")
	_, err := (projector{beforePromote: func() error { return injected }}).
		project(context.Background(), path, newCatalog, newIdentity, InputExpectation{})
	if !stderrors.Is(err, injected) {
		t.Fatalf("Project error = %v, want injected failure", err)
	}
	assertWorkspaceModel(t, path, "old", "Old Model")
	assertWorkspaceModelMissing(t, path, "new")
	assertNoProjectionStaging(t, path)
}

func TestFirstProjectionFailureLeavesWorkspaceAbsent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "catalog")
	input, err := ObserveInput(path)
	if err != nil {
		t.Fatalf("ObserveInput: %v", err)
	}
	if !input.RequiresSeed() {
		t.Fatalf("input = %#v, want absent workspace seed", input)
	}
	catalog, identity := testCatalog(t, "seed", "Embedded Seed")
	injected := stderrors.New("injected before first promotion")
	_, err = (projector{beforePromote: func() error { return injected }}).
		project(context.Background(), path, catalog, identity, input)
	if !stderrors.Is(err, injected) {
		t.Fatalf("Project error = %v, want injected failure", err)
	}
	if _, statErr := os.Lstat(path); !stderrors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("workspace exists after failed first projection: %v", statErr)
	}
	assertNoProjectionStaging(t, path)
}

func TestFirstProjectionRejectsWorkspaceCreatedAfterObservation(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "catalog")
	input, err := ObserveInput(path)
	if err != nil {
		t.Fatalf("ObserveInput: %v", err)
	}
	if err := os.MkdirAll(path, constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	operatorNote := filepath.Join(path, "operator.txt")
	if err := os.WriteFile(operatorNote, []byte("do not overwrite\n"), constants.FilePermissions); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	catalog, identity := testCatalog(t, "seed", "Embedded Seed")
	_, err = ProjectExpected(context.Background(), path, catalog, identity, input)
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("Project error = %T %v, want *errors.ConflictError", err, err)
	}
	data, readErr := os.ReadFile(operatorNote)
	if readErr != nil || string(data) != "do not overwrite\n" {
		t.Fatalf("operator file = %q, %v", data, readErr)
	}
	assertWorkspaceModelMissing(t, path, "seed")
	assertNoProjectionStaging(t, path)
}

func TestProjectExpectedRejectsSemanticEditAfterInputLoad(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "catalog")
	oldCatalog, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(context.Background(), path, oldCatalog, oldIdentity); err != nil {
		t.Fatalf("Project old catalog: %v", err)
	}
	input, err := ObserveInput(path)
	if err != nil {
		t.Fatalf("ObserveInput: %v", err)
	}
	loaded, err := catalogs.NewFromPath(path)
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	input, err = BindInputCatalog(input, mustCatalog(t, loaded))
	if err != nil {
		t.Fatalf("BindInputCatalog: %v", err)
	}

	editWorkspaceModel(t, path, "old", "Human Edit Before Projection")
	newCatalog, newIdentity := testCatalog(t, "new", "New Model")
	_, err = ProjectExpected(context.Background(), path, newCatalog, newIdentity, input)
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("ProjectExpected error = %T %v, want *errors.ConflictError", err, err)
	}
	assertWorkspaceModel(t, path, "old", "Human Edit Before Projection")
	assertWorkspaceModelMissing(t, path, "new")
	assertNoProjectionStaging(t, path)
}

func TestProjectRejectsSymlinkedWriterLock(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "catalog")
	operatorFile := filepath.Join(root, "operator.txt")
	if err := os.WriteFile(operatorFile, []byte("preserve\n"), constants.FilePermissions); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Symlink(operatorFile, writerLockPath(path)); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	catalog, identity := testCatalog(t, "seed", "Embedded Seed")
	_, err := Project(context.Background(), path, catalog, identity)
	var validation *errors.ValidationError
	if !stderrors.As(err, &validation) {
		t.Fatalf("Project error = %T %v, want *errors.ValidationError", err, err)
	}
	data, readErr := os.ReadFile(operatorFile)
	if readErr != nil || string(data) != "preserve\n" {
		t.Fatalf("operator file = %q, %v", data, readErr)
	}
}

func TestProjectRejectsCatalogThatDoesNotMatchCommittedPayload(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "catalog")
	oldCatalog, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(context.Background(), path, oldCatalog, oldIdentity); err != nil {
		t.Fatalf("Project old catalog: %v", err)
	}

	newCatalog, newIdentity := testCatalog(t, "new", "New Model")
	newIdentity.PayloadChecksum = "sha256:does-not-match"
	_, err := Project(context.Background(), path, newCatalog, newIdentity)
	var validation *errors.ValidationError
	if !stderrors.As(err, &validation) {
		t.Fatalf("Project error = %T %v, want *errors.ValidationError", err, err)
	}
	assertWorkspaceModel(t, path, "old", "Old Model")
	assertWorkspaceModelMissing(t, path, "new")
}

func TestProjectRejectsConcurrentSemanticEdit(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "catalog")
	oldCatalog, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(context.Background(), path, oldCatalog, oldIdentity); err != nil {
		t.Fatalf("Project old catalog: %v", err)
	}

	newCatalog, newIdentity := testCatalog(t, "new", "New Model")
	_, err := (projector{beforeInputCheck: func() error {
		editWorkspaceModel(t, path, "old", "Human Edit")
		return nil
	}}).project(context.Background(), path, newCatalog, newIdentity, InputExpectation{})
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("Project error = %T %v, want *errors.ConflictError", err, err)
	}
	assertWorkspaceModel(t, path, "old", "Human Edit")
	assertWorkspaceModelMissing(t, path, "new")
}

func TestProjectSeparatesCatalogAndGeneratedEndpointDigests(t *testing.T) {
	t.Parallel()

	builder := catalogs.NewEmpty()
	model := &catalogs.Model{ID: "org/model", Name: "Hierarchical Model"}
	if err := builder.SetProvider(catalogs.Provider{
		ID: "test-provider", Name: "Test Provider",
		Models: map[string]*catalogs.Model{model.ID: model},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	if err := builder.SetAuthor(catalogs.Author{
		ID: "test-author", Name: "Test Author",
		Catalog: &catalogs.AuthorCatalog{
			Attribution: &catalogs.AuthorAttribution{ProviderID: "test-provider"},
		},
	}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	model.ModelRef = "test-author/org--model"
	if err := builder.SetProviderModel("test-provider", *model); err != nil {
		t.Fatalf("SetProviderModel: %v", err)
	}
	if err := builder.SetAuthorModel("test-author", catalogs.Model{
		ID:      "org--model",
		Name:    model.Name,
		Authors: []catalogs.Author{{ID: "test-author", Name: "Test Author"}},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	identity := Identity{
		GenerationID:    "generation-with-derived-views",
		PayloadChecksum: catalogs.DescribeCatalogPayload(payload).Checksum,
	}

	path := filepath.Join(t.TempDir(), "catalog")
	receipt, err := Project(context.Background(), path, catalog, identity)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if receipt.WorkspaceChecksum != identity.PayloadChecksum {
		t.Fatalf("workspace checksum = %q, want committed payload %q",
			receipt.WorkspaceChecksum, identity.PayloadChecksum)
	}
	if receipt.EndpointChecksum == "" || receipt.EndpointChecksum == receipt.WorkspaceChecksum {
		t.Fatalf("endpoint checksum = %q, workspace checksum = %q; want distinct bound projections",
			receipt.EndpointChecksum, receipt.WorkspaceChecksum)
	}
	projected, err := catalogs.NewFromPath(path)
	if err != nil {
		t.Fatalf("Load projection: %v", err)
	}
	projectedCatalog, err := projected.Build()
	if err != nil {
		t.Fatalf("Build projection: %v", err)
	}
	if _, err := projectedCatalog.Offering("test-provider", catalogs.ProviderModelID(model.ID)); err != nil {
		t.Fatalf("canonical provider model missing: %v", err)
	}
	authorModels, err := projectedCatalog.AuthorModels("test-author")
	if err != nil {
		t.Fatalf("AuthorModels: %v", err)
	}
	if len(authorModels) != 1 || authorModels[0].ID != "test-author/org--model" {
		t.Fatalf("derived author models = %#v, want %q", authorModels, model.ID)
	}
}

func TestRepairStaleUnmodifiedProjection(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "catalog")
	oldCatalog, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(context.Background(), path, oldCatalog, oldIdentity); err != nil {
		t.Fatalf("Project old catalog: %v", err)
	}

	newCatalog, newIdentity := testCatalog(t, "new", "New Model")
	result, err := Repair(context.Background(), path, newCatalog, newIdentity)
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if result.Status != RepairStatusRepaired {
		t.Fatalf("Repair status = %q, want %q", result.Status, RepairStatusRepaired)
	}
	assertWorkspaceModel(t, path, "new", "New Model")
	assertWorkspaceModelMissing(t, path, "old")
}

func TestRepairDoesNotOverwriteDirtyWorkspace(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "catalog")
	oldCatalog, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(context.Background(), path, oldCatalog, oldIdentity); err != nil {
		t.Fatalf("Project old catalog: %v", err)
	}
	editWorkspaceModel(t, path, "old", "Human Edit")

	newCatalog, newIdentity := testCatalog(t, "new", "New Model")
	result, err := Repair(context.Background(), path, newCatalog, newIdentity)
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if result.Status != RepairStatusSkippedDirty || result.IssueCode != IssueDirty {
		t.Fatalf("Repair result = %#v, want skipped dirty", result)
	}
	assertWorkspaceModel(t, path, "old", "Human Edit")
	assertWorkspaceModelMissing(t, path, "new")
}

func TestRepairAcknowledgesSwapAfterMarkerFailure(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "catalog")
	oldCatalog, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(context.Background(), path, oldCatalog, oldIdentity); err != nil {
		t.Fatalf("Project old catalog: %v", err)
	}

	injected := stderrors.New("injected marker failure")
	newCatalog, newIdentity := testCatalog(t, "new", "New Model")
	receipt, err := (projector{beforeMarker: func() error { return injected }}).
		project(context.Background(), path, newCatalog, newIdentity, InputExpectation{})
	if !stderrors.Is(err, injected) {
		t.Fatalf("Project error = %v, want injected marker failure", err)
	}
	if receipt.GenerationID != newIdentity.GenerationID {
		t.Fatalf("Receipt = %#v, want generation %q", receipt, newIdentity.GenerationID)
	}
	assertWorkspaceModel(t, path, "new", "New Model")

	result, err := Repair(context.Background(), path, newCatalog, newIdentity)
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if result.Status != RepairStatusRepaired {
		t.Fatalf("Repair status = %q, want %q", result.Status, RepairStatusRepaired)
	}
	marker, err := readProjectionMarker(path)
	if err != nil {
		t.Fatalf("Read repaired marker: %v", err)
	}
	if marker.GenerationID != newIdentity.GenerationID {
		t.Fatalf("Marker generation = %q, want %q", marker.GenerationID, newIdentity.GenerationID)
	}
}

func TestProjectRejectsWorkspaceSymlink(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	realPath := filepath.Join(parent, "real")
	if err := os.Mkdir(realPath, constants.DirPermissions); err != nil {
		t.Fatalf("Mkdir real workspace: %v", err)
	}
	path := filepath.Join(parent, "catalog")
	if err := os.Symlink(realPath, path); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	catalog, identity := testCatalog(t, "new", "New Model")
	_, err := Project(context.Background(), path, catalog, identity)
	var validation *errors.ValidationError
	if !stderrors.As(err, &validation) {
		t.Fatalf("Project error = %T %v, want *errors.ValidationError", err, err)
	}
	if _, statErr := os.Lstat(path); statErr != nil {
		t.Fatalf("Workspace symlink was changed: %v", statErr)
	}
}

func testCatalog(t *testing.T, modelID, modelName string) (*catalogs.Catalog, Identity) {
	t.Helper()

	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{
		ID:   "test-provider",
		Name: "Test Provider",
		Models: map[string]*catalogs.Model{
			modelID: {ID: modelID, Name: modelName},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	completeWorkspaceTestCatalog(t, builder)
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	return catalog, Identity{
		GenerationID:    "generation-" + modelID,
		PayloadChecksum: catalogs.DescribeCatalogPayload(payload).Checksum,
	}
}

func mustCatalog(t testing.TB, builder *catalogs.Builder) *catalogs.Catalog {
	t.Helper()
	completeWorkspaceTestCatalog(t, builder)
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return catalog
}

func completeWorkspaceTestCatalog(t testing.TB, builder *catalogs.Builder) {
	t.Helper()
	author := catalogs.Author{ID: "test-author", Name: "Test Author"}
	authorSet := false
	for _, provider := range builder.Providers().List() {
		for modelID, model := range provider.Models {
			if model == nil || model.ModelRef != "" {
				continue
			}
			if !authorSet {
				if err := builder.SetAuthor(author); err != nil {
					t.Fatalf("SetAuthor: %v", err)
				}
				authorSet = true
			}
			slug := strings.ReplaceAll(string(provider.ID)+"--"+modelID, "/", "--")
			model.ModelRef = catalogs.AuthoredModelID(author.ID, slug)
			if err := builder.SetProviderModel(provider.ID, *model); err != nil {
				t.Fatalf("SetProviderModel(%s/%s): %v", provider.ID, modelID, err)
			}
			if err := builder.SetAuthorModel(author.ID, catalogs.Model{
				ID:      slug,
				Name:    model.Name,
				Authors: []catalogs.Author{author},
			}); err != nil {
				t.Fatalf("SetAuthorModel(%s): %v", model.ModelRef, err)
			}
		}
	}
}

func editWorkspaceModel(t *testing.T, path, modelID, modelName string) {
	t.Helper()

	builder, err := catalogs.NewFromPath(path)
	if err != nil {
		t.Fatalf("Load workspace for edit: %v", err)
	}
	model, err := builder.ProviderModel("test-provider", modelID)
	if err != nil {
		t.Fatalf("ProviderModel: %v", err)
	}
	model.Name = modelName
	if err := builder.SetProviderModel("test-provider", model); err != nil {
		t.Fatalf("SetProviderModel: %v", err)
	}
	if err := builder.SaveTo(path); err != nil {
		t.Fatalf("Save human edit: %v", err)
	}
}

func assertWorkspaceModel(t *testing.T, path, modelID, modelName string) {
	t.Helper()

	builder, err := catalogs.NewFromPath(path)
	if err != nil {
		t.Fatalf("Load workspace: %v", err)
	}
	model, err := builder.ProviderModel("test-provider", modelID)
	if err != nil {
		t.Fatalf("ProviderModel(%q): %v", modelID, err)
	}
	if model.Name != modelName {
		t.Fatalf("ProviderModel(%q).Name = %q, want %q", modelID, model.Name, modelName)
	}
}

func assertWorkspaceModelMissing(t *testing.T, path, modelID string) {
	t.Helper()

	builder, err := catalogs.NewFromPath(path)
	if err != nil {
		t.Fatalf("Load workspace: %v", err)
	}
	if _, err := builder.ProviderModel("test-provider", modelID); err == nil {
		t.Fatalf("ProviderModel(%q) unexpectedly exists", modelID)
	}
}

func assertNoProjectionStaging(t *testing.T, path string) {
	t.Helper()
	parent := filepath.Dir(path)
	base := filepath.Base(path)
	for _, pattern := range []string{
		"." + base + ".candidate-*",
		"." + base + ".candidate-*.verify-*",
	} {
		matches, err := filepath.Glob(filepath.Join(parent, pattern))
		if err != nil {
			t.Fatalf("Glob projection staging: %v", err)
		}
		if len(matches) != 0 {
			t.Fatalf("projection staging survived: %v", matches)
		}
	}
}
