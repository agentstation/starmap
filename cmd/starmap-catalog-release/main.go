// Command starmap-catalog-release stages the verified embedded generation as
// immutable catalog release assets.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogartifact"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
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
	verifyDir := flags.String("verify-dir", "", "verify an existing catalog release asset directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return &pkgerrors.ValidationError{Field: "catalog_release.arguments", Value: flags.Args(), Message: "positional arguments are not supported"}
	}
	var outputDirExplicit bool
	flags.Visit(func(current *flag.Flag) {
		if current.Name == "output-dir" {
			outputDirExplicit = true
		}
	})
	if strings.TrimSpace(*verifyDir) != "" {
		if outputDirExplicit {
			return &pkgerrors.ValidationError{Field: "catalog_release.mode", Message: "output-dir and verify-dir are mutually exclusive"}
		}
		report, err := verifyReleaseDirectory(strings.TrimSpace(*verifyDir))
		if err != nil {
			return err
		}
		return json.NewEncoder(output).Encode(report)
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

func verifyReleaseDirectory(directory string) (releaseReport, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return releaseReport{}, pkgerrors.WrapIO("resolve", directory, err)
	}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return releaseReport{}, pkgerrors.WrapIO("read", absolute, err)
	}
	if len(entries) != 3 {
		return releaseReport{}, &pkgerrors.ValidationError{
			Field: "catalog_release.files", Value: len(entries), Message: "release directory must contain exactly three assets",
		}
	}

	files := []string{
		filepath.Join(absolute, catalogartifact.Filename),
		filepath.Join(absolute, catalogartifact.AttestationFilename),
		filepath.Join(absolute, catalogartifact.ChecksumFilename),
	}
	archive, err := readReleaseAsset(files[0])
	if err != nil {
		return releaseReport{}, err
	}
	statement, err := readReleaseAsset(files[1])
	if err != nil {
		return releaseReport{}, err
	}
	checksumFile, err := readReleaseAsset(files[2])
	if err != nil {
		return releaseReport{}, err
	}

	digest := sha256.Sum256(archive)
	digestHex := fmt.Sprintf("%x", digest)
	wantChecksumFile := digestHex + "  " + catalogartifact.Filename + "\n"
	if string(checksumFile) != wantChecksumFile {
		return releaseReport{}, &pkgerrors.ValidationError{
			Field: "catalog_release.checksum", Value: strings.TrimSpace(string(checksumFile)), Message: "does not match archive bytes",
		}
	}
	generation, err := catalogartifact.Open(archive, statement)
	if err != nil {
		return releaseReport{}, pkgerrors.WrapResource("verify", "catalog release", absolute, err)
	}
	return releaseReport{
		GenerationID:    generation.Manifest.GenerationID,
		ArchiveChecksum: "sha256:" + digestHex,
		Directory:       absolute,
		Files:           files,
	}, nil
}

func readReleaseAsset(path string) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the caller selects a release directory and filenames are fixed.
	if err != nil {
		return nil, pkgerrors.WrapIO("read", path, err)
	}
	return data, nil
}
