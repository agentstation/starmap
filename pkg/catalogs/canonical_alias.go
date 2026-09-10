package catalogs

import (
	"slices"

	"github.com/agentstation/starmap/pkg/errors"
)

// CanonicalAliasState distinguishes a usable alias from an explicitly removed alias.
type CanonicalAliasState string

const (
	// CanonicalAliasActive resolves the former canonical ID to its current definition.
	CanonicalAliasActive CanonicalAliasState = "active"
	// CanonicalAliasRemoved reserves the retired ID without resolving client requests.
	CanonicalAliasRemoved CanonicalAliasState = "removed"
)

// CanonicalAlias retains one immutable rename edge and its explicit state.
// PublisherID requires separate source authentication. Removed edges preserve model identity across later renames and restores.
type CanonicalAlias struct {
	ID          ModelDefinitionID   `json:"id" yaml:"id"`
	TargetID    ModelDefinitionID   `json:"target_id" yaml:"target_id"`
	PublisherID string              `json:"publisher_id" yaml:"publisher_id"`
	State       CanonicalAliasState `json:"state" yaml:"state"`
}

// Validate checks one explicit rename record independently of its catalog and transport.
func (a CanonicalAlias) Validate() error {
	if _, _, err := ParseModelDefinitionID(a.ID); err != nil {
		return err
	}
	if _, _, err := ParseModelDefinitionID(a.TargetID); err != nil {
		return err
	}
	if a.ID == a.TargetID {
		return invalidCanonicalAlias("target_id", "must differ from the retired ID")
	}
	if !validMembershipIdentifier(a.PublisherID) {
		return invalidCanonicalAlias("publisher_id", "must be a bounded nonempty publisher identity")
	}
	if a.State != CanonicalAliasActive && a.State != CanonicalAliasRemoved {
		return invalidCanonicalAlias("state", "must explicitly select active or removed")
	}
	return nil
}

// CanonicalAliasIndex owns immutable rename records and precomputed terminal targets.
// Request lookups allocate no memory. They do not traverse graphs or read storage.
type CanonicalAliasIndex struct {
	records   map[ModelDefinitionID]CanonicalAlias
	terminals map[ModelDefinitionID]ModelDefinitionID
}

// NewCanonicalAliasIndex validates and copies a complete retained alias inventory.
// Live definitions cannot reuse retired IDs. Active aliases require a live terminal definition.
func NewCanonicalAliasIndex(definitions []ModelDefinitionID, records ...CanonicalAlias) (*CanonicalAliasIndex, error) {
	live := make(map[ModelDefinitionID]bool, len(definitions))
	for _, id := range definitions {
		if _, _, err := ParseModelDefinitionID(id); err != nil {
			return nil, err
		}
		live[id] = true
	}
	index := &CanonicalAliasIndex{
		records:   make(map[ModelDefinitionID]CanonicalAlias, len(records)),
		terminals: make(map[ModelDefinitionID]ModelDefinitionID, len(records)),
	}
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return nil, err
		}
		if _, exists := index.records[record.ID]; exists {
			return nil, invalidCanonicalAlias("id", "must have one explicit publisher and target")
		}
		if live[record.ID] {
			return nil, invalidCanonicalAlias("id", "a live definition cannot reuse a retired ID")
		}
		index.records[record.ID] = record
	}
	for _, record := range index.Records() {
		terminal, err := index.resolve(record.ID)
		if err != nil {
			return nil, err
		}
		if record.State == CanonicalAliasActive && !live[terminal] {
			return nil, invalidCanonicalAlias("target_id", "an active alias requires a live terminal definition")
		}
	}
	return index, nil
}

// Records returns caller-owned rename records in retired-ID order.
func (i *CanonicalAliasIndex) Records() []CanonicalAlias {
	if i == nil {
		return nil
	}
	ids := make([]ModelDefinitionID, 0, len(i.records))
	for id := range i.records {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	result := make([]CanonicalAlias, 0, len(ids))
	for _, id := range ids {
		result = append(result, i.records[id])
	}
	return result
}

// Lookup returns an alias's terminal target, explicit state, and whether the index reserves its ID.
// A removed alias still returns its target for diagnostics. Callers must refuse that ID before applying convenience lookup rules.
func (i *CanonicalAliasIndex) Lookup(id ModelDefinitionID) (ModelDefinitionID, CanonicalAliasState, bool) {
	if i == nil {
		return "", "", false
	}
	record, found := i.records[id]
	if !found {
		return "", "", false
	}
	return i.terminals[id], record.State, true
}

// ValidateSuccessor prevents a new inventory from dropping or reassigning retired IDs.
// Explicit state changes can remove or restore an alias. Later renames extend the existing edge chain.
func (i *CanonicalAliasIndex) ValidateSuccessor(next *CanonicalAliasIndex) error {
	if i == nil {
		return nil
	}
	for _, previous := range i.Records() {
		if next == nil {
			return invalidCanonicalAlias("inventory", "must retain earlier rename edges")
		}
		record, found := next.records[previous.ID]
		if !found || record.TargetID != previous.TargetID || record.PublisherID != previous.PublisherID {
			return invalidCanonicalAlias("inventory", "cannot drop or reassign a retired ID")
		}
	}
	return nil
}

// resolve flattens one edge chain without recursion. Removed intermediate IDs do not remove other aliases.
func (i *CanonicalAliasIndex) resolve(start ModelDefinitionID) (ModelDefinitionID, error) {
	if terminal, found := i.terminals[start]; found {
		return terminal, nil
	}
	path := make([]ModelDefinitionID, 0)
	visiting := make(map[ModelDefinitionID]bool)
	current := start
	for {
		if terminal, found := i.terminals[current]; found {
			current = terminal
			break
		}
		record, found := i.records[current]
		if !found {
			break
		}
		if visiting[current] {
			return "", invalidCanonicalAlias("target_id", "rename edges must not form a cycle")
		}
		visiting[current] = true
		path = append(path, current)
		if target, found := i.records[record.TargetID]; found && target.PublisherID != record.PublisherID {
			return "", invalidCanonicalAlias("publisher_id", "a rename chain cannot cross publisher authority")
		}
		current = record.TargetID
	}
	for _, id := range path {
		i.terminals[id] = current
	}
	return current, nil
}

func invalidCanonicalAlias(field, message string) error {
	return &errors.ValidationError{Field: "canonical_alias." + field, Message: message}
}
