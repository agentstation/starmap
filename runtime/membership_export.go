package runtime

import (
	"github.com/agentstation/starmap/internal/catalog/reconciler"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func (l *layerSet) attachMembershipScopes(builder *catalogs.Builder, membership *reconciler.MembershipState) error {
	records, err := membership.ExportScopes(l.publisherID)
	if err != nil {
		return err
	}
	selected := make(map[string]bool, len(records))
	for _, record := range records {
		selected[record.BindingID] = true
	}
	var scopes []catalogs.ProviderMembershipScope
	for _, scope := range builder.MembershipScopes() {
		if scope.PublisherID != l.publisherID || !selected[scope.BindingID] {
			scopes = append(scopes, scope)
		}
	}
	return builder.SetMembershipScopes(append(scopes, records...))
}
