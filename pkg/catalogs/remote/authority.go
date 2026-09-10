package remote

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// BindAuthorityObserver installs one observer before any manifest request starts.
// A validated current manifest reaches the observer before payload compatibility checks or transfer.
// An ordinary manifest supplies a zero head. Addressed historical reads never call the observer.
// An observer error prevents payload transfer. The callback must support concurrent reads and must not read this client's manifests.
func (c *Client) BindAuthorityObserver(observer func(context.Context, catalogs.CatalogAuthorityHead) error) error {
	if observer == nil {
		return &errors.ValidationError{Field: "catalog_remote.authority_observer", Message: "is required"}
	}
	c.authorityMu.Lock()
	defer c.authorityMu.Unlock()
	if c.manifestStarted || c.authorityObserver != nil {
		return &errors.ConflictError{Resource: "remote authority observer", Message: "must bind once before manifest requests start"}
	}
	c.authorityObserver = observer
	return nil
}

func (c *Client) beginManifestRead() func(context.Context, catalogs.CatalogAuthorityHead) error {
	c.authorityMu.Lock()
	defer c.authorityMu.Unlock()
	c.manifestStarted = true
	return c.authorityObserver
}
