package productpaths

import (
	"io/fs"
	"strconv"

	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

func assessManifestAccess(manifest FileManifest, report *FileInspection) {
	entries := make(map[string]FileEntry, len(manifest.Files))
	for _, entry := range manifest.Files {
		entries[entry.ID] = entry
	}
	for index := range report.Observations {
		item := &report.Observations[index]
		entry := entries[item.ID]
		if entry.Kind == "patterns" && item.Path == entry.Location.Path {
			item.AccessPolicy = ""
			item.AccessStatus = "not-assessed"
			item.AccessReason = "pattern-anchor"
			continue
		}
		item.AccessPolicy = entry.Policy.Access
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
		if item.AccessPolicy != policy.OwnerOnly || item.PermissionScope != "posix-mode-bits-only" ||
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
		}
		reason := policy.PrivatePOSIXReason(observed)
		if reason == policy.GroupOrOtherModeBits || reason == policy.DifferentEffectiveOwner {
			item.AccessStatus = "conflict"
			item.AccessReason = reason
		}
	}
}
