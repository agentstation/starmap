// Package workspace projects committed catalogs into the optional
// human-editable provider YAML workspace.
package workspace

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	stderrors "errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	markerVersion = 2

	// IssueDirty identifies a workspace whose semantic contents differ from the
	// last successfully projected generation.
	IssueDirty = "workspace_modified"
)

// Identity binds a workspace projection to one committed immutable generation.
type Identity struct {
	GenerationID    string
	PayloadChecksum string
}

// Validate verifies that the committed generation identity is complete.
func (i Identity) Validate() error {
	if strings.TrimSpace(i.GenerationID) == "" {
		return &errors.ValidationError{Field: "workspace_projection.generation_id", Message: "is required"}
	}
	if strings.TrimSpace(i.PayloadChecksum) == "" {
		return &errors.ValidationError{Field: "workspace_projection.payload_checksum", Message: "is required"}
	}
	return nil
}

// Receipt records the exact semantic workspace projection that became visible.
type Receipt struct {
	GenerationID      string
	WorkspaceChecksum string
	EndpointChecksum  string
}

// InputExpectation records workspace presence and, once loaded, its semantic
// digest before candidate construction. Projection rejects a different input.
type InputExpectation struct {
	Path             string
	Exists           bool
	Checksum         string
	EndpointChecksum string
}

// ObserveInput records the selected workspace's presence without creating or
// modifying it.
func ObserveInput(path string) (InputExpectation, error) {
	var input InputExpectation
	err := Read(context.Background(), path, func(observed InputExpectation) error {
		input = observed
		return nil
	})
	if err != nil {
		return InputExpectation{}, err
	}
	return input, nil
}

// RequiresSeed reports whether an explicit operation selected an absent human
// workspace. The operation must create that workspace even without fact changes.
func (i InputExpectation) RequiresSeed() bool {
	return i.Path != "" && !i.Exists
}

// BindInputCatalog records the semantic digest of the human catalog loaded
// from an existing workspace before candidate construction.
// Call it inside Read, after loading that catalog in the same callback.
func BindInputCatalog(input InputExpectation, catalog *catalogs.Catalog) (InputExpectation, error) {
	if input.Path == "" || !input.Exists {
		return input, nil
	}
	if catalog == nil {
		return InputExpectation{}, &errors.ValidationError{
			Field:   "workspace_projection.input_catalog",
			Message: "is required for an existing workspace",
		}
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		return InputExpectation{}, errors.WrapResource("encode", "workspace projection input", input.Path, err)
	}
	input.Checksum = catalogs.DescribeCatalogPayload(payload).Checksum
	endpointChecksum, err := readEndpointProjectionChecksum(input.Path)
	if stderrors.Is(err, fs.ErrNotExist) {
		endpointChecksum = ""
	} else if err != nil {
		return InputExpectation{}, errors.WrapIO("read", endpointProjectionFilename, err)
	}
	input.EndpointChecksum = endpointChecksum
	return input, nil
}

// RepairStatus describes startup reconciliation between the durable current
// generation and its optional YAML projection.
type RepairStatus string

const (
	// RepairStatusCurrent means workspace contents and marker already match.
	RepairStatusCurrent RepairStatus = "current"
	// RepairStatusRepaired means startup safely restored or acknowledged the
	// current committed generation without publishing another generation.
	RepairStatusRepaired RepairStatus = "repaired"
	// RepairStatusSkippedDirty means semantic human edits prevented automatic
	// replacement.
	RepairStatusSkippedDirty RepairStatus = "skipped_dirty"
)

// RepairResult reports startup projection reconciliation.
type RepairResult struct {
	Status    RepairStatus
	IssueCode string
}

type projector struct {
	writer                  *workspaceWriter
	recordWrites            workspaceRecordWriter
	beforeInputCheck        func() error
	beforePromote           func() error
	beforeMarker            func() error
	journalReplacement      bool
	afterReplacementPhase   func(replacementPhase) error
	afterStageRender        func(string) error
	afterVerificationRender func(string) error
	beforeAccessRestore     func(string) error
}

