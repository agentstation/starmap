package storage

import (
	"context"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// Collect retires unprotected generations before deleting their immutable objects.
// Its registry compare-and-swap fences expired uploads and concurrent publication.
// Later passes recover delayed orphan writes through their self-contained ownership headers.
func (s *CoordinatedObject) Collect(ctx context.Context, request RetentionRequest) (RetentionReport, error) {
	limit, err := request.scanLimit()
	if err != nil {
		return RetentionReport{}, err
	}
	state, version, err := s.load(ctx)
	if err != nil {
		return RetentionReport{}, err
	}
	if state.Current != request.ExpectedGenerationID {
		return RetentionReport{}, casConflict(request.ExpectedGenerationID, state.Current)
	}
	if len(state.Generations)+len(state.Readers) > limit {
		return RetentionReport{}, coordinatedInvalid("scan", "registered generations and readers exceed the scan limit")
	}
	entries, err := coordinatedRetentionEntries(state, request)
	if err != nil {
		return RetentionReport{}, err
	}
	report, err := selectRetention(ctx, request, entries)
	if err != nil {
		return report, err
	}
	inputLimit := request.InputMaxBytes
	if inputLimit == 0 {
		inputLimit = DefaultRetentionInputMaxBytes
	}
	owned, err := s.inventory(ctx, state, limit, inputLimit)
	if err != nil {
		return report, err
	}
	if request.DryRun {
		_, current, err := s.load(ctx)
		if err == nil && current != version {
			err = coordinatedBusy()
		}
		return report, err
	}
	retired := make(map[string]string, len(report.Candidates))
	for _, id := range report.Candidates {
		retired[state.Generations[id].UploadID] = id
		delete(state.Generations, id)
	}
	now := s.config.Now().UTC()
	for id, entry := range state.Generations {
		if !entry.Committed && !entry.ExpiresAt.After(now) {
			delete(state.Generations, id)
		}
	}
	live := make(map[string]bool, len(state.Generations))
	for _, entry := range state.Generations {
		live[entry.UploadID] = true
	}
	if err := s.saveRegistry(ctx, state, version); err != nil {
		return report, err
	}
	keys := make([]string, 0, len(owned))
	for uploadID := range owned {
		if !live[uploadID] {
			keys = append(keys, uploadID)
		}
	}
	slices.Sort(keys)
	for _, uploadID := range keys {
		entry := owned[uploadID]
		if err := s.objects.Delete(ctx, entry.Key, entry.Version); err != nil && !errors.IsNotFound(err) {
			return report, err
		}
		if id := retired[uploadID]; id != "" {
			report.Removed = append(report.Removed, id)
			report.After.Generations--
			for _, candidate := range entries {
				if candidate.id == id {
					report.After.Bytes -= candidate.bytes
					break
				}
			}
		}
	}
	return report, nil
}

func coordinatedRetentionEntries(state coordinatedRegistry, request RetentionRequest) ([]retentionEntry, error) {
	protected := make(map[string]bool, len(request.RequiredGenerationIDs)+1)
	protected[state.Current] = true
	for _, id := range request.RequiredGenerationIDs {
		if !state.Generations[id].Committed {
			return nil, generationNotFound(id)
		}
		protected[id] = true
	}
	readers := make(map[string]bool, len(state.Readers))
	for _, reader := range state.Readers {
		readers[reader.UploadID] = true
	}
	entries := make([]retentionEntry, 0, len(state.Generations))
	for id, entry := range state.Generations {
		if entry.Committed {
			entries = append(entries, retentionEntry{id: id, generatedAt: entry.GeneratedAt, bytes: entry.Bytes, protected: protected[id] || readers[entry.UploadID]})
		}
	}
	return entries, nil
}

func (s *CoordinatedObject) inventory(ctx context.Context, state coordinatedRegistry, limit int, inputLimit int64) (map[string]ObjectEntry, error) {
	prefix := s.config.Prefix + "/uploads/"
	owned := make(map[string]ObjectEntry)
	registered := make(map[string]string, len(state.Generations))
	for id, entry := range state.Generations {
		registered[entry.UploadID] = id
	}
	seen, cursors := make(map[string]bool), make(map[string]bool)
	cursor := ""
	var inputBytes int64
	for range limit + 1 {
		request := ObjectListRequest{Prefix: prefix, Cursor: cursor, Limit: min(MaxObjectListEntries, limit-len(seen)+1)}
		page, err := s.objects.List(ctx, request)
		if err != nil {
			return nil, err
		}
		if len(page.Objects) > request.Limit {
			return nil, coordinatedInvalid("inventory", "object service exceeded the requested page size")
		}
		for _, entry := range page.Objects {
			if seen[entry.Key] || !strings.HasPrefix(entry.Key, prefix) || entry.Version == "" || entry.Size < 0 {
				return nil, coordinatedInvalid("inventory", "object service returned an invalid or repeated entry")
			}
			seen[entry.Key] = true
			if len(seen) > limit {
				return nil, coordinatedInvalid("scan", "object inventory exceeds the scan limit")
			}
			uploadID := strings.TrimPrefix(entry.Key, prefix)
			if !coordinatedToken(uploadID) {
				continue
			}
			descriptor, exists := state.Generations[registered[uploadID]]
			if exists && !descriptor.Committed && descriptor.ExpiresAt.After(s.config.Now().UTC()) {
				continue
			}
			if !descriptor.Committed && entry.Size <= MaxCoordinatedObjectBytes {
				if entry.Size > inputLimit-inputBytes {
					return nil, coordinatedInvalid("input_max_bytes", "object recovery exceeds the configured input byte bound")
				}
				inputBytes += entry.Size
			}
			valid, err := s.inspectUpload(ctx, state, entry, uploadID, registered[uploadID])
			if err != nil {
				return nil, err
			}
			if valid {
				owned[uploadID] = entry
			}
		}
		if page.Next == "" {
			for id, entry := range state.Generations {
				if entry.Committed {
					if _, found := owned[entry.UploadID]; !found {
						return nil, generationNotFound(id)
					}
				}
			}
			return owned, nil
		}
		if cursors[page.Next] || len(seen) >= limit {
			return nil, coordinatedInvalid("scan", "object inventory is incomplete or repeats a cursor")
		}
		cursors[page.Next] = true
		cursor = page.Next
	}
	return nil, coordinatedInvalid("scan", "object inventory exceeds its page bound")
}

func (s *CoordinatedObject) inspectUpload(ctx context.Context, state coordinatedRegistry, entry ObjectEntry, uploadID, registeredID string) (bool, error) {
	if descriptor := state.Generations[registeredID]; descriptor.Committed {
		if entry.Version != descriptor.ObjectVersion || entry.Size != descriptor.ObjectBytes {
			return false, coordinatedInvalid("object_version", "stored object no longer matches the committed acknowledgment")
		}
		return true, nil
	}
	if entry.Size > MaxCoordinatedObjectBytes {
		if registeredID != "" {
			return false, coordinatedInvalid("object_size", "registered generation exceeds its bounded format")
		}
		return false, nil
	}
	value, err := s.objects.Get(ctx, entry.Key)
	if errors.IsNotFound(err) && registeredID == "" {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if value.Version != entry.Version {
		return false, &errors.ConflictError{Resource: "catalog object inventory", Expected: entry.Version, Actual: value.Version}
	}
	generation, err := decodeCoordinatedObject(value.Data, state.StoreID, uploadID)
	if err != nil {
		if registeredID != "" {
			return false, err
		}
		return false, nil
	}
	if registeredID != "" {
		descriptor, _, err := coordinatedCandidate(ctx, generation)
		if err != nil {
			return false, err
		}
		if generation.Manifest.GenerationID != registeredID || !sameCoordinatedCandidate(descriptor, state.Generations[registeredID]) {
			return false, coordinatedInvalid("generation", "owned upload does not match registered content")
		}
	}
	return true, nil
}

var (
	_ RetainingStore      = (*CoordinatedObject)(nil)
	_ AuthorityHeadReader = (*CoordinatedObject)(nil)
)
