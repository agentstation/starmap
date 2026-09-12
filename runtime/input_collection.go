package runtime

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// InputCollectionRequest bounds one explicit scan of retained input files.
// MaxEntries includes unknown directory entries. MaxBytes bounds raw snapshot bytes.
// DryRun validates references and reports candidates without deleting files.
type InputCollectionRequest struct {
	MaxEntries int
	MaxBytes   int64
	DryRun     bool
}

// InputCollectionReport separates protected references from unrecognized files and removable inputs.
// Scanned includes one possible entry beyond MaxEntries to detect overflow.
// SnapshotBytes counts captured raw records, excluding decoder and filesystem verification work.
// Removed includes visible deletions when later directory synchronization fails.
type InputCollectionReport struct {
	Scanned       int
	SnapshotBytes int64
	Protected     []string
	Preserved     []string
	Candidates    []string
	Removed       []string
}

// CollectRetainedInputs removes validated immutable inputs outside retained history and pending publications.
// It cleans unused local inputs even when acquisition is offline or a generation pin prevents updates.
// Limits must be positive. MaxEntries cannot exceed storage.MaxRetentionScanEntries.
// A failed preflight removes nothing. Later failures return the partial removal report.
func (r *Runtime) CollectRetainedInputs(ctx context.Context, request InputCollectionRequest) (report InputCollectionReport, resultErr error) {
	if r == nil || r.client == nil || r.ctx == nil {
		return report, &errors.ValidationError{Field: "runtime", Message: "requires an open runtime"}
	}
	if ctx == nil {
		return report, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if request.MaxEntries < 1 || request.MaxEntries > storage.MaxRetentionScanEntries || request.MaxBytes < 1 {
		return report, &errors.ValidationError{Field: "input_collection", Message: "requires positive byte and bounded entry limits"}
	}
	id, err := r.client.NextID()
	if err != nil {
		return report, err
	}
	// Manual runs serialize distinct requests and keep directory ownership through shutdown.
	run, _, err := r.runs.start(ctx, r.ctx, runKindManual, id, 0)
	if err != nil {
		return report, err
	}
	defer func() { r.runs.finish(run, RefreshReport{}, resultErr) }()
	stop := context.AfterFunc(ctx, run.cancel)
	defer stop()
	return r.collectRetainedInputs(run.ctx, request)
}

func (r *Runtime) collectRetainedInputs(ctx context.Context, request InputCollectionRequest) (InputCollectionReport, error) {
	r.publicationMu.Lock()
	defer r.publicationMu.Unlock()
	r.providerRetentionMu.Lock()
	defer r.providerRetentionMu.Unlock()
	if err := ctx.Err(); err != nil {
		return InputCollectionReport{}, err
	}
	if !r.store.durable() {
		return InputCollectionReport{}, nil
	}
	snapshot := inputCollectionSnapshot{request: request, records: make(map[string][]byte)}
	if err := snapshot.capture(ctx, r.store); err != nil {
		return snapshot.report, err
	}
	r.mu.RLock()
	active := r.layers.manual
	r.mu.RUnlock()
	if err := snapshot.selectInputs(ctx, r.store, active); err != nil {
		return snapshot.report, err
	}
	if err := snapshot.checkRoots(r.store); err != nil {
		return snapshot.report, err
	}
	if request.DryRun {
		return snapshot.report, ctx.Err()
	}
	for _, name := range snapshot.report.Candidates {
		if err := ctx.Err(); err != nil {
			return snapshot.report, err
		}
		if err := snapshot.checkRoots(r.store); err != nil {
			return snapshot.report, err
		}
		removed, err := snapshot.directory.CompareAndRemoveFileContext(ctx, name, snapshot.records[name])
		if removed {
			snapshot.report.Removed = append(snapshot.report.Removed, name)
		}
		if err != nil {
			return snapshot.report, err
		}
	}
	return snapshot.report, nil
}
