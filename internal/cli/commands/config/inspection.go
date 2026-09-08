package config

import (
	"fmt"
	"io"

	"github.com/agentstation/starmap/internal/cli/format"
	"github.com/agentstation/starmap/pkg/productpaths"
)

func formatInspection(writer io.Writer, kind format.Kind, report productpaths.FileInspection) error {
	data := format.Data{Headers: []string{"Path role", "Location", "State", "Type", "Mode", "Owner", "Owner binding"}}
	if kind == format.FormatWide {
		data.Headers = append(data.Headers, "Permission scope", "Reason", "Access policy", "Access status", "Access reason", "DACL", "Native reason")
	}
	for _, item := range report.Observations {
		owner := "unverified"
		if item.Owner != nil {
			owner = fmt.Sprintf("%d:%d", item.Owner.UID, item.Owner.GID)
		} else if item.WindowsSecurity != nil && item.WindowsSecurity.OwnerSID != "" {
			owner = item.WindowsSecurity.OwnerSID
		}
		row := []string{item.ID, item.Path, item.State, item.Kind, item.Mode, owner, item.OwnerBinding}
		if kind == format.FormatWide {
			dacl, nativeReason := "", ""
			if item.WindowsSecurity != nil {
				dacl, nativeReason = item.WindowsSecurity.DACLState, item.WindowsSecurity.Reason
			}
			row = append(row, item.PermissionScope, item.Reason, item.AccessPolicy, item.AccessStatus, item.AccessReason, dacl, nativeReason)
		}
		data.Rows = append(data.Rows, row)
	}
	if err := format.New(kind).Format(writer, data); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Metadata scan: %d/%d entries. Complete: %t.\n", report.Examined, report.EntryLimit, report.Complete); err != nil {
		return err
	}
	for _, note := range report.Limitations {
		if _, err := fmt.Fprintln(writer, note); err != nil {
			return err
		}
	}
	return nil
}
