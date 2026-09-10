package catalogs

import (
	"cmp"
	"slices"
	"sync"
)

// CanonicalAliasSchemaVersion is the first payload schema that retains canonical rename history.
const CanonicalAliasSchemaVersion uint64 = 9

type canonicalAliasStore struct {
	mu      sync.RWMutex
	records []CanonicalAlias
}

// CanonicalAliasRecords returns caller-owned rename records.
func (b *Builder) CanonicalAliasRecords() []CanonicalAlias {
	b.canonicalAliases.mu.RLock()
	defer b.canonicalAliases.mu.RUnlock()
	return slices.Clone(b.canonicalAliases.records)
}

// SetCanonicalAliasRecords replaces explicit rename records after record validation.
// Build checks targets and cycles. The caller authorizes replacement and validates the preceding inventory.
func (b *Builder) SetCanonicalAliasRecords(records []CanonicalAlias) error {
	prepared, err := prepareCanonicalAliases(records)
	if err != nil {
		return err
	}
	b.canonicalAliases.mu.Lock()
	b.canonicalAliases.records = prepared
	b.canonicalAliases.mu.Unlock()
	return nil
}

// CanonicalAliasRecords returns caller-owned rename records from the accepted catalog.
func (c *Catalog) CanonicalAliasRecords() []CanonicalAlias {
	return c.source.CanonicalAliasRecords()
}

// CanonicalAliases returns the immutable canonical rename index.
// A successful lookup grants no routing permission. Consumers still enforce current removal and membership policies.
func (c *Catalog) CanonicalAliases() *CanonicalAliasIndex {
	return c.canonicalAliases
}

func prepareCanonicalAliases(records []CanonicalAlias) ([]CanonicalAlias, error) {
	prepared := slices.Clone(records)
	slices.SortFunc(prepared, func(a, b CanonicalAlias) int { return cmp.Compare(a.ID, b.ID) })
	for index, record := range prepared {
		if err := record.Validate(); err != nil {
			return nil, err
		}
		if index > 0 && prepared[index-1].ID == record.ID {
			return nil, invalidCanonicalAlias("id", "must have one explicit publisher and target")
		}
	}
	return prepared, nil
}

func mergeCanonicalAliases(left, right []CanonicalAlias) ([]CanonicalAlias, error) {
	combined := make(map[ModelDefinitionID]CanonicalAlias, len(left)+len(right))
	for _, records := range [][]CanonicalAlias{left, right} {
		prepared, err := prepareCanonicalAliases(records)
		if err != nil {
			return nil, err
		}
		for _, record := range prepared {
			if previous, found := combined[record.ID]; found {
				if previous.TargetID != record.TargetID || previous.PublisherID != record.PublisherID {
					return nil, invalidCanonicalAlias("merge", "cannot reassign a retired identity")
				}
				if previous.State == CanonicalAliasRemoved {
					record.State = CanonicalAliasRemoved
				}
			}
			combined[record.ID] = record
		}
	}
	result := make([]CanonicalAlias, 0, len(combined))
	for _, record := range combined {
		result = append(result, record)
	}
	return prepareCanonicalAliases(result)
}

func buildCanonicalAliasIndex(records []CanonicalAlias, definitions map[ModelDefinitionID]ModelDefinition, offerings map[OfferingKey]ProviderOffering) (*CanonicalAliasIndex, error) {
	if len(records) == 0 {
		return nil, nil
	}
	ids := make([]ModelDefinitionID, 0, len(definitions))
	for id := range definitions {
		ids = append(ids, id)
	}
	index, err := NewCanonicalAliasIndex(ids, records...)
	if err != nil {
		return nil, err
	}
	for key, offering := range offerings {
		name := ModelDefinitionID(string(key.ProviderID) + "/" + string(key.ProviderModelID))
		if target, _, found := index.Lookup(name); found && target != offering.DefinitionID {
			return nil, invalidCanonicalAlias("id", "a provider route cannot assign the same request name to another model")
		}
	}
	return index, nil
}
