package update

import (
	"context"
	stderrors "errors"
	"os"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/rs/zerolog"
)

type activitySyncClient struct{ calls, failAt int }

func (c *activitySyncClient) Sync(_ context.Context, opts ...pkgsync.Option) (*pkgsync.Result, error) {
	c.calls++
	accepted := &sources.AcceptedSourceState{GenerationID: "prior", Sources: []sources.ID{sources.ModelsDevHTTPID}}
	rows := []sources.SourceActivity{{Source: sources.ProvidersID, Supported: true, Enabled: true, Eligibility: sources.EligibilityIneligible, Attempted: true}, {Source: "private-source", Eligibility: "private-reason"}}
	if c.calls == c.failAt {
		return nil, &sources.ActivityError{Activities: rows, AcceptedSources: accepted, Err: context.Canceled}
	}
	return &pkgsync.Result{SourceActivities: rows, AcceptedSources: accepted, TotalChanges: 1, DryRun: pkgsync.Defaults().Apply(opts...).DryRun}, nil
}

func TestUpdateReportsSourceActivityOnSuccessAndFailure(t *testing.T) {
	for _, test := range []struct {
		name   string
		failAt int
		auto   bool
		quiet  bool
	}{
		{"publisher-success", 0, true, false}, {"initial-failure", 1, true, false}, {"apply-failure", 2, false, false}, {"quiet-success", 0, true, true}, {"quiet-failure", 1, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			output, err := os.CreateTemp(t.TempDir(), "stderr")
			if err != nil {
				t.Fatal(err)
			}
			previous := os.Stderr
			os.Stderr = output
			t.Cleanup(func() { os.Stderr = previous; _ = output.Close() })
			logger := zerolog.Nop()
			client := &activitySyncClient{failAt: test.failAt}
			err = updateCatalogWithConfirmation(t.Context(), client, &Flags{AutoApprove: test.auto}, &logger, test.quiet, func() (bool, error) { return true, nil })
			if (err != nil) != (test.failAt != 0) || (err != nil && !stderrors.Is(err, context.Canceled)) {
				t.Fatalf("update error=%v", err)
			}
			raw, readErr := os.ReadFile(output.Name())
			if readErr != nil {
				t.Fatal(readErr)
			}
			text := string(raw)
			if strings.Contains(text, "providers: supported=yes enabled=yes eligible=ineligible attempted=yes") == test.quiet || strings.Contains(text, "private-") {
				t.Fatalf("source report=%s", text)
			}

			if strings.Contains(text, `Accepted acquisition inputs for generation "prior": models_dev_http.`) == test.quiet {
				t.Fatalf("accepted input report=%s", text)
			}
			want := 1
			if !test.auto {
				want = 2
			}
			if test.quiet {
				want = 0
			}
			if strings.Count(text, "providers: supported=") != want {
				t.Fatalf("report count: %s", text)
			}
		})
	}
}

type activityWriterFailure struct{ err error }

func (w activityWriterFailure) Write([]byte) (int, error) { return 0, w.err }

func TestSourceActivityWriteFailurePreservesOriginalError(t *testing.T) {
	failure := stderrors.New("writer refused output")
	original := &sources.ActivityError{Activities: []sources.SourceActivity{{Source: sources.LocalCatalogID, SupportUnknown: true, SelectionUnknown: true, Eligibility: sources.EligibilityUnknown}}, Err: context.Canceled}
	err := displayFailedSourceActivity(activityWriterFailure{err: failure}, original)
	if !stderrors.Is(err, context.Canceled) || !stderrors.Is(err, failure) {
		t.Fatalf("combined error=%v", err)
	}
}
