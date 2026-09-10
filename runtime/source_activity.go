package runtime

import (
	"context"
	stderrors "errors"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// sourceReportAcquirer supplies activity owned by one acquisition call.
// Collectors without this optional role leave activity unknown.
type sourceReportAcquirer interface {
	AcquireSourceReport(context.Context, SourceAcquisitionRequest) ([]sources.Observation, []sources.SourceActivity, error)
}

func (r *Runtime) acquireSourceReport(ctx context.Context, request SourceAcquisitionRequest) ([]sources.Observation, []sources.SourceActivity, error) {
	detailed, ok := r.config.sourceAcquirer.(sourceReportAcquirer)
	if !ok {
		observations, err := r.config.sourceAcquirer.AcquireSources(ctx, request)
		return observations, nil, err
	}
	observations, activity, err := detailed.AcquireSourceReport(ctx, request)
	var owned []sources.SourceActivity
	seen := make(map[sources.ID]bool)
	invalid := false
	for _, row := range activity {
		if !row.Valid() || row.Source == sources.ProvidersID || seen[row.Source] {
			invalid = true
			continue
		}
		seen[row.Source] = true
		owned = append(owned, row)
	}
	if invalid {
		err = stderrors.Join(err, &errors.ValidationError{Field: "source_acquirer.activity", Message: "must contain unique valid metadata source states"})
	}
	return observations, owned, err
}
