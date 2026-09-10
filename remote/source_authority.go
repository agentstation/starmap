package remote

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// BindAuthorityObserver connects the runtime's requirement recorder before source acquisition starts.
// The protocol reports verified current heads before payload processing, including failed and incompatible transfers.
func (s *Source) BindAuthorityObserver(observer func(context.Context, catalogs.CatalogAuthorityHead) error) error {
	return s.subscriber.protocol.BindAuthorityObserver(observer)
}
