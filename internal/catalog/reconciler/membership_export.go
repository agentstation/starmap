package reconciler

import (
	"cmp"
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// ExportScopes returns effective scope records under the caller's publisher identity.
// Each record retains original receipt identities and excludes credential references.
func (s *MembershipState) ExportScopes(publisher string) ([]catalogs.ProviderMembershipScope, error) {
	var records []catalogs.ProviderMembershipScope
	if s == nil {
		return records, nil
	}
	for _, provider := range s.providers {
		for _, scope := range provider.scopes {
			binding := scope.binding
			record := catalogs.ProviderMembershipScope{
				PublisherID: publisher, BindingID: binding.ID, BindingRevision: binding.Revision,
				ProviderID: binding.ProviderID, AccountID: binding.AccountID, ProjectID: binding.ProjectID,
				Region: binding.Region, APISurface: binding.APISurface, Public: binding.Public,
				Authority: catalogs.MembershipAuthority(binding.MembershipAuthority),
			}
			if scope.inventory != nil {
				inventory := scope.inventory
				models := make([]string, 0, len(inventory.models))
				for model := range inventory.models {
					models = append(models, model)
				}
				slices.Sort(models)
				record.Inventory = &catalogs.MembershipInventory{ObservationID: inventory.fact.observationID, ObservedAt: inventory.fact.at, ModelIDs: models}
			}
			for model, fact := range scope.positive {
				if scope.inventory != nil && !fact.at.After(scope.inventory.fact.at) {
					continue
				}
				record.Additions = append(record.Additions, catalogs.MembershipPresence{ModelID: model, ObservationID: fact.observationID, ObservedAt: fact.at})
			}
			slices.SortFunc(record.Additions, func(a, b catalogs.MembershipPresence) int { return cmp.Compare(a.ModelID, b.ModelID) })
			if err := record.Validate(); err != nil {
				return nil, err
			}
			records = append(records, record)
		}
	}
	slices.SortFunc(records, func(a, b catalogs.ProviderMembershipScope) int {
		return cmp.Or(cmp.Compare(a.ProviderID, b.ProviderID), cmp.Compare(a.BindingID, b.BindingID), cmp.Compare(a.BindingRevision, b.BindingRevision))
	})
	return records, nil
}
