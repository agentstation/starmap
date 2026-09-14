package storage

import (
	"context"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

type retentionEntry struct {
	id          string
	generatedAt time.Time
	bytes       int64
	protected   bool
}

// selectRetention removes oldest generated content first, with an ID tie break.
// Required content survives even when it exceeds the requested capacity.
func selectRetention(ctx context.Context, request RetentionRequest, entries []retentionEntry) (RetentionReport, error) {
	var report RetentionReport
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return RetentionReport{}, err
		}
		if entry.bytes < 0 || entry.bytes > math.MaxInt64-report.Before.Bytes {
			return RetentionReport{}, &errors.ValidationError{Field: "catalog_retention.bytes", Message: "generation sizes exceed the accounting range"}
		}
		report.Before.Generations++
		report.Before.Bytes += entry.bytes
		if entry.protected {
			report.Protected.Generations++
			report.Protected.Bytes += entry.bytes
		}
	}
	report.After, report.Projected = report.Before, report.Before
	slices.SortFunc(entries, func(left, right retentionEntry) int {
		if order := left.generatedAt.Compare(right.generatedAt); order != 0 {
			return order
		}
		return strings.Compare(left.id, right.id)
	})
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return RetentionReport{}, err
		}
		if report.Projected.Generations <= request.MaxGenerations && report.Projected.Bytes <= request.MaxBytes {
			break
		}
		if entry.protected {
			continue
		}
		report.Candidates = append(report.Candidates, entry.id)
		report.Projected.Generations--
		report.Projected.Bytes -= entry.bytes
	}
	report.OverLimit = report.Projected.Generations > request.MaxGenerations || report.Projected.Bytes > request.MaxBytes
	return report, nil
}
