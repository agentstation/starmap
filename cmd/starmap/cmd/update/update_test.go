package update

import (
	"context"
	"testing"

	"github.com/rs/zerolog"

	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type exportPathProviderStub struct {
	path  string
	calls int
}

func (s *exportPathProviderStub) CatalogExportPath() (string, error) {
	s.calls++
	return s.path, nil
}

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

func TestUpdateCatalogLeavesOmittedOutputForConfiguredExportPath(t *testing.T) {
	client := &recordingSyncClient{}
	flags := Flags{AutoApprove: true}
	logger := zerolog.Nop()
	if err := updateCatalogWithConfirmation(context.Background(), client, &flags, &logger, true, func() (bool, error) {
		return true, nil
	}); err != nil {
		t.Fatalf("updateCatalogWithConfirmation: %v", err)
	}
	if len(client.options) != 1 || client.options[0].OutputPath != "" {
		t.Fatalf("sync options = %#v, want omitted output path", client.options)
	}
}

func TestResolveUpdateOutputPath(t *testing.T) {
	t.Run("explicit flag wins without config lookup", func(t *testing.T) {
		app := &exportPathProviderStub{path: "/configured"}
		got, err := resolveUpdateOutputPath(app, "/explicit")
		if err != nil || got != "/explicit" {
			t.Fatalf("resolveUpdateOutputPath = %q, %v", got, err)
		}
		if app.calls != 0 {
			t.Fatalf("CatalogExportPath calls = %d, want 0", app.calls)
		}
	})

	t.Run("omitted flag uses configured or default path", func(t *testing.T) {
		app := &exportPathProviderStub{path: "/configured"}
		got, err := resolveUpdateOutputPath(app, "")
		if err != nil || got != "/configured" {
			t.Fatalf("resolveUpdateOutputPath = %q, %v", got, err)
		}
	})
}
