package starmap

import "github.com/agentstation/starmap/pkg/catalogs"

// CurrentAuthorityHead returns the authority head of the committed in-memory publication.
// Construction and publication validate it with the complete generation. Ordinary generations return the zero head.
// The read allocates no memory and reads no storage. It does not authenticate an authority or renew permission.
func (c *Client) CurrentAuthorityHead() catalogs.CatalogAuthorityHead {
	if c == nil {
		return catalogs.CatalogAuthorityHead{}
	}
	c.mu.RLock()
	head := c.generationAuthorityHead
	c.mu.RUnlock()
	return head
}
