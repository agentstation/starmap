package files_test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starport/internal/blob"
	"github.com/agentstation/starport/internal/files"
	"github.com/agentstation/starport/internal/limits"
	"github.com/agentstation/starport/internal/repotest"
	"github.com/agentstation/starport/internal/storage"
	"github.com/stretchr/testify/require"
)

type synchronizedDeletes struct {
	blob.Store
	calls atomic.Int64
	ready chan struct{}
}

func (b *synchronizedDeletes) Delete(ctx context.Context, key string) error {
	if b.calls.Add(1) == 2 {
		close(b.ready)
	}
	select {
	case <-b.ready:
	case <-ctx.Done():
		return ctx.Err()
	}
	return b.Store.Delete(ctx, key)
}
func TestConcurrentFileRetirementPreservesOtherStoredBytes(t *testing.T) {
	repotest.Run(t, func(t *testing.T, store storage.KVStore) {
		records, err := files.OpenRepository(store)
		require.NoError(t, err)
		blobs, err := blob.NewFilesystem(t.TempDir())
		require.NoError(t, err)
		barrier := &synchronizedDeletes{Store: blobs, ready: make(chan struct{})}
		meter, err := limits.NewStorageMeter(store)
		require.NoError(t, err)
		service, err := files.NewService(records, barrier, files.WithMeter(meter))
		require.NoError(t, err)
		create := func(name string) files.File {
			file, err := service.Upload(t.Context(), files.UploadRequest{Account: "account", Filename: name, Purpose: files.PurposeBatchOutput, Size: 8, StoredBytesBound: 16}, strings.NewReader("12345678"))
			require.NoError(t, err)
			return file
		}
		removed, retained := create("removed"), create("retained")
		deleting := removed
		deleting.State = files.FileStateDeleting
		require.NoError(t, records.Replace(t.Context(), deleting))
		var workers sync.WaitGroup
		errors := make(chan error, 2)
		for range 2 {
			workers.Go(func() { errors <- service.Delete(t.Context(), "account", removed.ID) })
		}
		workers.Wait()
		close(errors)
		for err := range errors {
			require.NoError(t, err)
		}
		_, err = service.Get(t.Context(), "account", retained.ID)
		require.NoError(t, err)
		total, err := meter.Total(t.Context(), "account")
		require.NoError(t, err)
		require.Equal(t, int64(8), total, "retiring one file twice must preserve the retained file's byte charge")
	})
}