// Project stages, validates, syncs, and publishes one workspace.
// Windows uses a recovery journal when replacing an existing workspace.
func Project(ctx context.Context, path string, catalog *catalogs.Catalog, identity Identity) (Receipt, error) {
	return (projector{}).project(ctx, path, catalog, identity, InputExpectation{})
}

// ProjectExpected projects one workspace only if its presence still matches
// the state observed before candidate construction.
func ProjectExpected(
	ctx context.Context,
	path string,
	catalog *catalogs.Catalog,
	identity Identity,
	input InputExpectation,
) (Receipt, error) {
	return (projector{}).project(ctx, path, catalog, identity, input)
}

func (p projector) project(
	ctx context.Context,
	path string,
	catalog *catalogs.Catalog,
	identity Identity,
	expectation InputExpectation,
) (Receipt, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	if catalog == nil {
		return Receipt{}, &errors.ValidationError{Field: "workspace_projection.catalog", Message: "is required"}
	}
	if err := identity.Validate(); err != nil {
		return Receipt{}, err
	}
	if err := validateCommittedCatalog(catalog, identity); err != nil {
		return Receipt{}, err
	}
	target, err := resolveTarget(path)
	if err != nil {
		return Receipt{}, err
	}
	if err := ValidateHumanLayout(target, ""); err != nil {
		return Receipt{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), directoryMode); err != nil {
		return Receipt{}, errors.WrapIO("create", filepath.Dir(target), err)
	}
	writer, err := acquireWorkspaceWriter(target)
	if err != nil {
		return Receipt{}, err
	}
	defer writer.close()
	p.writer = writer
	p.recordWrites.checkWriter = writer.check
	receipt, _, err := p.projectLocked(ctx, target, catalog, identity, expectation)
	return receipt, err
}

func (p projector) projectLocked(
	ctx context.Context,
	target string,
	catalog *catalogs.Catalog,
	identity Identity,
	expectation InputExpectation,
) (Receipt, treeSnapshot, error) {
	if _, err := recoverWorkspace(ctx, target, p.writer); err != nil {
		return Receipt{}, treeSnapshot{}, err
	}
	input, err := readSemanticState(target)
	if err != nil {
		return Receipt{}, treeSnapshot{}, err
	}
	if err := validateInputExpectation(target, input, expectation); err != nil {
		return Receipt{}, treeSnapshot{}, err
	}
	var original treeSnapshot
	if input.exists {
		original, err = snapshotTree(ctx, target)
		if err != nil {
			return Receipt{}, treeSnapshot{}, errors.WrapResource("inspect", "workspace replacement", target, err)
		}
	}
	candidate, stagedState, err := p.stageCatalog(ctx, target, catalog, identity, &original)
	if err != nil {
		return Receipt{}, treeSnapshot{}, err
	}
	receipt, err := p.publishCandidate(ctx, target, identity, input, candidate, stagedState)
	if receipt.GenerationID == "" {
		return receipt, treeSnapshot{}, err
	}
	return receipt, candidate.tree, err
}

