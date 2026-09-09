package update

import (
	stderrors "errors"
	"fmt"
	"io"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func displaySourceActivity(out io.Writer, activity []sources.SourceActivity) error {
	for _, row := range activity {
		if !row.Valid() {
			continue
		}
		if _, err := fmt.Fprintf(out, "  %s: supported=%s enabled=%s eligible=%s attempted=%s\n", row.Source, activityFlag(row.Supported, row.SupportUnknown), activityFlag(row.Enabled, row.SelectionUnknown), row.Eligibility, activityFlag(row.Attempted, false)); err != nil {
			return errors.WrapResource("write", "source activity summary", "", err)
		}
	}
	return nil
}

func displayFailedSourceActivity(out io.Writer, err error) error {
	if writeErr := displaySourceActivity(out, sources.ActivityFromError(err)); writeErr != nil {
		return stderrors.Join(err, writeErr)
	}
	return err
}

func activityFlag(value, unknown bool) string {
	if unknown {
		return "unknown"
	}
	if value {
		return "yes"
	}
	return "no"
}
