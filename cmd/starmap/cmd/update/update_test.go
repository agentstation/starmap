package update

import (
	"context"
	"testing"

	"github.com/rs/zerolog"

	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type recordingSyncClient struct {
	options []*pkgsync.Options
}

func (c *recordingSyncClient) Sync(_ context.Context, opts ...pkgsync.Option) (*pkgsync.Result, error) {
	options := pkgsync.Defaults().Apply(opts...)
	c.options = append(c.options, options)
	return &pkgsync.Result{
		TotalChanges: 1,
		DryRun:       options.DryRun,
	}, nil
}

func TestUpdateCatalogCommitBehavior(t *testing.T) {
	tests := []struct {
		name         string
		flags        Flags
		confirmed    bool
		wantDryRuns  []bool
		wantConfirms int
	}{
		{
			name:         "manual decline never commits",
			confirmed:    false,
			wantDryRuns:  []bool{true},
			wantConfirms: 1,
		},
		{
			name:         "manual approval previews then commits once",
			confirmed:    true,
			wantDryRuns:  []bool{true, false},
			wantConfirms: 1,
		},
		{
			name: "auto approval commits once",
			flags: Flags{
				AutoApprove: true,
			},
			wantDryRuns: []bool{false},
		},
		{
			name: "explicit dry run never commits or confirms",
			flags: Flags{
				DryRun: true,
			},
			wantDryRuns: []bool{true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &recordingSyncClient{}
			confirmCount := 0
			confirm := func() (bool, error) {
				confirmCount++
				return tt.confirmed, nil
			}
			tt.flags.OutputDir = t.TempDir()
			logger := zerolog.Nop()
			if err := updateCatalogWithConfirmation(context.Background(), client, &tt.flags, &logger, true, confirm); err != nil {
				t.Fatalf("updateCatalogWithConfirmation: %v", err)
			}

			if confirmCount != tt.wantConfirms {
				t.Fatalf("confirm calls = %d, want %d", confirmCount, tt.wantConfirms)
			}
			if len(client.options) != len(tt.wantDryRuns) {
				t.Fatalf("sync calls = %d, want %d", len(client.options), len(tt.wantDryRuns))
			}
			for i, wantDryRun := range tt.wantDryRuns {
				if client.options[i].DryRun != wantDryRun {
					t.Fatalf("sync call %d DryRun = %v, want %v", i, client.options[i].DryRun, wantDryRun)
				}
			}
		})
	}
}
