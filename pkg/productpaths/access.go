package productpaths

import (
	"io/fs"
	"strconv"

	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

func assessManifestAccess(manifest FileManifest, report *FileInspection) {
	policies := make(map[string]string, len(manifest.Files))
	for _, entry := range manifest.Files {
		policies[entry.ID] = entry.Policy.Access
	}
	for index := range report.Observations {
		item := &report.Observations[index]
		item.AccessPolicy = policies[item.ID]
		if item.AccessPolicy == "" {
			continue
		}
		item.AccessStatus = "not-assessed"
		if item.State != "present" {
			continue
		}
		item.AccessStatus = "unverified"
		item.AccessReason = "effective-access-not-verified"
		if item.AccessPolicy == policy.OwnerOnly && item.PermissionScope == "windows-owner-and-dacl" &&
			(item.Kind == "file" || item.Kind == "directory") && item.WindowsSecurity != nil && item.WindowsSecurity.PolicyStatus == "conflict" {
			item.AccessStatus = "conflict"
			item.AccessReason = item.WindowsSecurity.Reason
			continue
		}
		if item.AccessPolicy == policy.ServiceManaged && item.PermissionScope == "windows-owner-and-dacl" &&
			item.Kind == "file" && item.WindowsSecurity != nil && item.WindowsSecurity.ServicePolicyStatus == "conflict" {
			item.AccessStatus = "conflict"
			item.AccessReason = item.WindowsSecurity.ServicePolicyReason
			continue
		}
		if (item.AccessPolicy != policy.OwnerOnly && item.AccessPolicy != policy.ServiceManaged) || item.PermissionScope != "posix-mode-bits-only" ||
			(item.Kind != "file" && item.Kind != "directory") {
			continue
		}
		mode, err := strconv.ParseUint(item.Mode, 8, 32)
		if err != nil {
			continue
		}
		observed := policy.POSIXMetadata{Mode: fs.FileMode(mode), OwnerKnown: item.Owner != nil}
		if item.Owner != nil {
			observed.OwnerMatches = item.Owner.MatchesEffectiveUser
			observed.OwnerTrusted = item.Owner.MatchesEffectiveUser || item.Owner.UID == 0
		}
		reason := policy.PrivatePOSIXReason(observed)
		if item.AccessPolicy == policy.ServiceManaged {
			reason = policy.ServicePOSIXReason(observed)
		}
		if reason != "" && reason != policy.OwnerUnavailable {
			item.AccessStatus = "conflict"
			item.AccessReason = reason
		}
	}
}
