package jobs_test

import (
	"testing"
	"time"

	"github.com/agentstation/starport/internal/jobs"
	"github.com/agentstation/starport/internal/repotest"
	"github.com/agentstation/starport/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestBatchCancellationAcrossWorkersPreventsNextLine(t *testing.T) {
	repotest.Run(t, func(t *testing.T, store storage.KVStore) {
		records, err := jobs.OpenBatchRepository(store)
		require.NoError(t, err)
		worker, err := jobs.NewBatchService(records, jobs.WithBatchConcurrency(1))
		require.NoError(t, err)
		other, err := jobs.NewBatchService(records)
		require.NoError(t, err)
		runner := newBlockingRunner()
		batch, err := worker.Submit(t.Context(), jobs.BatchSubmission{
			Account: "account", Endpoint: "/v1/chat/completions", InputFileID: "input",
			IO: newMemoryBatchIO("{}\n{}\n{}\n"), Runner: runner,
		})
		require.NoError(t, err)
		select {
		case <-runner.started:
		case <-time.After(5 * time.Second):
			close(runner.release)
			t.Fatal("first line did not start")
		}
		_, cancelErr := other.Cancel(t.Context(), batch.Account, batch.ID)
		close(runner.release)
		require.NoError(t, cancelErr)
		require.Eventually(t, func() bool {
			result, err := records.Get(t.Context(), batch.Account, batch.ID)
			return err == nil && result.RunFinished
		}, 5*time.Second, 5*time.Millisecond)
		runner.mu.Lock()
		lines := append([]int(nil), runner.lines...)
		runner.mu.Unlock()
		require.Equal(t, []int{1}, lines, "cancellation acknowledged by another worker must prevent untouched lines")
	})
}
