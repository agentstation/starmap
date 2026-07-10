// Command starmap-catalog-release stages the verified embedded generation as
// immutable catalog release assets.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogartifact"
)

type releaseReport struct {
	GenerationID    string   `json:"generation_id"`
	ArchiveChecksum string   `json:"archive_checksum"`
	Directory       string   `json:"directory"`
	Files           []string `json:"files"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("starmap-catalog-release", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	outputDir := flags.String("output-dir", "dist/catalog-release", "immutable catalog release staging root")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}
	generation, err := bootstrap.Generation()
	if err != nil {
		return err
	}
	artifact, err := catalogartifact.Build(generation)
	if err != nil {
		return err
	}
	assets, err := catalogartifact.StageReleaseAssets(*outputDir, artifact)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(releaseReport{
		GenerationID: assets.GenerationID, ArchiveChecksum: assets.ArchiveChecksum,
		Directory: assets.Directory, Files: assets.Files,
	})
}
