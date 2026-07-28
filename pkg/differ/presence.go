package differ

import (
	"fmt"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func diffModelDescription(existing, updated catalogs.Model) []FieldChange {
	existingValue, existingPresence := existing.DescriptionValue()
	updatedValue, updatedPresence := updated.DescriptionValue()
	if existingValue == updatedValue && existingPresence == updatedPresence {
		return nil
	}
	return []FieldChange{{
		Path:     "description",
		OldValue: formatPresence(existingValue, existingPresence),
		NewValue: formatPresence(updatedValue, updatedPresence),
		Type:     ChangeTypeUpdate,
	}}
}

func formatPresence(value any, presence catalogs.ValuePresence) string {
	switch presence {
	case catalogs.ValueMissing:
		return "missing"
	case catalogs.ValueUnknown:
		return "unknown"
	default:
		return fmt.Sprint(value)
	}
}
