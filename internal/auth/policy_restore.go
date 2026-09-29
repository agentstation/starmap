package auth

import (
	"context"
	stderrors "errors"
	"io"
	"io/fs"

	"github.com/agentstation/starmap/internal/privatefiles"
)

const (
	policyInspectionBatch = 128
	policyInspectionLimit = 100000
)

// InspectFilePolicyStore validates retained selection policy without writes or credential access.
// The caller must fence writers and verify the complete inventory before and after inspection.
// Inspection refuses pending publications. It does not approve replica reuse or acquisition.
func InspectFilePolicyStore(ctx context.Context, path string, owner PolicyOwner, current EnvironmentPolicy) (resultErr error) {
	if ctx == nil {
		return policyStoreError("inspection requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if owner.Product == "" || owner.Deployment == "" || owner.Instance == "" || !validEnvironmentPolicy(current) || current.current() != current {
		return policyStoreError("inspection requires a complete owner and current policy family")
	}
	directory, err := privatefiles.ExistingDirectory(path)
	if err != nil {
		return err
	}
	if err := directory.CheckNoPendingPublications(ctx); err != nil {
		return err
	}
	root, err := directory.Open()
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	listing, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, listing.Close()) }()
	count, hasDefault := 0, false
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, readErr := listing.ReadDir(policyInspectionBatch)
		count += len(entries)
		if count > policyInspectionLimit {
			return policyStoreError("inspection exceeds the policy entry limit")
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			isDefault, err := inspectPolicyEntry(directory, entry, owner, current)
			if err != nil {
				return err
			}
			hasDefault = hasDefault || isDefault
		}
		if stderrors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if !hasDefault {
		return policyStoreError("default policy is absent")
	}
	if err := directory.CheckNoPendingPublications(ctx); err != nil {
		return err
	}
	verified, err := directory.Open()
	if err != nil {
		return err
	}
	return stderrors.Join(verified.Close(), ctx.Err())
}

func inspectPolicyEntry(directory *privatefiles.Directory, entry fs.DirEntry, owner PolicyOwner, current EnvironmentPolicy) (bool, error) {
	if entry.Name() == privatefiles.PublicationDirectoryName && entry.IsDir() {
		return false, nil
	}
	if !entry.Type().IsRegular() {
		return false, policyStoreError("inspection found an unsupported policy entry")
	}
	body, err := directory.ReadFile(entry.Name(), policyRecordLimit)
	if err != nil {
		return false, err
	}
	record, err := decodePolicyRecord(body, owner, current)
	if err != nil {
		return false, err
	}
	if entry.Name() == policyRecordName {
		if record.Provider != "" {
			return false, policyStoreError("default policy contains a provider")
		}
		return true, nil
	}
	if record.Provider == "" || entry.Name() != providerPolicyName(record.Provider) || record.Policy != current {
		return false, policyStoreError("inspection found an invalid provider policy")
	}
	return false, nil
}
