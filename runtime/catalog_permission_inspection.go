package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
)

const maxCatalogPermissionRequestBytes = 32 << 10

// RetainedCatalogPermissionRequest binds a selected catalog to its configured consumer and private runtime state.
// The host must verify the catalog payload, source policy, and deployment fence independently.
type RetainedCatalogPermissionRequest struct {
	Directory         string
	Owner             DirectoryOwner
	SchedulerIdentity string
	SourcePolicy      SourcePolicy
	AcceptedHead      catalogs.CatalogAuthorityHead
}

// RetainedCatalogPermission holds original passive permission evidence for a stopped consumer.
// It grants no permission renewal, source acquisition, catalog activation, or replica ownership.
type RetainedCatalogPermission struct {
	record []byte
}

type retainedCatalogPermissionRecord struct {
	Version    int                        `json:"version"`
	RequestSHA string                     `json:"request_sha256"`
	Directory  privatefiles.EntryReceipt  `json:"directory"`
	Layers     privatefiles.EntryReceipt  `json:"layers"`
	Owner      privatefiles.RecordReceipt `json:"owner"`
	Seed       privatefiles.RecordReceipt `json:"seed"`
	Checkpoint privatefiles.RecordReceipt `json:"checkpoint,omitzero"`
	HasReceipt bool                       `json:"has_receipt"`
}

// InspectRetainedCatalogPermission checks original retained permission without opening a connected runtime.
// Writers must remain fenced throughout inspection and rechecks. Internal authority receipts require a qualified current clock sample.
// An uncertain checkpoint requires separate source-owner verification. This operation does not clear uncertainty or extend expiry.
func InspectRetainedCatalogPermission(ctx context.Context, request RetainedCatalogPermissionRequest, clock permission.ClockReading) (*RetainedCatalogPermission, error) {
	requestSHA, err := validateCatalogPermissionRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	directory, err := privatefiles.ExistingDirectory(request.Directory)
	if err != nil {
		return nil, err
	}
	layers, err := directory.ExistingChild(layerDirectoryName)
	if err != nil {
		return nil, err
	}
	if err := inspectMaterializationOwner(ctx, directory, CatalogMaterializationRequest{Owner: request.Owner, SchedulerIdentity: request.SchedulerIdentity}); err != nil {
		return nil, err
	}
	before, err := captureCatalogPermission(ctx, directory, layers, requestSHA)
	if err != nil {
		return nil, err
	}
	if err := checkRetainedCatalogPermission(request, clock, layers, before.HasReceipt); err != nil {
		return nil, err
	}
	if err := inspectMaterializationOwner(ctx, directory, CatalogMaterializationRequest{Owner: request.Owner, SchedulerIdentity: request.SchedulerIdentity}); err != nil {
		return nil, err
	}
	after, err := captureCatalogPermission(ctx, directory, layers, requestSHA)
	if err != nil {
		return nil, err
	}
	if before != after {
		return nil, invalidInputPublication("retained catalog permission changed during inspection")
	}
	record, err := json.Marshal(before, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	return &RetainedCatalogPermission{record: record}, nil
}

// Record returns a private, caller-owned copy of the original native evidence.
// Seal this record with the complete recovery decision before releasing any component.
func (p *RetainedCatalogPermission) Record() []byte {
	if p == nil {
		return nil
	}
	return bytes.Clone(p.record)
}

// Digest returns the SHA-256 digest of the original record, or an empty string for an absent capability.
func (p *RetainedCatalogPermission) Digest() string {
	if p == nil || len(p.record) == 0 {
		return ""
	}
	digest := sha256.Sum256(p.record)
	return hex.EncodeToString(digest[:])
}

// Check verifies the original evidence and current expiry without changing files or receipt deadlines.
// After restart, compare fresh inspection with the record sealed in the original recovery decision.
func (p *RetainedCatalogPermission) Check(ctx context.Context, request RetainedCatalogPermissionRequest, clock permission.ClockReading) error {
	if p == nil || len(p.record) == 0 {
		return invalidInputPublication("original catalog permission evidence is required")
	}
	current, err := InspectRetainedCatalogPermission(ctx, request, clock)
	if err != nil {
		return err
	}
	if !bytes.Equal(p.record, current.record) {
		return invalidInputPublication("retained catalog permission differs from the original recovery decision")
	}
	return nil
}

// Format hides private native evidence from diagnostic output.
func (p *RetainedCatalogPermission) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "<retained catalog permission>")
}

