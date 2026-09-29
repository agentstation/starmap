package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/agentstation/starmap/internal/privatefiles"
)

// InspectPolicyPublications selects verified staging evidence for inactive retention.
// The caller preserves these bytes and validates the remaining policy through its owner.
// Complete legacy stages require the configured owner and policy family. No stage becomes accepted policy.
func InspectPolicyPublications(ctx context.Context, owner PolicyOwner, current EnvironmentPolicy, files map[string]privatefiles.RetainedFile, read privatefiles.RetainedRecordReader) ([]string, error) {
	if owner.Product == "" || owner.Deployment == "" || owner.Instance == "" || !validEnvironmentPolicy(current) || current.current() != current {
		return nil, policyStoreError("inspection requires a complete owner and current policy family")
	}
	inactive, err := privatefiles.InspectRetainedPublications(ctx, files, read, func(parent, destination, prefix string) bool {
		return parent == "." && prefix == policyRecordPrefix && policyPublicationDestination(destination)
	})
	if err != nil {
		return nil, err
	}
	selected := make(map[string]bool, len(inactive))
	for _, name := range inactive {
		selected[name] = true
	}
	for name, file := range files {
		if !strings.HasPrefix(name, policyRecordPrefix) || selected[name] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		nonce := strings.TrimPrefix(name, policyRecordPrefix)
		if len(nonce) != 26 || strings.Trim(nonce, "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567") != "" {
			return nil, policyStoreError("legacy policy stage has an invalid name")
		}
		data, err := read(ctx, name, policyRecordLimit)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		if len(data) > policyRecordLimit || int64(len(data)) != file.Size || hex.EncodeToString(sum[:]) != file.SHA256 {
			return nil, policyStoreError("legacy policy stage differs from the verified backup")
		}
		record, err := decodePolicyRecord(data, owner, current)
		if err != nil {
			return nil, err
		}
		if record.Provider != "" && record.Policy != current {
			return nil, policyStoreError("legacy provider stage requires the current policy")
		}
		inactive = append(inactive, name)
	}
	slices.Sort(inactive)
	return inactive, ctx.Err()
}

func policyPublicationDestination(name string) bool {
	if name == policyRecordName {
		return true
	}
	digest, ok := strings.CutPrefix(name, "provider-")
	if !ok {
		return false
	}
	digest, ok = strings.CutSuffix(digest, ".json")
	decoded, err := hex.DecodeString(digest)
	return ok && err == nil && len(decoded) == sha256.Size && hex.EncodeToString(decoded) == digest
}