func (p projector) publishCandidate(
	ctx context.Context,
	target string,
	identity Identity,
	input semanticState,
	candidate stagedWorkspace,
	stagedState semanticState,
) (result Receipt, resultErr error) {
	original := candidate.original
	journaled := input.exists && (journalWorkspaceReplacement || p.journalReplacement)
	staged := candidate.path
	cleanupStaged := true
	var exchanged treeSnapshot
	defer func() {
		if cleanupStaged {
			resultErr = stderrors.Join(resultErr, candidate.cleanup(ctx, exchanged))
		} else if resultErr == nil {
			resultErr = candidate.cleanup(ctx, treeSnapshot{})
		}
	}()
	if p.beforeInputCheck != nil {
		if err := p.beforeInputCheck(); err != nil {
			return Receipt{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	current, err := readSemanticState(target)
	if err != nil {
		return Receipt{}, err
	}
	if !input.equal(current) {
		return Receipt{}, &errors.ConflictError{
			Resource: "catalog workspace projection",
			Expected: input.describe(),
			Actual:   current.describe(),
			Message:  "workspace changed while the committed generation was being staged",
		}
	}
	if p.beforePromote != nil {
		if err := p.beforePromote(); err != nil {
			return Receipt{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	if err := p.writer.check(); err != nil {
		return Receipt{}, err
	}
	marker := projectionMarker{
		Version: markerVersion, GenerationID: identity.GenerationID,
		PayloadChecksum: identity.PayloadChecksum, WorkspaceChecksum: stagedState.checksum,
		EndpointChecksum: stagedState.endpointChecksum,
	}
	if input.exists && !journaled {
		currentTree, err := snapshotTree(ctx, target)
		if err != nil {
			return Receipt{}, err
		}
		if !sameTree(original, currentTree) {
			return Receipt{}, replacementConflict(target, "workspace content or access changed during staging")
		}
	}
	if journaled {
		if err := candidate.validatePublication(ctx); err != nil {
			return Receipt{}, err
		}
		owned, visible, err := p.replaceWithJournal(ctx, target, staged, original, candidate.tree, marker)
		cleanupStaged = !owned
		var receipt Receipt
		if visible {
			receipt = Receipt{GenerationID: identity.GenerationID, WorkspaceChecksum: stagedState.checksum, EndpointChecksum: stagedState.endpointChecksum}
		}
		if err != nil {
			return receipt, errors.WrapResource("replace", "catalog workspace", target, err)
		}
		return receipt, nil
	}
	if err := p.writer.check(); err != nil {
		return Receipt{}, err
	}
	if err := candidate.validatePublication(ctx); err != nil {
		return Receipt{}, err
	}
	exchanged = original
	if err := promoteDirectory(staged, target, input.exists); err != nil {
		return Receipt{}, err
	}

	receipt := Receipt{
		GenerationID:      identity.GenerationID,
		WorkspaceChecksum: stagedState.checksum,
		EndpointChecksum:  stagedState.endpointChecksum,
	}
	if p.beforeMarker != nil {
		if err := p.beforeMarker(); err != nil {
			return receipt, err
		}
	}
	if err := p.recordWrites.writeProjectionMarker(ctx, target, projectionMarker{
		Version:           markerVersion,
		GenerationID:      identity.GenerationID,
		PayloadChecksum:   identity.PayloadChecksum,
		WorkspaceChecksum: stagedState.checksum,
		EndpointChecksum:  stagedState.endpointChecksum,
	}); err != nil {
		return receipt, err
	}
	return receipt, nil
}

func validateInputExpectation(target string, input semanticState, expectation InputExpectation) error {
	if expectation.Path == "" {
		return nil
	}
	expectedPath, err := resolveTarget(expectation.Path)
	if err != nil {
		return err
	}
	if expectedPath != target {
		return &errors.ValidationError{
			Field:   "workspace_projection.input_path",
			Value:   expectation.Path,
			Message: "does not match the selected workspace path",
		}
	}
	if expectation.Exists == input.exists {
		if !expectation.Exists || expectation.Checksum == "" {
			return nil
		}
		if expectation.Checksum != input.checksum {
			return &errors.ConflictError{
				Resource: "catalog workspace projection input",
				Expected: expectation.Checksum,
				Actual:   input.checksum,
				Message:  "workspace semantics changed after candidate construction",
			}
		}
		if expectation.EndpointChecksum != input.endpointChecksum {
			return &errors.ConflictError{
				Resource: "catalog endpoint projection input",
				Expected: expectation.EndpointChecksum,
				Actual:   input.endpointChecksum,
				Message:  "generated endpoint projection changed after candidate construction",
			}
		}
		return nil
	}
	return &errors.ConflictError{
		Resource: "catalog workspace projection input",
		Expected: describePresence(expectation.Exists),
		Actual:   input.describe(),
		Message:  "workspace presence changed after candidate construction",
	}
}

func describePresence(exists bool) string {
	if exists {
		return "present"
	}
	return "absent"
}

// Repair compares a workspace and its durable marker with current. It repairs
// only a missing workspace or an unchanged prior projection.
func Repair(ctx context.Context, path string, current *catalogs.Catalog, identity Identity) (RepairResult, error) {
	return (projector{}).repair(ctx, path, current, identity)
}

func (p projector) repair(ctx context.Context, path string, current *catalogs.Catalog, identity Identity) (result RepairResult, resultErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return RepairResult{}, err
	}
	if current == nil {
		return RepairResult{}, &errors.ValidationError{Field: "workspace_repair.catalog", Message: "is required"}
	}
	if err := identity.Validate(); err != nil {
		return RepairResult{}, err
	}
	if err := validateCommittedCatalog(current, identity); err != nil {
		return RepairResult{}, err
	}
	target, err := resolveTarget(path)
	if err != nil {
		return RepairResult{}, err
	}
	if err := ValidateHumanLayout(target, ""); err != nil {
		return RepairResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), directoryMode); err != nil {
		return RepairResult{}, errors.WrapIO("create", filepath.Dir(target), err)
	}
	writer, err := acquireWorkspaceWriter(target)
	if err != nil {
		return RepairResult{}, err
	}
	defer writer.close()
	p.writer = writer
	p.recordWrites.checkWriter = writer.check
	recovered, err := recoverWorkspace(ctx, target, p.writer)
	if err != nil {
		return RepairResult{}, err
	}
	state, err := readSemanticState(target)
	if err != nil {
		return RepairResult{}, err
	}
	marker, markerErr := readProjectionMarker(target)
	if markerErr == nil {
		if state.exists &&
			marker.GenerationID == identity.GenerationID &&
			marker.PayloadChecksum == identity.PayloadChecksum &&
			marker.WorkspaceChecksum == state.checksum &&
			marker.EndpointChecksum == state.endpointChecksum {
			if recovered {
				return RepairResult{Status: RepairStatusRepaired}, nil
			}
			return RepairResult{Status: RepairStatusCurrent}, nil
		}
	} else if !stderrors.Is(markerErr, fs.ErrNotExist) {
		return RepairResult{}, markerErr
	}

	candidate, desired, err := p.stageCatalog(ctx, target, current, identity, nil)
	if err != nil {
		return RepairResult{}, err
	}
	cleanupCandidate := true
	defer func() {
		if cleanupCandidate {
			resultErr = stderrors.Join(resultErr, candidate.cleanup(ctx, treeSnapshot{}))
		}
	}()
	if state.equal(desired) {
		if err := p.recordWrites.writeProjectionMarker(ctx, target, projectionMarker{
			Version: markerVersion, GenerationID: identity.GenerationID,
			PayloadChecksum: identity.PayloadChecksum, WorkspaceChecksum: state.checksum,
			EndpointChecksum: state.endpointChecksum,
		}); err != nil {
			return RepairResult{}, err
		}
		return RepairResult{Status: RepairStatusRepaired}, nil
	}
	if markerErr == nil && state.exists &&
		(state.checksum != marker.WorkspaceChecksum ||
			state.endpointChecksum != marker.EndpointChecksum) {
		return RepairResult{Status: RepairStatusSkippedDirty, IssueCode: IssueDirty}, nil
	}
	if stderrors.Is(markerErr, fs.ErrNotExist) && state.exists {
		return RepairResult{Status: RepairStatusSkippedDirty, IssueCode: IssueDirty}, nil
	}

	cleanupCandidate = false
	if _, err := p.publishCandidate(ctx, target, identity, state, candidate, desired); err != nil {
		return RepairResult{}, err
	}
	return RepairResult{Status: RepairStatusRepaired}, nil
}

type semanticState struct {
	exists           bool
	checksum         string
	endpointChecksum string
}

func validateCommittedCatalog(catalog *catalogs.Catalog, identity Identity) error {
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		return errors.WrapResource("encode", "committed catalog", identity.GenerationID, err)
	}
	checksum := catalogs.DescribeCatalogPayload(payload).Checksum
	if checksum != identity.PayloadChecksum {
		return &errors.ValidationError{
			Field: "workspace_projection.payload_checksum", Value: checksum,
			Message: "catalog does not match the committed generation",
		}
	}
	return nil
}

func (s semanticState) equal(other semanticState) bool {
	return s.exists == other.exists &&
		s.checksum == other.checksum &&
		s.endpointChecksum == other.endpointChecksum
}

func (s semanticState) describe() string {
	if !s.exists {
		return "absent"
	}
	return s.checksum + " endpoints=" + s.endpointChecksum
}

func resolveTarget(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", &errors.ValidationError{Field: "workspace_projection.path", Message: "is required"}
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return "", errors.WrapIO("resolve", path, err)
	}
	if target == filepath.Dir(target) {
		return "", &errors.ValidationError{Field: "workspace_projection.path", Value: target, Message: "filesystem root is not supported"}
	}
	return target, nil
}

func readSemanticState(path string) (semanticState, error) {
	info, err := os.Lstat(path)
	if stderrors.Is(err, fs.ErrNotExist) {
		return semanticState{}, nil
	}
	if err != nil {
		return semanticState{}, errors.WrapIO("stat", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return semanticState{}, &errors.ValidationError{
			Field: "workspace_projection.path", Value: path, Message: "symbolic links are not supported",
		}
	}
	if !info.IsDir() {
		return semanticState{}, &errors.ValidationError{
			Field: "workspace_projection.path", Value: path, Message: "must be a directory",
		}
	}
	builder, err := catalogs.NewFromPath(path)
	if err != nil {
		return semanticState{}, errors.WrapResource("load", "catalog workspace", path, err)
	}
	if err := builder.LoadReport().Err(); err != nil {
		return semanticState{}, errors.WrapResource("validate", "catalog workspace model files", path, err)
	}
	catalog, err := builder.Build()
	if err != nil {
		return semanticState{}, errors.WrapResource("build", "catalog workspace", path, err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		return semanticState{}, errors.WrapResource("encode", "catalog workspace", path, err)
	}
	endpointChecksum, err := readEndpointProjectionChecksum(path)
	if stderrors.Is(err, fs.ErrNotExist) {
		endpointChecksum = ""
	} else if err != nil {
		return semanticState{}, errors.WrapIO("read", endpointProjectionFilename, err)
	}
	return semanticState{
		exists:           true,
		checksum:         catalogs.DescribeCatalogPayload(payload).Checksum,
		endpointChecksum: endpointChecksum,
	}, nil
}

func (p projector) stageCatalog(
	ctx context.Context,
	target string,
	catalog *catalogs.Catalog,
	identity Identity,
	expected *treeSnapshot,
) (result stagedWorkspace, state semanticState, resultErr error) {
	stage, err := prepareWorkspaceStage(ctx, target, p.writer)
	if err != nil {
		return stagedWorkspace{}, semanticState{}, errors.WrapResource("prepare", "workspace staging", target, err)
	}
	defer func() {
		var err error
		if result.path != "" {
			err = stage.detach(ctx)
		} else {
			err = stage.close(ctx)
		}
		if err != nil {
			resultErr = stderrors.Join(resultErr, err)
			if result.path != "" {
				resultErr = stderrors.Join(resultErr, result.cleanup(ctx, treeSnapshot{}))
				result = stagedWorkspace{}
			}
		}
	}()
	staged := stage.renderPath()
	cleanup := func(err error) (stagedWorkspace, semanticState, error) {
		return stagedWorkspace{}, semanticState{}, err
	}
	if err := stage.readSource(ctx, target, expected); err != nil {
		return cleanup(err)
	}
	if err := renderPreparation(ctx, stage.trees["render"], catalog, identity); err != nil {
		return cleanup(err)
	}
	if err := stage.copySource(ctx); err != nil {
		return cleanup(err)
	}
	state, err = readSemanticState(staged)
	if err != nil {
		return cleanup(err)
	}
	if err := stage.validateStableProjection(ctx, staged, state, catalog, identity, p.afterVerificationRender); err != nil {
		return cleanup(err)
	}
	if p.afterStageRender != nil {
		if err := p.afterStageRender(staged); err != nil {
			return cleanup(err)
		}
	}
	candidate, err := stage.finish(ctx, target, p.beforeAccessRestore)
	if err != nil {
		return cleanup(errors.WrapResource("preserve access", "workspace staging", target, err))
	}
	owner := &preparationOwner{target: target, name: stage.name, writer: p.writer, journal: stage.journal.state}
	return stagedWorkspace{path: candidate, tree: stage.published, original: stage.original, owner: owner}, state, nil
}

func (s *workspaceStage) validateStableProjection(
	ctx context.Context,
	staged string,
	state semanticState,
	source *catalogs.Catalog,
	identity Identity,
	afterRender func(string) error,
) error {
	builder, err := catalogs.NewFromPath(staged)
	if err != nil {
		return errors.WrapResource("load", "staged workspace projection", staged, err)
	}
	if err := builder.LoadReport().Err(); err != nil {
		return errors.WrapResource("validate", "staged workspace model files", staged, err)
	}
	catalog, err := builder.Build()
	if err != nil {
		return errors.WrapResource("build", "staged workspace projection", staged, err)
	}
	if err := validateProjectionCoverage(source, catalog); err != nil {
		return err
	}
	name := ".render.verify-" + rand.Text()
	tree, err := s.createTree(ctx, name)
	if err != nil {
		return err
	}
	verification := filepath.Join(s.private.Name(), name)
	if err := renderPreparation(ctx, tree, catalog, identity); err != nil {
		return err
	}
	if afterRender != nil {
		if err := afterRender(verification); err != nil {
			return err
		}
	}
	verified, err := readSemanticState(verification)
	if err != nil {
		return err
	}
	if state.checksum != verified.checksum {
		return &errors.ValidationError{
			Field:   "workspace_projection.workspace_checksum",
			Value:   state.checksum,
			Message: "YAML projection is not semantically stable across repeated save/load cycles",
		}
	}
	if state.endpointChecksum != verified.endpointChecksum {
		return &errors.ValidationError{
			Field:   "workspace_projection.endpoint_checksum",
			Value:   state.endpointChecksum,
			Message: "endpoint projection is not byte-stable across repeated save/load cycles",
		}
	}
	return s.removeTree(ctx, name)
}

func validateProjectionCoverage(source, projected *catalogs.Catalog) error {
	sourceProviders := source.Providers().List()
	if got := len(projected.Providers().List()); got != len(sourceProviders) {
		return projectionCoverageError("providers", len(sourceProviders), got)
	}
	for _, provider := range sourceProviders {
		projectedProvider, err := projected.Provider(provider.ID)
		if err != nil {
			return errors.WrapResource("validate", "projected provider", string(provider.ID), err)
		}
		if got, want := len(projectedProvider.Models), len(provider.Models); got != want {
			return projectionCoverageError("provider."+string(provider.ID)+".models", want, got)
		}
		for modelID := range provider.Models {
			if _, exists := projectedProvider.Models[modelID]; !exists {
				return &errors.ValidationError{
					Field:   "workspace_projection.coverage",
					Value:   "provider." + string(provider.ID) + ".models." + modelID,
					Message: "projected workspace omitted a persisted provider model",
				}
			}
		}
	}

	sourceAuthors := source.Authors().List()
	if got := len(projected.Authors().List()); got != len(sourceAuthors) {
		return projectionCoverageError("authors", len(sourceAuthors), got)
	}
	for _, author := range sourceAuthors {
		if _, err := projected.Author(author.ID); err != nil {
			return errors.WrapResource("validate", "projected author", string(author.ID), err)
		}
		sourceModels, err := source.AuthorModels(author.ID)
		if err != nil {
			return errors.WrapResource("validate", "source author models", string(author.ID), err)
		}
		projectedModels, err := projected.AuthorModels(author.ID)
		if err != nil {
			return errors.WrapResource("validate", "projected author models", string(author.ID), err)
		}
		if len(projectedModels) != len(sourceModels) {
			return projectionCoverageError("author."+string(author.ID)+".models", len(sourceModels), len(projectedModels))
		}
		for index := range sourceModels {
			if sourceModels[index].ID != projectedModels[index].ID {
				return &errors.ValidationError{
					Field:   "workspace_projection.coverage",
					Value:   "author." + string(author.ID) + ".models",
					Message: "projected workspace changed derived author membership",
				}
			}
		}
	}

	sourceProvenance := source.Provenance().Map()
	projectedProvenance := projected.Provenance().Map()
	if len(projectedProvenance) != len(sourceProvenance) {
		return projectionCoverageError("provenance", len(sourceProvenance), len(projectedProvenance))
	}
	for key, entries := range sourceProvenance {
		if got := len(projectedProvenance[key]); got != len(entries) {
			return projectionCoverageError("provenance."+key, len(entries), got)
		}
	}
	return nil
}

func projectionCoverageError(field string, want, got int) error {
	return &errors.ValidationError{
		Field: "workspace_projection.coverage",
		Value: field,
		Message: "projected workspace has " + field + " count " +
			strconv.Itoa(got) + ", expected " + strconv.Itoa(want),
	}
}

func promoteDirectory(staged, target string, targetExists bool) error {
	parent := filepath.Dir(target)
	if targetExists {
		return promoteExistingDirectory(staged, target)
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return errors.WrapIO("open", parent, err)
	}
	defer func() { _ = root.Close() }()
	if err := filepublish.DirectoryNoReplace(root, filepath.Base(staged), filepath.Base(target)); err != nil {
		if os.IsExist(err) {
			return replacementConflict(target, "publication destination already exists")
		}
		return errors.WrapIO("promote", target, err)
	}
	if err := filepublish.SyncDirectory(root); err != nil {
		return errors.WrapIO("sync", parent, err)
	}
	return nil
}

type projectionMarker struct {
	Version           int    `json:"version"`
	GenerationID      string `json:"generation_id"`
	PayloadChecksum   string `json:"payload_checksum"`
	WorkspaceChecksum string `json:"workspace_checksum"`
	EndpointChecksum  string `json:"endpoint_checksum"`
}

func projectionMarkerPath(target string) string {
	return filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".starmap-projection.json")
}

func readProjectionMarker(target string) (projectionMarker, error) {
	path := projectionMarkerPath(target)
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return projectionMarker{}, err
	}
	defer func() { _ = root.Close() }()
	data, err := readWorkspaceRecordBytes(root, filepath.Base(path), replacementJournalMax)
	if err != nil {
		return projectionMarker{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var marker projectionMarker
	if err := decoder.Decode(&marker); err != nil {
		return projectionMarker{}, &errors.ParseError{Format: "json", File: path, Message: err.Error(), Err: err}
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return projectionMarker{}, &errors.ParseError{Format: "json", File: path, Message: "invalid trailing data", Err: err}
	}
	if marker.Version != markerVersion || marker.GenerationID == "" ||
		marker.PayloadChecksum == "" || marker.WorkspaceChecksum == "" ||
		marker.EndpointChecksum == "" {
		return projectionMarker{}, &errors.ValidationError{
			Field: "workspace_projection.marker", Value: path, Message: "is incomplete or unsupported",
		}
	}
	return marker, nil
}

func (hooks workspaceRecordWriter) writeProjectionMarker(ctx context.Context, target string, marker projectionMarker) error {
	data, err := json.Marshal(marker)
	if err != nil {
		return errors.WrapResource("encode", "workspace projection marker", marker.GenerationID, err)
	}
	data = append(data, '\n')
	path := projectionMarkerPath(target)
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	_, err = hooks.publish(ctx, root, filepath.Base(path), data, recordPublication{replace: true, normalizeMode: true})
	return err
}

func syncDirectory(path string) error {
	directory, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }()
	return filepublish.SyncDirectory(directory)
}
