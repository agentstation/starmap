package runtime

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
)

// preparePinGeneration preserves ordinary bytes and gives an origin rollback a new sequence.
// Exact preparation also recognizes another replica's identical completed selection.
func (r *Runtime) preparePinGeneration(ctx context.Context, target catalogs.Generation, record *generationPinRecord) (catalogs.Generation, error) {
	if r.config.origin == nil {
		return target, nil
	}
	current, err := r.client.CurrentGeneration(ctx)
	if err != nil {
		return catalogs.Generation{}, err
	}
	if err := r.config.origin.validateCurrent(current); err != nil {
		return catalogs.Generation{}, err
	}
	if target.Manifest.GenerationID == current.Manifest.GenerationID {
		if !samePinGeneration(target, current) {
			return catalogs.Generation{}, pinRecordConflict("the selected generation differs between storage reads")
		}
		return current, nil
	}
	input := target.Copy()
	input.Manifest.ManifestVersion = catalogs.CurrentGenerationManifestVersion
	input.Manifest.AuthorityHead = catalogs.CatalogAuthorityHead{}
	sequence := current.Manifest.AuthorityHead.Sequence
	if record != nil && record.Phase == pinPrepared {
		sequence = record.Receipt.AuthorityHead.Sequence
	}
	prepared, err := permission.PrepareGeneration(input, permission.GenerationConfig{
		AuthorityID: r.config.origin.config.AuthorityID, PolicyID: r.config.origin.config.PolicyID, Sequence: sequence})
	if err != nil {
		return catalogs.Generation{}, err
	}
	if samePinGeneration(prepared, current) {
		return current, nil
	}
	if record != nil && record.Phase == pinAccepted {
		return catalogs.Generation{}, pinRecordConflict("the active authority does not derive from the accepted pin")
	}
	expected := current.Manifest.GenerationID
	if record != nil && record.Phase == pinPrepared {
		expected = record.Receipt.PreviousGenerationID
		if current.Manifest.GenerationID != expected {
			return catalogs.Generation{}, pinRecordConflict("the pending selection no longer has its recorded predecessor")
		}
	}
	return r.config.origin.publisher.PrepareCatalog(ctx, input, expected)
}

func samePinGeneration(left, right catalogs.Generation) bool {
	if !bytes.Equal(left.Payload, right.Payload) {
		return false
	}
	a, errA := json.Marshal(left.Manifest)
	b, errB := json.Marshal(right.Manifest)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}

func pinRecordMatches(record generationPinRecord, generation catalogs.Generation) bool {
	receipt := record.Receipt
	return receipt.AcceptedGenerationID == generation.Manifest.GenerationID && receipt.PayloadChecksum == generation.Manifest.Payload.Checksum && receipt.AuthorityHead == generation.Manifest.AuthorityHead
}
