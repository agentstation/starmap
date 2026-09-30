package productfiles

import "context"

// CompletePublication finishes an exact record after the host checks its original decision and native receipt.
// It refuses foreign journals, changed native identities, and incomplete staging evidence.
// The callback runs under the writer lock. It must not publish or recover records in this directory.
// An already published exact value permits a retry. This operation grants no host transaction authority.
func (d *Directory) CompletePublication(ctx context.Context, name string, previous, data []byte, check func(context.Context) error) error {
	if err := d.checkContext(ctx); err != nil {
		return err
	}
	return d.private.CompletePublication(ctx, name, previous, data, ".product-stage-", check)
}

// InspectPublication checks an exact pending write without cleanup or publication.
// Stage names let the host read its original decision while those checked records remain pending.
// Names grant no completion or transaction authority. CompletePublication rechecks every native receipt.
func (d *Directory) InspectPublication(ctx context.Context, name string, previous, data []byte) ([]string, error) {
	if err := d.checkContext(ctx); err != nil {
		return nil, err
	}
	return d.private.InspectPublication(ctx, name, previous, data, ".product-stage-")
}
