package runtime

import (
	"cmp"
	"context"
	"crypto/sha256"
	"slices"

	"github.com/agentstation/starmap/pkg/sources"
)

type manualHistoryPosition struct{ batch, observation int }

type providerHistoryEntry struct {
	position     manualHistoryPosition
	observation  manualObservation
	priority     sources.Observation
	shape        providerHistoryShape
	epoch, order int
	eligible     bool
}

type replacementHistoryRun struct {
	value           [sha256.Size]byte
	first, previous int
	hasPredecessor  bool
}

type providerHistorySegment struct {
	context [sha256.Size]byte
	epoch   int
	entries []providerHistoryEntry
	fields  map[string]replacementHistoryRun
}

func providerHistorySegments(entries []providerHistoryEntry) []*providerHistorySegment {
	var segments []*providerHistorySegment
	active := make(map[sources.ProviderAcquisitionBinding]*providerHistorySegment)
	for _, entry := range entries {
		var binding sources.ProviderAcquisitionBinding
		if selected := entry.observation.Receipt.ProviderBinding; selected != nil {
			binding = *selected
		}
		segment := active[binding]
		if segment == nil || segment.context != entry.shape.context || segment.epoch != entry.epoch || !entry.eligible {
			segment = &providerHistorySegment{context: entry.shape.context, epoch: entry.epoch,
				fields: make(map[string]replacementHistoryRun, len(entry.shape.values))}
			for field, value := range entry.shape.values {
				segment.fields[field] = replacementHistoryRun{value: value}
			}
			segments = append(segments, segment)
			active[binding] = segment
		} else {
			for field, value := range entry.shape.values {
				run := segment.fields[field]
				if run.value != value {
					run.value, run.first = value, len(segment.entries)
					run.previous, run.hasPredecessor = len(segment.entries)-1, true
					segment.fields[field] = run
				}
			}
		}
		segment.entries = append(segment.entries, entry)
		if !entry.eligible {
			delete(active, binding)
		}
	}
	return segments
}

func (s *providerHistorySegment) protect(required map[manualHistoryPosition]bool) {
	required[s.entries[0].position], required[s.entries[len(s.entries)-1].position] = true, true
	for _, run := range s.fields {
		required[s.entries[run.first].position] = true
		if run.hasPredecessor {
			required[s.entries[run.previous].position] = true
		}
	}
}

// A selected account can establish a change between two reports from another account.
// Preserve the first final-value report after each peer's last different value.
// This also preserves change times when operators select a different account subset.
func protectProviderChangeWitnesses(ctx context.Context, segments []*providerHistorySegment, required map[manualHistoryPosition]bool) error {
	for _, target := range segments {
		for field, run := range target.fields {
			if err := ctx.Err(); err != nil {
				return err
			}
			tail := target.entries[run.first:]
			if len(tail) < 3 {
				continue
			}
			for _, peer := range segments {
				if err := ctx.Err(); err != nil {
					return err
				}
				other, exists := peer.fields[field]
				if !exists || peer == target {
					continue
				}
				cutoff := len(peer.entries) - 1
				if other.value == run.value {
					if !other.hasPredecessor {
						continue
					}
					cutoff = other.previous
				}
				order := peer.entries[cutoff].order
				index, equal := slices.BinarySearchFunc(tail, order, func(entry providerHistoryEntry, value int) int { return cmp.Compare(entry.order, value) })
				if equal {
					index++
				}
				if index < len(tail) {
					required[tail[index].position] = true
				}
			}
		}
	}
	return ctx.Err()
}
