package acquisition

import (
	"context"
	stderrors "errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/agentstation/starmap/runtime"
)

type sourceReportReader interface {
	AcquireSourceReport(context.Context, runtime.SourceAcquisitionRequest) ([]sources.Observation, []sources.SourceActivity, error)
}

func TestSourceAcquirerReportsCanceledSelection(t *testing.T) {
	acquirer, err := NewSourceAcquirer(pkgsync.WithSources(sources.LocalCatalogID), pkgsync.WithCatalogPath(filepath.Join(t.TempDir(), "absent")))
	if err != nil {
		t.Fatal(err)
	}
	detailed, ok := any(acquirer).(sourceReportReader)
	if !ok {
		t.Fatal("source acquirer has no per-run report contract")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	observations, activity, err := detailed.AcquireSourceReport(ctx, runtime.SourceAcquisitionRequest{Current: acquisitionTestCatalog(t)})
	if !stderrors.Is(err, context.Canceled) || len(observations) != 0 {
		t.Fatalf("canceled acquisition=%v, %d observations", err, len(observations))
	}
	if len(activity) != 3 {
		t.Fatalf("activity=%+v", activity)
	}
	index := slices.IndexFunc(activity, func(row sources.SourceActivity) bool { return row.Source == sources.LocalCatalogID })
	if index < 0 || !activity[index].Enabled || !activity[index].Supported || activity[index].Attempted || activity[index].Eligibility != sources.EligibilityUnknown {
		t.Fatalf("canceled selection=%+v", activity)
	}
	for _, row := range activity {
		if !row.Valid() {
			t.Fatalf("invalid report=%+v", row)
		}
	}
}

func TestSourceConfigurationReportsBuiltInDefaultsWithoutAcquisition(t *testing.T) {
	acquirer, err := NewSourceAcquirer(pkgsync.WithSources(sources.ModelsDevGitID), pkgsync.WithSourcesDir(filepath.Join(t.TempDir(), "not-created")))
	if err != nil {
		t.Fatal(err)
	}
	report := acquirer.SourceConfiguration()
	if len(report) != 3 {
		t.Fatalf("configuration=%+v", report)
	}
	for _, row := range report {
		if !row.Supported || row.SupportUnknown || row.SelectionUnknown || row.Enabled != (row.Source == sources.ModelsDevGitID) || row.Attempted || row.Eligibility != sources.EligibilityUnknown {
			t.Fatalf("configuration=%+v", row)
		}
	}
	report[0].Source = sources.ProvidersID
	if acquirer.SourceConfiguration()[0].Source == sources.ProvidersID {
		t.Fatal("configuration shares caller-owned state")
	}
}
