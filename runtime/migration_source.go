package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func verifyMigrationSourceIdentity(root *os.Root, identity string) (string, error) {
	encoded, err := readMigrationFile(root, ownerRecordName, ownerRecordMaxBytes)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var record ownerRecord
	if err := json.Unmarshal(encoded, &record); err != nil {
		return "", invalidMigrationIntent("source_owner")
	}
	if record.SchemaVersion != ownerSchemaVersion {
		return "", invalidMigrationIntent("source_owner")
	}
	if err := record.Validate(); err != nil {
		return "", err
	}
	canonical, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", err
	}
	if !bytes.Equal(encoded, append(canonical, '\n')) {
		return "", invalidMigrationIntent("source_owner")
	}
	if record.SchedulerIdentitySHA256 != "" {
		digest := sha256.Sum256([]byte(identity))
		if record.SchedulerIdentitySHA256 != hex.EncodeToString(digest[:]) {
			return "", invalidMigrationIntent("source_identity")
		}
	} else {
		seed, err := readInstanceSeed(root)
		if err != nil {
			return "", err
		}
		if identity != persistentInstanceIdentity(seed, record.DirectoryOwner) {
			return "", invalidMigrationIntent("source_identity")
		}
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func inspectMigrationFiles(ctx context.Context, root *os.Root) ([]directoryMigrationFile, error) {
	var files []directoryMigrationFile
	err := walkMigrationTree(ctx, root, migrationSourceMaxEntries, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		if path == directoryLockName || path == ownerRecordName || path == migrationReceiptName || path == migrationRetiredName || path == migrationCompletionName {
			if !entry.Type().IsRegular() {
				return invalidMigrationIntent("source_metadata")
			}
			return nil
		}
		if !migrationRelativeFile(path) {
			return invalidMigrationIntent("files.source")
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() || len(files) >= migrationManifestMaxFiles {
			return invalidMigrationIntent("files")
		}
		file, err := inspectMigrationFile(ctx, root, path)
		if err != nil {
			return err
		}
		if path == layerDirectoryName+"/"+instanceSeedFileName {
			file.Target = instanceSeedFileName
		}
		if file.Target == instanceSeedFileName {
			if err := verifyMigrationSeed(root, path); err != nil {
				return err
			}
		}
		files = append(files, file)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, validateMigrationFiles(files)
}

func verifyMigrationSeed(root *os.Root, path string) error {
	encoded, err := readMigrationFile(root, filepath.FromSlash(path), int64(hex.EncodedLen(instanceSeedBytes)))
	if err != nil {
		return err
	}
	decoded, err := hex.DecodeString(string(encoded))
	if err != nil || len(decoded) != instanceSeedBytes || hex.EncodeToString(decoded) != string(encoded) {
		return invalidMigrationIntent("seed")
	}
	return nil
}

func inspectMigrationFile(ctx context.Context, root *os.Root, path string) (directoryMigrationFile, error) {
	var result directoryMigrationFile
	name := filepath.FromSlash(path)
	info, err := root.Lstat(name)
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 {
		return result, invalidMigrationIntent("files.source")
	}
	file, err := root.Open(name)
	if err != nil {
		return result, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return result, err
	}
	if !os.SameFile(info, opened) {
		return result, invalidMigrationIntent("changed_source")
	}
	digest := sha256.New()
	n, err := io.Copy(digest, migrationContextReader{ctx: ctx, reader: io.LimitReader(file, info.Size())})
	if err != nil {
		return result, err
	}
	var extra [1]byte
	if count, err := file.Read(extra[:]); count != 0 || !stderrors.Is(err, io.EOF) {
		return result, invalidMigrationIntent("changed_source")
	}
	current, err := file.Stat()
	if err != nil {
		return result, err
	}
	if n != info.Size() || current.Size() != info.Size() || current.ModTime() != info.ModTime() {
		return result, invalidMigrationIntent("changed_source")
	}
	return directoryMigrationFile{Source: path, Target: path, Size: n, SHA256: hex.EncodeToString(digest.Sum(nil))}, nil
}

type migrationContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r migrationContextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}
