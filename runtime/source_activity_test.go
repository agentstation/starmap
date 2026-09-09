package runtime

import (
	"context"
	stderrors "errors"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"testing"

	"github.com/agentstation/starmap/pkg/sources"
)

type activityCollector struct {
	t        *testing.T
	activity []sources.SourceActivity
	calls    int
}

func (c *activityCollector) AcquireSources(context.Context, SourceAcquisitionRequest) ([]sources.Observation, error) {
	c.t.Error("runtime discarded the detailed source report interface")
	return nil, context.Canceled
}
func (c *activityCollector) AcquireSourceReport(context.Context, SourceAcquisitionRequest) ([]sources.Observation, []sources.SourceActivity, error) {
	c.calls++
	return nil, c.activity, context.Canceled
}

func TestRuntimeRetainsFailedSourceActivity(t *testing.T) {
	collector := &activityCollector{t: t, activity: []sources.SourceActivity{{Source: sources.LocalCatalogID, Supported: true, Enabled: true, Eligibility: sources.EligibilityUnknown}}}
	connected := openTestRuntime(t, WithCatalogSource("embedded"), WithSourceAcquirer(collector), WithAcquisitionEnabled(false))
	report, err := connected.Sync(t.Context())
	if err == nil {
		t.Fatal("missing canceled source error")
	}
	if len(report.SourceActivities) != 1 {
		t.Fatalf("run activity=%+v", report.SourceActivities)
	}
	report.SourceActivities[0].Source = sources.ModelsDevGitID
	collector.activity[0].Source = sources.ModelsDevHTTPID
	for range 2 {
		status := connected.Status()
		if len(status.SourceActivities) != 1 || status.SourceActivities[0].Source != sources.LocalCatalogID {
			t.Fatalf("status activity=%+v", status.SourceActivities)
		}
		status.SourceActivities[0].Source = sources.ProvidersID
	}
	if collector.calls != 1 {
		t.Fatalf("status repeated acquisition: %d calls", collector.calls)
	}
}

func TestRuntimeRejectsMalformedSourceActivity(t *testing.T) {
	collector := &activityCollector{t: t, activity: []sources.SourceActivity{
		{Source: sources.LocalCatalogID, Supported: true, Enabled: true, Eligibility: sources.EligibilityUnknown},
		{Source: sources.LocalCatalogID, Supported: true, Enabled: true, Eligibility: sources.EligibilityUnknown},
		{Source: sources.ProvidersID, Supported: true, Enabled: true, Eligibility: sources.EligibilityUnknown},
		{Source: "private-source", Eligibility: "private-state"},
	}}
	connected := &Runtime{config: options{sourceAcquirer: collector}}
	_, activity, err := connected.acquireSourceReport(t.Context(), SourceAcquisitionRequest{})
	var invalid *pkgerrors.ValidationError
	if !stderrors.As(err, &invalid) || len(activity) != 1 || activity[0].Source != sources.LocalCatalogID {
		t.Fatalf("malformed report=%+v, error=%v", activity, err)
	}
	collector.activity[0].Source = sources.ModelsDevHTTPID
	if activity[0].Source != sources.LocalCatalogID {
		t.Fatal("report aliases collector output")
	}
}
