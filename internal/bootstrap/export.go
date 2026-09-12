package bootstrap

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

const (
	baselineDirectoryMode = 0o700
	baselineFileMode      = 0o600
	baselineManifestName  = "manifest.json"
	baselinePayloadName   = "catalog.json"
)

// ExportResult identifies the installed baseline export and its recovery outcomes.
type ExportResult struct {
	GenerationID string           `json:"generation_id"`
	Directory    string           `json:"directory"`
	Created      bool             `json:"created"`
	Recovery     BaselineRecovery `json:"recovery"`
}

// Export writes the installed generation as an immutable inspectable baseline.
// It never changes a catalog store's accepted head or exports acquired credentials.
func Export(ctx context.Context, directory string) (ExportResult, error) {
	return exportBaseline(ctx, directory, nil)
}

func exportBaseline(ctx context.Context, directory string, checkpoint func(string) error) (result ExportResult, resultErr error) {
	if err := policy.Require("baseline", policy.PublicRead); err != nil {
		return result, err
	}
	if ctx == nil {
		return ExportResult{}, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return ExportResult{}, err
	}
	if !filepath.IsAbs(directory) {
		return ExportResult{}, &errors.ValidationError{Field: "baseline.directory", Message: "must be an absolute directory"}
	}
	generation, err := Generation()
	if err != nil {
		return ExportResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return ExportResult{}, err
	}
	digest := sha256.Sum256([]byte(generation.Manifest.GenerationID))
	name := hex.EncodeToString(digest[:])
	result = ExportResult{GenerationID: generation.Manifest.GenerationID, Directory: filepath.Join(directory, name)}
	if err := privatefiles.CreateDirectory(directory); err != nil {
		return result, errors.WrapIO("create", directory, err)
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return result, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return result, &errors.ValidationError{Field: "baseline.directory", Message: "must be a real directory"}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return result, err
	}
	defer func() { _ = root.Close() }()
	recovery, err := openBaselineRecovery(ctx, root, directory)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = stderrors.Join(resultErr, recovery.close()) }()
	result.Recovery, err = recovery.recover(ctx, checkpoint)
	if err != nil {
		return result, err
	}
	if _, err := root.Lstat(name); err == nil {
		return result, verifyAndSyncBaseline(root, name, generation)
	} else if !stderrors.Is(err, fs.ErrNotExist) {
		return result, err
	}
	stage := baselineStagePrefix + rand.Text()
	if err := privatefiles.CreateChild(root, stage); err != nil {
		return result, err
	}
	staging, err := openBaselineStage(root, stage)
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = stderrors.Join(resultErr, staging.cleanup(context.Background(), nil), staging.close())
	}()
	if err := recovery.newJournal(ctx, staging, name); err != nil {
		return result, err
	}
	check := func(point string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if checkpoint != nil {
			return checkpoint(point)
		}
		return nil
	}
	if err := check("created"); err != nil {
		return result, err
	}
	if err := staging.writeGeneration(ctx, generation, check); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := filepublish.DirectoryNoReplace(root, stage, name); err != nil {
		if _, found := root.Lstat(name); found == nil {
			return result, verifyAndSyncBaseline(root, name, generation)
		}
		return result, errors.WrapIO("publish", result.Directory, err)
	}
	staging.published = true
	result.Created = true
	if err := check("promoted"); err != nil {
		return result, err
	}
	return result, verifyAndSyncBaseline(root, name, generation)
}

func syncBaselineDirectory(root *os.Root) error {
	return filepublish.SyncDirectory(root)
}

func verifyBaseline(root *os.Root, name string, expected catalogs.Generation) error {
	conflict := func() error {
		return &errors.ConflictError{Resource: "embedded baseline export", Message: "existing export differs from the installed generation"}
	}
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return conflict()
	}
	directory, err := root.OpenRoot(name)
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }()
	expectedBytes, err := json.MarshalIndent(expected.Manifest, "", "  ")
	if err != nil {
		return err
	}
	sizes := map[string]int{baselineManifestName: len(expectedBytes) + 1, baselinePayloadName: len(expected.Payload)}
	for file, size := range sizes {
		info, err := directory.Lstat(file)
		if err != nil {
			return conflict()
		}
		if !info.Mode().IsRegular() || info.Size() != int64(size) {
			return conflict()
		}
	}
	manifestBytes, err := readBaselineFile(directory, baselineManifestName, sizes[baselineManifestName])
	if err != nil {
		return err
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(manifestBytes)
	if err != nil {
		return conflict()
	}
	payload, err := readBaselineFile(directory, baselinePayloadName, sizes[baselinePayloadName])
	if err != nil {
		return err
	}
	actual := catalogs.Generation{Manifest: manifest, Payload: payload}
	if err := actual.Validate(); err != nil {
		return conflict()
	}
	expectedManifest, err := json.Marshal(expected.Manifest)
	if err != nil {
		return err
	}
	actualManifest, err := json.Marshal(actual.Manifest)
	if err != nil {
		return err
	}
	if !bytes.Equal(expectedManifest, actualManifest) || !bytes.Equal(expected.Payload, actual.Payload) {
		return conflict()
	}
	return nil
}

func verifyAndSyncBaseline(root *os.Root, name string, generation catalogs.Generation) error {
	if err := verifyBaseline(root, name, generation); err != nil {
		return err
	}
	return syncBaselineDirectory(root)
}

func readBaselineFile(root *os.Root, name string, size int) ([]byte, error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, int64(size)+1))
	if err != nil {
		return nil, err
	}
	if len(data) != size {
		return nil, &errors.ConflictError{Resource: "embedded baseline export", Message: "file size changed during verification"}
	}
	return data, nil
}

func (staging *baselineStage) writeGeneration(ctx context.Context, generation catalogs.Generation, check func(string) error) error {
	manifest, err := json.MarshalIndent(generation.Manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := staging.write(baselineManifestName, append(manifest, '\n')); err != nil {
		return err
	}
	if err := staging.journal.save(ctx, staging, baselineJournalWriting); err != nil {
		return err
	}
	if err := check("manifest-written"); err != nil {
		return err
	}
	if err := staging.write(baselinePayloadName, generation.Payload); err != nil {
		return err
	}
	if err := staging.journal.save(ctx, staging, baselineJournalWriting); err != nil {
		return err
	}
	if err := check("payload-written"); err != nil {
		return err
	}
	if err := staging.validate(); err != nil {
		return err
	}
	if err := syncBaselineDirectory(staging.root); err != nil {
		return err
	}
	return nil
}
