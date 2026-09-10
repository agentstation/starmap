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
	return builder.SetMembershipScopes(append(builder.MembershipScopes(), records...))
}
