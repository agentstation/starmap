package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	instanceSeedFileName = "instance-seed"
	instanceSeedBytes    = 16
)

// prepareInstanceSeed runs under directory ownership before catalog initialization.
func prepareInstanceSeed(ctx context.Context, directory string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if directory == "" {
		return newInstanceSeed()
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()
	if _, err := root.Lstat(filepath.Join(layerDirectoryName, instanceSeedFileName)); err == nil {
		return "", &errors.ConflictError{Resource: "runtime instance seed", Message: "legacy seed requires explicit verified migration"}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if _, err := root.Lstat(instanceSeedFileName); err == nil {
		return readInstanceSeed(root)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if _, err := root.Lstat(layerDirectoryName); err == nil {
		return "", &errors.ConflictError{Resource: "runtime instance seed", Message: "existing runtime state has no seed and requires explicit recovery"}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	seed, err := newInstanceSeed()
	if err != nil {
		return "", err
	}
	if err := writeOwnerFile(ctx, root, instanceSeedFileName, []byte(seed)); err != nil && !os.IsExist(err) {
		return "", err
	}
	return readInstanceSeed(root)
}

func newInstanceSeed() (string, error) {
	var raw [instanceSeedBytes]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.WrapResource("generate", "runtime instance seed", "", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func readInstanceSeed(root *os.Root) (string, error) {
	invalid := func() error {
		return &errors.ConflictError{Resource: "runtime instance seed", Message: "seed is not a valid private identity record"}
	}
	info, err := root.Lstat(instanceSeedFileName)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() != int64(hex.EncodedLen(instanceSeedBytes)) {
		return "", invalid()
	}
	file, err := root.Open(instanceSeedFileName)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	raw, err := io.ReadAll(io.LimitReader(file, int64(hex.EncodedLen(instanceSeedBytes)+1)))
	if err != nil {
		return "", err
	}
	decoded, err := hex.DecodeString(string(raw))
	if err != nil || len(decoded) != instanceSeedBytes || hex.EncodeToString(decoded) != string(raw) {
		return "", invalid()
	}
	if err := filepublish.SyncDirectory(root); err != nil {
		return "", err
	}
	return string(raw), nil
}
