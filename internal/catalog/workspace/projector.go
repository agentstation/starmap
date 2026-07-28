// Package workspace atomically projects committed catalogs into the optional
// human-editable provider YAML workspace.
package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/save"
)

const (
	markerVersion = 1

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
}

// InputExpectation records workspace presence and, once loaded, its semantic
// digest before candidate construction. Projection rejects a different input.
type InputExpectation struct {
	Path     string
	Exists   bool
	Checksum string
}

// ObserveInput records the selected workspace's presence without creating or
// modifying it.
func ObserveInput(path string) (InputExpectation, error) {
	if strings.TrimSpace(path) == "" {
		return InputExpectation{}, nil
	}
	target, err := resolveTarget(path)
	if err != nil {
		return InputExpectation{}, err
	}
	if err := ValidateHumanLayout(target, ""); err != nil {
		return InputExpectation{}, err
	}
	_, err = os.Lstat(target)
	switch {
	case err == nil:
		return InputExpectation{Path: target, Exists: true}, nil
	case stderrors.Is(err, fs.ErrNotExist):
		return InputExpectation{Path: target}, nil
	default:
		return InputExpectation{}, errors.WrapIO("inspect", target, err)
	}
}

// RequiresSeed reports whether an explicit operation selected an absent human
// workspace that must be materialized even when catalog facts are unchanged.
func (i InputExpectation) RequiresSeed() bool {
	return i.Path != "" && !i.Exists
}

// BindInputCatalog records the semantic digest of the human catalog loaded
// from an existing workspace before candidate construction.
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
	beforeInputCheck func() error
	beforePromote    func() error
	beforeMarker     func() error
}

// Project stages, validates, syncs, and atomically publishes one workspace.
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
	if err := os.MkdirAll(filepath.Dir(target), constants.DirPermissions); err != nil {
		return Receipt{}, errors.WrapIO("create", filepath.Dir(target), err)
	}
	release, err := acquireWriterLock(target)
	if err != nil {
		return Receipt{}, err
	}
	defer release()
	return p.projectLocked(ctx, target, catalog, identity, expectation)
}

