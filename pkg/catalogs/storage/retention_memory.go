package storage

import (
	"context"
	"slices"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// AcquireGeneration copies a generation and protects its stored form until release.
// The release function is idempotent and does not depend on the request context.
func (s *Memory) AcquireGeneration(ctx context.Context, id string) (catalogs.Generation, func() error, error) {
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, nil, err
	}
	generation, found := s.generations[id]
	if !found {
		return catalogs.Generation{}, nil, generationNotFound(id)
	}
	owned := generation.Copy()
	if s.readLeases == nil {
		s.readLeases = make(map[string]int)
	}
	s.readLeases[id]++
	release := sync.OnceValue(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.readLeases[id]--
		if s.readLeases[id] == 0 {
			delete(s.readLeases, id)
		}
		return nil
	})
	return owned, release, nil
}

// Collect applies one bounded retention decision under the publication lock.
// Missing requirements, stale current, and incomplete scans preserve all content.
func (s *Memory) Collect(ctx context.Context, request RetentionRequest) (RetentionReport, error) {
	if err := ctx.Err(); err != nil {
		return RetentionReport{}, err
	}
	limit, err := request.scanLimit()
	if err != nil {
		return RetentionReport{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return RetentionReport{}, err
	}
	if s.currentID != request.ExpectedGenerationID {
		return RetentionReport{}, casConflict(request.ExpectedGenerationID, s.currentID)
	}
	if len(s.generations) > limit {
		return RetentionReport{}, &errors.ConflictError{Resource: "catalog retention scan", Message: "generation count exceeds the scan limit"}
	}
	required := make(map[string]bool, len(request.RequiredGenerationIDs))
	for _, id := range request.RequiredGenerationIDs {
		if _, found := s.generations[id]; !found {
			return RetentionReport{}, generationNotFound(id)
		}
		required[id] = true
	}
	entries := make([]retentionEntry, 0, len(s.generations))
	for id, generation := range s.generations {
		if err := ctx.Err(); err != nil {
			return RetentionReport{}, err
		}
		manifest, err := marshalManifest(generation.Manifest)
		if err != nil {
			return RetentionReport{}, err
		}
		entries = append(entries, retentionEntry{
			id: id, generatedAt: generation.Manifest.GeneratedAt,
			bytes:     int64(len(manifest)) + int64(len(generation.Payload)),
			protected: id == s.currentID || required[id] || s.readLeases[id] > 0,
		})
	}
	report, err := selectRetention(ctx, request, entries)
	if err != nil || request.DryRun {
		return report, err
	}
	if err := ctx.Err(); err != nil {
		return RetentionReport{}, err
	}
	for _, id := range report.Candidates {
		delete(s.generations, id)
	}
	report.Removed = slices.Clone(report.Candidates)
	report.After = report.Projected
	return report, nil
}

var _ RetainingStore = (*Memory)(nil)
