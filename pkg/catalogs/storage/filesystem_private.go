package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func readPrivateStoreFile(path string) ([]byte, error) {
	directory, err := privatefiles.ExistingDirectory(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	root, err := directory.Open()
	if err != nil {
		return nil, err
	}
	info, err := root.Lstat(filepath.Base(path))
	_ = root.Close()
	if err != nil {
		return nil, err
	}
	return directory.ReadFile(filepath.Base(path), info.Size())
}

func (s *Filesystem) prepareCommitLock() error {
	directory, err := privatefiles.ExistingDirectory(s.root)
	if err != nil {
		return err
	}
	root, err := directory.Open()
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	file, err := privatefiles.CreateFile(root, ".commit.lock")
	if err != nil && !os.IsExist(err) {
		return err
	}
	if err == nil {
		if err := file.Close(); err != nil {
			return err
		}
	}
	return validateFilesystemEntry(filepath.Join(s.root, ".commit.lock"), false)
}

func (s *Filesystem) validateCommitLock() error {
	locked, err := s.commitLock.Stat()
	if err != nil {
		return err
	}
	current, err := os.Lstat(filepath.Join(s.root, ".commit.lock"))
	if err != nil {
		return err
	}
	if !current.Mode().IsRegular() || !os.SameFile(locked, current) {
		return &errors.ConflictError{Resource: "catalog filesystem commit lock", Message: "lock file changed during access"}
	}
	return nil
}

func (s *Filesystem) writeGeneration(ctx context.Context, generation catalogs.Generation) error {
	manifest, err := marshalManifest(generation.Manifest)
	if err != nil {
		return err
	}
	directory, err := privatefiles.ExistingDirectory(filepath.Join(s.root, "generations"))
	if err != nil {
		return err
	}
	parent, err := directory.Open()
	if err != nil {
		return err
	}
	defer func() { _ = parent.Close() }()
	name := ".candidate-" + rand.Text()
	if err := privatefiles.CreateChild(parent, name); err != nil {
		return err
	}
	identity, err := parent.Lstat(name)
	if err != nil {
		return err
	}
	stage, err := parent.OpenRoot(name)
	if err != nil {
		return err
	}
	defer func() { _ = stage.Close() }()
	owned := make(map[string]storeStageRecord)
	defer cleanupStoreStage(parent, stage, name, identity, owned)
	for _, record := range []struct {
		name string
		data []byte
	}{
		{manifestFilename, manifest}, {payloadFilename, generation.Payload},
	} {
		file, err := privatefiles.CreateFile(stage, record.name)
		if err != nil {
			return err
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return err
		}
		owned[record.name] = storeStageRecord{info: info, digest: sha256.Sum256(nil)}
		if _, err := file.Write(record.data); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			return err
		}
		written, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return err
		}
		owned[record.name] = storeStageRecord{info: written, digest: sha256.Sum256(record.data)}
		if err := file.Close(); err != nil {
			return err
		}
	}
	if err := filepublish.SyncDirectory(stage); err != nil {
		return err
	}
	if s.beforeGenerationPromotion != nil {
		if err := s.beforeGenerationPromotion(filepath.Join(parent.Name(), name)); err != nil {
			return err
		}
	}
	current, err := parent.Lstat(name)
	if err != nil {
		return err
	}
	if !os.SameFile(identity, current) {
		return &errors.ConflictError{Resource: "catalog candidate", Message: "candidate directory changed before publication"}
	}
	staged, err := directory.ExistingChild(name)
	if err != nil {
		return err
	}
	entries, err := staged.ReadDir()
	if err != nil {
		return err
	}
	if len(entries) != len(owned) {
		return &errors.ConflictError{Resource: "catalog candidate", Message: "candidate entries changed before publication"}
	}
	for _, record := range []struct {
		name string
		data []byte
	}{{manifestFilename, manifest}, {payloadFilename, generation.Payload}} {
		data, err := staged.ReadFile(record.name, int64(len(record.data)))
		if err != nil {
			return err
		}
		if !bytes.Equal(data, record.data) {
			return &errors.ConflictError{Resource: "catalog candidate", Message: "candidate contents changed before publication"}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	checked, err := directory.Open()
	if err != nil {
		return err
	}
	_ = checked.Close()
	if err := validateFilesystemEntry(filepath.Join(parent.Name(), name), true); err != nil {
		return err
	}
	if err := filepublish.DirectoryNoReplace(parent, name, filepath.Base(s.generationDir(generation.Manifest.GenerationID))); err != nil {
		return err
	}
	return filepublish.SyncDirectory(parent)
}

type storeStageRecord struct {
	info   fs.FileInfo
	digest [sha256.Size]byte
}

func cleanupStoreStage(parent, stage *os.Root, name string, identity fs.FileInfo, owned map[string]storeStageRecord) {
	current, err := parent.Lstat(name)
	if err != nil || !os.SameFile(identity, current) {
		return
	}
	for name, original := range owned {
		info, err := stage.Lstat(name)
		if err != nil || !os.SameFile(original.info, info) || info.Size() != original.info.Size() || !info.ModTime().Equal(original.info.ModTime()) {
			continue
		}
		data, err := privatefiles.ReadFile(stage, name, original.info.Size())
		if err == nil && sha256.Sum256(data) == original.digest {
			_ = stage.Remove(name)
		}
	}
	_ = parent.Remove(name)
}
