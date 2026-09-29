package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/agentstation/starmap/internal/privatefiles"
	githubsource "github.com/agentstation/starmap/internal/sources/github"
)

// inspectRuntimeInventory rejects unknown records before any owner reads retained data.
func inspectRuntimeInventory(ctx context.Context, directory *privatefiles.Directory) (resultErr error) {
	root, err := directory.Open()
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	directories := map[string]*privatefiles.Directory{".": directory}
	return walkMigrationTree(ctx, root, migrationSourceMaxEntries, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		parent, found := directories[path.Dir(name)]
		if !found {
			return invalidInputPublication("runtime inventory has an unknown parent")
		}
		if entry.IsDir() {
			if !knownRuntimeRecoveryDirectory(name) {
				return invalidInputPublication("runtime directory requires a separate owner recovery procedure")
			}
			child, err := parent.ExistingChild(entry.Name())
			if err != nil {
				return err
			}
			directories[name] = child
			if isRecordPublicationDirectory(name) {
				return parent.CheckNoPendingPublications(ctx)
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return invalidInputPublication("runtime inventory contains a non-regular file")
		}
		limit, immutable, err := runtimeRecoveryFileLimit(name)
		if err != nil {
			return err
		}
		output := io.Discard
		digest := sha256.New()
		if immutable {
			output = digest
		}
		if _, err := parent.CopyFile(ctx, entry.Name(), output, limit); err != nil {
			return err
		}
		if immutable && entry.Name() != hex.EncodeToString(digest.Sum(nil))+".json" {
			return invalidInputPublication("immutable retained input differs from its content identity")
		}
		return nil
	})
}

func knownRuntimeRecoveryDirectory(name string) bool {
	switch name {
	case layerDirectoryName, layerDirectoryName + "/" + providerLayerDirectoryName,
		layerDirectoryName + "/" + providerLayerDirectoryName + "/" + bindingLayerDirectoryName,
		layerDirectoryName + "/" + inputPublicationDirectory, "github-catalog-source":
		return true
	default:
		return isRecordPublicationDirectory(name)
	}
}

func runtimeRecoveryFileLimit(name string) (int64, bool, error) {
	parent, base := path.Dir(name), path.Base(name)
	if isRecordPublicationDirectory(parent) && base == directoryLockName {
		return 0, false, nil
	}
	switch parent {
	case ".":
		switch base {
		case ownerRecordName:
			return ownerRecordMaxBytes, false, nil
		case instanceSeedFileName:
			return int64(hex.EncodedLen(instanceSeedBytes)), false, nil
		case directoryLockName:
			return 0, false, nil
		}
	case layerDirectoryName:
		switch base {
		case sourceLayerFileName, manualHistoryName, removalPolicyName, inputPublicationName:
			return maxLayerBytes, false, nil
		case permissionCheckpointFile:
			return maxPermissionCheckpointBytes, false, nil
		case generationPinRecordFile:
			return maxGenerationPinRecordBytes, false, nil
		}
	case layerDirectoryName + "/" + providerLayerDirectoryName, layerDirectoryName + "/" + providerLayerDirectoryName + "/" + bindingLayerDirectoryName:
		if strings.HasSuffix(base, ".json") {
			return maxLayerBytes, false, nil
		}
	case layerDirectoryName + "/" + inputPublicationDirectory:
		if validInputReference(base) {
			return maxLayerBytes, true, nil
		}
	case "github-catalog-source":
		if validInputReference(base) {
			return githubsource.MaxStateRecordBytes, false, nil
		}
	}
	return 0, false, invalidInputPublication("runtime file requires a separate owner recovery procedure")
}
