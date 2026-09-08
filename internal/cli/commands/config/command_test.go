package config

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/productpaths"
)

type reportApplication struct {
	format string
	calls  int
}

func (app *reportApplication) OutputFormat() string { return app.format }
func (app *reportApplication) InspectFiles(_ context.Context, _ int) (productpaths.FileManifest, error) {
	report, err := app.FileManifest()
	report.Inspection = &productpaths.FileInspection{Complete: true, EntryLimit: 10000, Observations: []productpaths.FileObservation{{ID: "runtime-owner", Path: "/runtime/owner.json", State: "absent"}}}
	return report, err
}
func (app *reportApplication) FileManifest() (productpaths.FileManifest, error) {
	app.calls++
	return productpaths.FileManifest{SchemaVersion: 1, Product: productpaths.Starmap, Roots: productpaths.Roots{productpaths.Config: {Path: "/config", Origin: "selected"}}, Files: []productpaths.FileEntry{{ID: "runtime-owner", Location: productpaths.Path{Path: "/runtime/owner.json", Origin: "selected"}, Availability: "available"}}}, nil
}

func TestPathsFormatsAndArgumentRefusal(t *testing.T) {
	for _, kind := range []string{"json", "yaml", "table", "wide"} {
		t.Run(kind, func(t *testing.T) {
			app := &reportApplication{format: kind}
			command := NewCommand(app)
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs([]string{"paths"})
			if err := command.ExecuteContext(t.Context()); err != nil {
				t.Fatal(err)
			}
			if app.calls != 1 || !strings.Contains(output.String(), "runtime-owner") || !strings.Contains(output.String(), "/config") {
				t.Fatalf("incomplete %s report: %s", kind, output.String())
			}
		})
	}
	for _, arguments := range [][]string{{"paths", "extra"}, {"paths"}} {
		app := &reportApplication{format: "invalid"}
		command := NewCommand(app)
		command.SetOut(io.Discard)
		command.SetErr(io.Discard)
		command.SetArgs(arguments)
		if err := command.ExecuteContext(t.Context()); err == nil {
			t.Fatal("invalid report request succeeded")
		}
		if app.calls != 0 {
			t.Fatal("invalid report request reached application")
		}
	}
}

func TestInspectionFormatsAndLimits(t *testing.T) {
	for _, kind := range []string{"json", "yaml", "table", "wide"} {
		t.Run(kind, func(t *testing.T) {
			app := &reportApplication{format: kind}
			command := NewCommand(app)
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetArgs([]string{"paths", "--inspect"})
			if err := command.ExecuteContext(t.Context()); err != nil {
				t.Fatal(err)
			}
			if app.calls != 1 || !strings.Contains(output.String(), "absent") || !strings.Contains(output.String(), "/runtime/owner.json") {
				t.Fatalf("inspection omits metadata: %s", output.String())
			}
			if (kind == "table" || kind == "wide") && !strings.Contains(output.String(), "Complete: true") {
				t.Fatal("text output hides inspection completeness")
			}
		})
	}
	for _, args := range [][]string{{"paths", "--max-entries", "10"}, {"paths", "--inspect", "--max-entries", "0"}, {"paths", "--inspect", "--max-entries", "100001"}} {
		app := &reportApplication{format: "json"}
		command := NewCommand(app)
		command.SetErr(io.Discard)
		command.SetArgs(args)
		if err := command.ExecuteContext(t.Context()); err == nil || app.calls != 0 {
			t.Fatal("invalid inspection options reached application reads")
		}
	}
}