func (p projector) projectLocked(
	ctx context.Context,
	target string,
	catalog *catalogs.Catalog,
	identity Identity,
	expectation InputExpectation,
) (Receipt, error) {
	input, err := readSemanticState(target)
	if err != nil {
		return Receipt{}, err
	}
	if err := validateInputExpectation(target, input, expectation); err != nil {
		return Receipt{}, err
	}
	staged, stagedState, err := stageCatalog(target, catalog)
	if err != nil {
		return Receipt{}, err
	}
	defer func() { _ = os.RemoveAll(staged) }()
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
	if err := promoteDirectory(staged, target, input.exists); err != nil {
		return Receipt{}, err
	}

	receipt := Receipt{
		GenerationID:      identity.GenerationID,
		WorkspaceChecksum: stagedState.checksum,
	}
	if p.beforeMarker != nil {
		if err := p.beforeMarker(); err != nil {
			return receipt, err
		}
	}
	if err := writeProjectionMarker(target, projectionMarker{
		Version:           markerVersion,
		GenerationID:      identity.GenerationID,
		PayloadChecksum:   identity.PayloadChecksum,
		WorkspaceChecksum: stagedState.checksum,
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
		if !expectation.Exists || expectation.Checksum == "" || expectation.Checksum == input.checksum {
			return nil
		}
		return &errors.ConflictError{
			Resource: "catalog workspace projection input",
			Expected: expectation.Checksum,
			Actual:   input.checksum,
			Message:  "workspace semantics changed after candidate construction",
		}
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

func (p projector) repair(ctx context.Context, path string, current *catalogs.Catalog, identity Identity) (RepairResult, error) {
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
	if err := os.MkdirAll(filepath.Dir(target), constants.DirPermissions); err != nil {
		return RepairResult{}, errors.WrapIO("create", filepath.Dir(target), err)
	}
	release, err := acquireWriterLock(target)
	if err != nil {
		return RepairResult{}, err
	}
	defer release()
	state, err := readSemanticState(target)
	if err != nil {
		return RepairResult{}, err
	}
	marker, markerErr := readProjectionMarker(target)
	if markerErr == nil {
		if state.exists &&
			marker.GenerationID == identity.GenerationID &&
			marker.PayloadChecksum == identity.PayloadChecksum &&
			marker.WorkspaceChecksum == state.checksum {
			return RepairResult{Status: RepairStatusCurrent}, nil
		}
	} else if !stderrors.Is(markerErr, fs.ErrNotExist) {
		return RepairResult{}, markerErr
	}

	desiredPath, desired, err := stageCatalog(target, current)
	if err != nil {
		return RepairResult{}, err
	}
	defer func() { _ = os.RemoveAll(desiredPath) }()
	if state.exists && state.checksum == desired.checksum {
		if err := writeProjectionMarker(target, projectionMarker{
			Version: markerVersion, GenerationID: identity.GenerationID,
			PayloadChecksum: identity.PayloadChecksum, WorkspaceChecksum: state.checksum,
		}); err != nil {
			return RepairResult{}, err
		}
		return RepairResult{Status: RepairStatusRepaired}, nil
	}
	if markerErr == nil && state.exists && state.checksum != marker.WorkspaceChecksum {
		return RepairResult{Status: RepairStatusSkippedDirty, IssueCode: IssueDirty}, nil
	}
	if stderrors.Is(markerErr, fs.ErrNotExist) && state.exists {
		return RepairResult{Status: RepairStatusSkippedDirty, IssueCode: IssueDirty}, nil
	}

	if _, err := p.projectLocked(ctx, target, current, identity, InputExpectation{}); err != nil {
		return RepairResult{}, err
	}
	return RepairResult{Status: RepairStatusRepaired}, nil
}

type semanticState struct {
	exists   bool
	checksum string
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
	return s.exists == other.exists && s.checksum == other.checksum
}

func (s semanticState) describe() string {
	if !s.exists {
		return "absent"
	}
	return s.checksum
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
	return semanticState{exists: true, checksum: catalogs.DescribeCatalogPayload(payload).Checksum}, nil
}

func stageCatalog(target string, catalog *catalogs.Catalog) (string, semanticState, error) {
	parent := filepath.Dir(target)
	staged, err := os.MkdirTemp(parent, "."+filepath.Base(target)+".candidate-")
	if err != nil {
		return "", semanticState{}, errors.WrapIO("create", parent, err)
	}
	cleanup := func(err error) (string, semanticState, error) {
		_ = os.RemoveAll(staged)
		return "", semanticState{}, err
	}
	if err := os.Chmod(staged, constants.DirPermissions); err != nil {
		return cleanup(errors.WrapIO("chmod", staged, err))
	}
	if info, statErr := os.Lstat(target); statErr == nil && info.IsDir() {
		if err := os.CopyFS(staged, os.DirFS(target)); err != nil {
			return cleanup(errors.WrapIO("copy", target, err))
		}
	} else if statErr != nil && !stderrors.Is(statErr, fs.ErrNotExist) {
		return cleanup(errors.WrapIO("stat", target, statErr))
	}
	builder, err := catalogs.NewBuilderFrom(catalog)
	if err != nil {
		return cleanup(errors.WrapResource("build", "workspace projection", target, err))
	}
	if err := builder.Save(save.WithPath(staged)); err != nil {
		return cleanup(errors.WrapIO("stage", staged, err))
	}
	state, err := readSemanticState(staged)
	if err != nil {
		return cleanup(err)
	}
	if err := validateStableProjection(staged, state, catalog); err != nil {
		return cleanup(err)
	}
	if err := syncTree(staged); err != nil {
		return cleanup(errors.WrapIO("sync", staged, err))
	}
	return staged, state, nil
}

func validateStableProjection(staged string, state semanticState, source *catalogs.Catalog) error {
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
	verification, err := os.MkdirTemp(filepath.Dir(staged), "."+filepath.Base(staged)+".verify-")
	if err != nil {
		return errors.WrapIO("create", filepath.Dir(staged), err)
	}
	defer func() { _ = os.RemoveAll(verification) }()
	if err := os.Chmod(verification, constants.DirPermissions); err != nil {
		return errors.WrapIO("chmod", verification, err)
	}
	verificationBuilder, err := catalogs.NewBuilderFrom(catalog)
	if err != nil {
		return errors.WrapResource("build", "workspace verification", staged, err)
	}
	if err := verificationBuilder.Save(save.WithPath(verification)); err != nil {
		return errors.WrapIO("verify", verification, err)
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
	return nil
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
		projectedAuthor, err := projected.Author(author.ID)
		if err != nil {
			return errors.WrapResource("validate", "projected author", string(author.ID), err)
		}
		wantModels := 0
		for modelID := range author.Models {
			if strings.Contains(modelID, "/") {
				continue
			}
			wantModels++
			if _, exists := projectedAuthor.Models[modelID]; !exists {
				return &errors.ValidationError{
					Field:   "workspace_projection.coverage",
					Value:   "author." + string(author.ID) + ".models." + modelID,
					Message: "projected workspace omitted a persisted author model",
				}
			}
		}
		gotModels := 0
		for modelID := range projectedAuthor.Models {
			if !strings.Contains(modelID, "/") {
				gotModels++
			}
		}
		if gotModels != wantModels {
			return projectionCoverageError("author."+string(author.ID)+".models", wantModels, gotModels)
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
		if err := swapDirectories(staged, target); err != nil {
			return errors.WrapIO("promote", target, err)
		}
	} else if err := os.Rename(staged, target); err != nil {
		return errors.WrapIO("promote", target, err)
	}
	if err := syncDirectory(parent); err != nil {
		return errors.WrapIO("sync", parent, err)
	}
	return nil
}

func syncTree(root string) error {
	var directories []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			directories = append(directories, path)
		case info.Mode().IsRegular():
			file, err := os.Open(path) //nolint:gosec // path is confined to the private staging directory.
			if err != nil {
				return err
			}
			if err := file.Sync(); err != nil {
				_ = file.Close()
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
		default:
			return &errors.ValidationError{
				Field: "workspace_projection.entry", Value: path,
				Message: "only regular files and directories are supported",
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for index := len(directories) - 1; index >= 0; index-- {
		if err := syncDirectory(directories[index]); err != nil {
			return err
		}
	}
	return nil
}

type projectionMarker struct {
	Version           int    `json:"version"`
	GenerationID      string `json:"generation_id"`
	PayloadChecksum   string `json:"payload_checksum"`
	WorkspaceChecksum string `json:"workspace_checksum"`
}

func projectionMarkerPath(target string) string {
	return filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".starmap-projection.json")
}

func readProjectionMarker(target string) (projectionMarker, error) {
	path := projectionMarkerPath(target)
	data, err := os.ReadFile(path) //nolint:gosec // path is derived from the configured workspace.
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
		marker.PayloadChecksum == "" || marker.WorkspaceChecksum == "" {
		return projectionMarker{}, &errors.ValidationError{
			Field: "workspace_projection.marker", Value: path, Message: "is incomplete or unsupported",
		}
	}
	return marker, nil
}

func writeProjectionMarker(target string, marker projectionMarker) error {
	data, err := json.Marshal(marker)
	if err != nil {
		return errors.WrapResource("encode", "workspace projection marker", marker.GenerationID, err)
	}
	data = append(data, '\n')
	path := projectionMarkerPath(target)
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".")
	if err != nil {
		return errors.WrapIO("create", path, err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(constants.FilePermissions); err != nil {
		_ = temp.Close()
		return errors.WrapIO("chmod", tempPath, err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return errors.WrapIO("write", tempPath, err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return errors.WrapIO("sync", tempPath, err)
	}
	if err := temp.Close(); err != nil {
		return errors.WrapIO("close", tempPath, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return errors.WrapIO("promote", path, err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return errors.WrapIO("sync", filepath.Dir(path), err)
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path) //nolint:gosec // caller passes a configured or staging-owned directory.
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }()
	return directory.Sync()
}
