package runtime

import (
	"bytes"
	"context"
	"encoding/hex"
	stderrors "errors"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/privatefiles"
	githubsource "github.com/agentstation/starmap/internal/sources/github"
)

// InspectRetainedDirectory validates private runtime records without opening a runtime.
// The caller must fence writers and verify the complete inventory around inspection.
// This check preserves identity and replay evidence. It does not approve replica reuse,
// deployment settings, accepted-catalog consistency, permission freshness, or admission.
// Path-bound migration records require their separate recovery procedure.
func InspectRetainedDirectory(ctx context.Context, path string, owner DirectoryOwner, identity string) error {
	if ctx == nil {
		return invalidInputPublication("inspection requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := owner.Validate(); err != nil {
		return err
	}
	directory, err := privatefiles.ExistingDirectory(path)
	if err != nil {
		return err
	}
	expected, err := encodeOwnerRecord(owner, identity)
	if err != nil {
		return err
	}
	actual, err := directory.ReadFile(ownerRecordName, ownerRecordMaxBytes)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return invalidInputPublication("retained directory belongs to another owner or scheduler identity")
	}
	seed, err := directory.ReadFile(instanceSeedFileName, int64(hex.EncodedLen(instanceSeedBytes)))
	if err != nil {
		return err
	}
	decoded, err := hex.DecodeString(string(seed))
	if err != nil || len(decoded) != instanceSeedBytes || hex.EncodeToString(decoded) != string(seed) {
		return invalidInputPublication("retained directory has an invalid instance seed")
	}
	if err := inspectRuntimeInventory(ctx, directory); err != nil {
		return err
	}
	store, err := existingLayerStore(path)
	if err != nil {
		return err
	}
	if err := store.inspectRetainedRecords(ctx); err != nil {
		return err
	}
	discovery := filepath.Join(path, "github-catalog-source")
	if _, err := os.Lstat(discovery); err == nil {
		if err := githubsource.InspectStateDirectory(ctx, discovery); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	verified, err := directory.Open()
	if err != nil {
		return err
	}
	return stderrors.Join(verified.Close(), ctx.Err())
}

func (s *layerStore) inspectRetainedRecords(ctx context.Context) error {
	source, err := s.loadSource()
	if err != nil {
		return err
	}
	if source != nil {
		if err := validateSourceInput(source); err != nil {
			return err
		}
	}
	if _, err := s.loadProviders(); err != nil {
		return err
	}
	if _, err := s.loadManualHistory(ctx); err != nil {
		return err
	}
	if _, err := s.loadRemovals(); err != nil {
		return err
	}
	if _, err := s.loadPinRecord(); err != nil {
		return err
	}
	if err := inspectMaterializationJournals(ctx, s); err != nil {
		return err
	}
	if _, err := s.loadRecoveryBaseline(ctx); err != nil {
		return err
	}
	if err := s.inspectCatalogRecovery(ctx); err != nil {
		return err
	}
	if err := s.inspectRetainedPermission(); err != nil {
		return err
	}
	publication, err := s.loadInputPublication()
	if err != nil || publication == nil {
		return err
	}
	if _, _, err := s.publicationInputs(*publication); err != nil {
		return err
	}
	if publication.Manual != "" {
		if _, err := s.readManualHistory(ctx, publication.Manual); err != nil {
			return err
		}
	}
	if _, err := s.readRemovalInput(publication.Removals); err != nil {
		return err
	}
	return ctx.Err()
}

func (s *layerStore) inspectRetainedPermission() error {
	if !s.durable() {
		return nil
	}
	raw, err := s.directory.ReadFile(permissionCheckpointFile, maxPermissionCheckpointBytes)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var record permissionCheckpoint
	if err := decodeInputRecord(raw, &record); err != nil {
		return err
	}
	if record.AuthorityID == "" || record.PolicyID == "" {
		return invalidInputPublication("retained permission requires its authority and policy identity")
	}
	_, err = s.loadPermission(authorityPermissions{authorityID: record.AuthorityID, policyID: record.PolicyID})
	return err
}
