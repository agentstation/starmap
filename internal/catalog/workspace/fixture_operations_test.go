package workspace

import (
	"context"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"os"
	"path/filepath"
)

func writeEndpointProjection(path string, catalog *catalogs.Catalog, identity Identity) (string, error) {
	data, err := EncodeEndpointProjection(catalog, identity)
	if err != nil {
		return "", err
	}
	target := filepath.Join(path, endpointProjectionFilename)
	if err := os.WriteFile(target, data, fileMode); err != nil {
		return "", errors.WrapIO("write", target, err)
	}
	return endpointProjectionChecksum(data), nil
}

func acquireLegacyStoreLock(ctx context.Context, legacy string) (func(), error) {
	lease, err := acquireLegacyStoreLease(ctx, legacy)
	if err != nil {
		return nil, err
	}
	return lease.close, nil
}

func recoverPreparations(ctx context.Context, target string, writer *workspaceWriter) error {
	return recoverPreparationsExcept(ctx, target, writer, nil)
}

func acquireWriterLock(target string) (func(), error) {
	writer, err := acquireWorkspaceWriter(target)
	if err != nil {
		return nil, err
	}
	return writer.close, nil
}