func validateCatalogPermissionRequest(ctx context.Context, request RetainedCatalogPermissionRequest) (string, error) {
	if ctx == nil {
		return "", invalidInputPublication("catalog permission inspection requires a context")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !filepath.IsAbs(request.Directory) || filepath.Clean(request.Directory) != request.Directory {
		return "", invalidInputPublication("catalog permission directory must be absolute and canonical")
	}
	if err := request.Owner.Validate(); err != nil {
		return "", err
	}
	if err := request.SourcePolicy.Validate(); err != nil {
		return "", err
	}
	if request.SourcePolicy.StartupPolicy != StartupRequireAuthority && request.AcceptedHead != (catalogs.CatalogAuthorityHead{}) {
		return "", invalidInputPublication("selected authority catalog requires its configured internal authority policy")
	}
	if request.SourcePolicy.StartupPolicy == StartupRequireAuthority {
		if err := request.AcceptedHead.Validate(); err != nil {
			return "", err
		}
	}
	policy := request.SourcePolicy
	remaining := maxCatalogPermissionRequestBytes
	for _, value := range []string{request.Directory, request.SchedulerIdentity, policy.URL, policy.Repository, policy.Channel, policy.SignerWorkflow} {
		if len(value) > remaining {
			return "", invalidInputPublication("catalog permission request exceeds its byte limit")
		}
		remaining -= len(value)
	}
	data, err := json.Marshal(request, json.Deterministic(true), jsonv1.FormatDurationAsNano(true))
	if err != nil {
		return "", err
	}
	if len(data) > maxCatalogPermissionRequestBytes {
		return "", invalidInputPublication("catalog permission request exceeds its byte limit")
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func captureCatalogPermission(ctx context.Context, directory, layers *privatefiles.Directory, requestSHA string) (record retainedCatalogPermissionRecord, resultErr error) {
	record.Version, record.RequestSHA = 1, requestSHA
	for _, dir := range []*privatefiles.Directory{directory, layers} {
		if err := dir.CheckNoPendingPublications(ctx); err != nil {
			return record, err
		}
	}
	root, err := directory.Open()
	if err != nil {
		return record, err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	layerRoot, err := layers.Open()
	if err != nil {
		return record, err
	}
	defer func() { resultErr = stderrors.Join(resultErr, layerRoot.Close()) }()
	record.Directory, err = privatefiles.CaptureEntry(root, ".", true)
	if err != nil {
		return record, err
	}
	record.Layers, err = privatefiles.CaptureEntry(layerRoot, ".", true)
	if err != nil {
		return record, err
	}
	record.Owner, err = privatefiles.CaptureRecord(root, ownerRecordName, ownerRecordMaxBytes)
	if err != nil {
		return record, err
	}
	record.Seed, err = privatefiles.CaptureRecord(root, instanceSeedFileName, instanceSeedBytes*2)
	if err != nil {
		return record, err
	}
	record.Checkpoint, err = privatefiles.CaptureRecord(layerRoot, permissionCheckpointFile, maxPermissionCheckpointBytes)
	if os.IsNotExist(err) {
		record.Checkpoint, err = privatefiles.RecordReceipt{}, nil
	} else if err == nil {
		record.HasReceipt = true
	}
	if err != nil {
		return record, err
	}
	return record, ctx.Err()
}

func checkRetainedCatalogPermission(request RetainedCatalogPermissionRequest, clock permission.ClockReading, layers *privatefiles.Directory, hasReceipt bool) error {
	if request.SourcePolicy.StartupPolicy != StartupRequireAuthority {
		if hasReceipt {
			return invalidInputPublication("retained authority checkpoint requires its original authority policy")
		}
		return nil
	}
	if !hasReceipt {
		return invalidInputPublication("internal authority permission requires its retained checkpoint")
	}
	store := &layerStore{root: filepath.Join(request.Directory, layerDirectoryName), directory: layers}
	p, err := store.loadPermission(authorityPermissions{authorityID: request.SourcePolicy.AuthorityID, policyID: request.SourcePolicy.PolicyID})
	if err != nil {
		return err
	}
	p, err = p.activate(request.AcceptedHead)
	if err != nil {
		return err
	}
	if !p.retained {
		return invalidInputPublication("retained authority permission is unconfirmed and requires source-owner verification")
	}
	if !clock.Known {
		return invalidInputPublication("internal authority permission requires a qualified current clock sample")
	}
	if !p.allowsNewAttempt(clock.Time, clock.Uncertainty, clock.Known) {
		return invalidInputPublication("retained authority permission is outside its original validity interval")
	}
	return nil
}
