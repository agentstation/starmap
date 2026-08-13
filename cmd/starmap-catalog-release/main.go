// Command starmap-catalog-release stages one exact committed generation as
// immutable catalog release assets.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

type releaseReport struct {
	GenerationID     string   `json:"generation_id"`
	SemanticChecksum string   `json:"semantic_checksum"`
	PayloadChecksum  string   `json:"payload_checksum"`
	ArchiveChecksum  string   `json:"archive_checksum"`
	Directory        string   `json:"directory"`
	Files            []string `json:"files"`
}

type inspectionReport struct {
	GenerationID          string                         `json:"generation_id"`
	ManifestVersion       uint64                         `json:"manifest_version"`
	SchemaVersion         uint64                         `json:"schema_version"`
	ConsumerCompatibility catalogs.ConsumerCompatibility `json:"consumer_compatibility"`
	CurrentSchemaVersion  uint64                         `json:"current_schema_version"`
	SupportsCurrentSchema bool                           `json:"supports_current_schema"`
	ArchiveChecksum       string                         `json:"archive_checksum"`
	Directory             string                         `json:"directory"`
	Files                 []string                       `json:"files"`
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
	inspectDir := flags.String(
		"inspect-dir",
		"",
		"verify a release envelope and report schema compatibility without decoding its payload",
	)
	generationStore := flags.String(
		"generation-store",
		"",
		"filesystem store containing the exact committed generation to stage",
	)
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
	if strings.TrimSpace(*inspectDir) != "" {
		if outputDirExplicit || strings.TrimSpace(*generationStore) != "" || strings.TrimSpace(*verifyDir) != "" {
			return &pkgerrors.ValidationError{
				Field:   "catalog_release.mode",
				Message: "inspect-dir cannot be combined with output-dir, generation-store, or verify-dir",
			}
		}
		report, err := inspectReleaseDirectory(strings.TrimSpace(*inspectDir))
		if err != nil {
			return err
		}
		return json.NewEncoder(output).Encode(report)
	}
	if strings.TrimSpace(*verifyDir) != "" {
		if outputDirExplicit || strings.TrimSpace(*generationStore) != "" {
			return &pkgerrors.ValidationError{
				Field:   "catalog_release.mode",
				Message: "verify-dir cannot be combined with output-dir or generation-store",
			}
		}
		report, err := verifyReleaseDirectory(strings.TrimSpace(*verifyDir))
		if err != nil {
			return err
		}
		return json.NewEncoder(output).Encode(report)
	}
	if strings.TrimSpace(*generationStore) == "" {
		return &pkgerrors.ValidationError{
			Field:   "catalog_release.generation_store",
			Message: "is required when staging release assets",
		}
	}
	store, err := storage.NewFilesystem(strings.TrimSpace(*generationStore))
	if err != nil {
		return err
	}
	generation, err := store.Current(context.Background())
	if err != nil {
		return pkgerrors.WrapResource(
			"read",
			"committed catalog generation",
			strings.TrimSpace(*generationStore),
			err,
		)
	}
	semanticChecksum, err := generationSemanticChecksum(generation)
	if err != nil {
		return err
	}
	bundle, err := artifact.Build(generation)
	if err != nil {
		return err
	}
	assets, err := artifact.StageReleaseAssets(*outputDir, bundle)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(releaseReport{
		GenerationID:     generation.Manifest.GenerationID,
		SemanticChecksum: semanticChecksum,
		PayloadChecksum:  generation.Manifest.Payload.Checksum,
		ArchiveChecksum:  assets.ArchiveChecksum,
		Directory:        assets.Directory, Files: assets.Files,
	})
}

