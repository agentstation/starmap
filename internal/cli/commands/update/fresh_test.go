package update

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/acquisition"
)

func TestFreshFlagAliases(t *testing.T) {
	for _, arg := range []string{"--fresh", "--force", "-f"} {
		t.Run(arg, func(t *testing.T) {
			command := &cobra.Command{Use: "update"}
			flags := addUpdateFlags(command)
			if err := command.ParseFlags([]string{arg}); err != nil {
				t.Fatal(err)
			}
			if !flags.Force {
				t.Fatal("reset flag did not select fresh acquisition")
			}
		})
	}
}

type freshPreviewApplication struct {
	called bool
	err    error
	logger zerolog.Logger
}

func (a *freshPreviewApplication) Starmap(...starmap.Option) (*starmap.Client, error) {
	a.called = true
	return nil, a.err
}

func (a *freshPreviewApplication) Logger() *zerolog.Logger { return &a.logger }

func (a *freshPreviewApplication) CatalogAcquisition(*starmap.Client) (*acquisition.Syncer, error) {
	return nil, a.err
}

func (a *freshPreviewApplication) ResolveOperationPath(value, _ string) (string, error) {
	return value, nil
}

func TestFreshDryRunDoesNotAskBeforePreview(t *testing.T) {
	input, err := os.CreateTemp(t.TempDir(), "input")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if _, err := input.WriteString("n\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := input.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	previous := os.Stdin
	os.Stdin = input
	t.Cleanup(func() { os.Stdin = previous })
	stopped := errors.New("stop at catalog construction")
	app := &freshPreviewApplication{err: stopped, logger: zerolog.Nop()}
	command := NewCommand(app)
	command.SetArgs([]string{"--force", "--dry-run"})
	if err := command.ExecuteContext(t.Context()); !errors.Is(err, stopped) || !app.called {
		t.Fatalf("fresh preview did not reach catalog construction: called=%v, err=%v", app.called, err)
	}
	if offset, err := input.Seek(0, io.SeekCurrent); err != nil || offset != 0 {
		t.Fatalf("preview consumed confirmation input: offset=%d, err=%v", offset, err)
	}
}