func verifyReleaseDirectory(directory string) (releaseReport, error) {
	release, err := readReleaseDirectory(directory)
	if err != nil {
		return releaseReport{}, err
	}
	generation, err := artifact.Open(release.archive, release.statement)
	if err != nil {
		return releaseReport{}, pkgerrors.WrapResource("verify", "catalog release", release.directory, err)
	}
	semanticChecksum, err := generationSemanticChecksum(generation)
	if err != nil {
		return releaseReport{}, err
	}
	return releaseReport{
		GenerationID:     generation.Manifest.GenerationID,
		SemanticChecksum: semanticChecksum,
		PayloadChecksum:  generation.Manifest.Payload.Checksum,
		ArchiveChecksum:  release.archiveChecksum,
		Directory:        release.directory,
		Files:            release.files,
	}, nil
}

func inspectReleaseDirectory(directory string) (inspectionReport, error) {
	release, err := readReleaseDirectory(directory)
	if err != nil {
		return inspectionReport{}, err
	}
	descriptor, err := artifact.Inspect(release.archive, release.statement)
	if err != nil {
		return inspectionReport{}, pkgerrors.WrapResource("inspect", "catalog release", release.directory, err)
	}
	return inspectionReport{
		GenerationID:          descriptor.GenerationID,
		ManifestVersion:       descriptor.ManifestVersion,
		SchemaVersion:         descriptor.SchemaVersion,
		ConsumerCompatibility: descriptor.ConsumerCompatibility,
		CurrentSchemaVersion:  catalogs.CurrentCatalogSchemaVersion,
		SupportsCurrentSchema: descriptor.ConsumerCompatibility.SupportsSchema(catalogs.CurrentCatalogSchemaVersion),
		ArchiveChecksum:       release.archiveChecksum,
		Directory:             release.directory,
		Files:                 release.files,
	}, nil
}

type releaseDirectory struct {
	directory       string
	archive         []byte
	statement       []byte
	archiveChecksum string
	files           []string
}

func readReleaseDirectory(directory string) (releaseDirectory, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return releaseDirectory{}, pkgerrors.WrapIO("resolve", directory, err)
	}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return releaseDirectory{}, pkgerrors.WrapIO("read", absolute, err)
	}
	if len(entries) != 3 {
		return releaseDirectory{}, &pkgerrors.ValidationError{
			Field: "catalog_release.files", Value: len(entries), Message: "release directory must contain exactly three assets",
		}
	}

	files := []string{
		filepath.Join(absolute, artifact.Filename),
		filepath.Join(absolute, artifact.AttestationFilename),
		filepath.Join(absolute, artifact.ChecksumFilename),
	}
	archive, err := readReleaseAsset(files[0])
	if err != nil {
		return releaseDirectory{}, err
	}
	statement, err := readReleaseAsset(files[1])
	if err != nil {
		return releaseDirectory{}, err
	}
	checksumFile, err := readReleaseAsset(files[2])
	if err != nil {
		return releaseDirectory{}, err
	}

	digest := sha256.Sum256(archive)
	digestHex := fmt.Sprintf("%x", digest)
	wantChecksumFile := digestHex + "  " + artifact.Filename + "\n"
	if string(checksumFile) != wantChecksumFile {
		return releaseDirectory{}, &pkgerrors.ValidationError{
			Field: "catalog_release.checksum", Value: strings.TrimSpace(string(checksumFile)), Message: "does not match archive bytes",
		}
	}
	return releaseDirectory{
		directory: absolute, archive: archive, statement: statement,
		archiveChecksum: "sha256:" + digestHex, files: files,
	}, nil
}

func generationSemanticChecksum(generation catalogs.Generation) (string, error) {
	catalog, err := catalogs.DecodeCatalogPayload(generation.Payload)
	if err != nil {
		return "", pkgerrors.WrapResource(
			"decode",
			"catalog release semantics",
			generation.Manifest.GenerationID,
			err,
		)
	}
	checksum, err := catalogs.CatalogSemanticChecksum(catalog)
	if err != nil {
		return "", pkgerrors.WrapResource(
			"encode",
			"catalog release semantics",
			generation.Manifest.GenerationID,
			err,
		)
	}
	return checksum, nil
}

func readReleaseAsset(path string) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the caller selects a release directory and filenames are fixed.
	if err != nil {
		return nil, pkgerrors.WrapIO("read", path, err)
	}
	return data, nil
}
