// Command starmap-bootstrap-manifest atomically refreshes embedded generation
// metadata only when canonical catalog bytes changed.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/agentstation/starmap/internal/bootstrapmanifest"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, time.Now().UTC()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer, now time.Time) error {
	flags := flag.NewFlagSet("starmap-bootstrap-manifest", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	catalogDir := flags.String("catalog-dir", "", "candidate embedded catalog directory")
	manifestPath := flags.String("output", "", "bootstrap generation manifest path")
	endpointsPath := flags.String("endpoints-output", "", "optional generated endpoint projection path")
	generationStorePath := flags.String(
		"generation-store",
		"",
		"optional filesystem store containing the exact committed generation",
	)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return &errors.ValidationError{Field: "arguments", Value: flags.Args(), Message: "positional arguments are not supported"}
	}
	if *catalogDir == "" || *manifestPath == "" {
		return &errors.ValidationError{Field: "bootstrap_manifest.paths", Message: "catalog-dir and output are required"}
	}
	builder, err := catalogs.NewFromPath(*catalogDir)
	if err != nil {
		return err
	}
	if err := builder.LoadReport().Err(); err != nil {
		return err
	}
	catalog, err := builder.Build()
	if err != nil {
		return err
	}
	current, err := readCurrentManifest(*manifestPath)
	if err != nil {
		return err
	}
	var manifest catalogs.BootstrapManifest
	var report bootstrapmanifest.Report
	if *generationStorePath == "" {
		manifest, report, err = bootstrapmanifest.Derive(catalog, current, now)
	} else {
		store, storeErr := catalogstore.NewFilesystem(*generationStorePath)
		if storeErr != nil {
			return storeErr
		}
		generation, currentErr := store.Current(context.Background())
		if currentErr != nil {
			if !errors.IsNotFound(currentErr) {
				return errors.WrapResource(
					"read",
					"committed catalog generation",
					*generationStorePath,
					currentErr,
				)
			}
			manifest, report, err = bootstrapmanifest.Derive(catalog, current, now)
			if err == nil && report.Changed {
				return &errors.ValidationError{
					Field:   "bootstrap_manifest.committed_generation",
					Value:   *generationStorePath,
					Message: "changed catalog has no committed generation",
				}
			}
		} else {
			manifest, report, err = bootstrapmanifest.DeriveCommitted(
				catalog,
				generation,
				current,
			)
		}
	}
	if err != nil {
		return err
	}
	var endpointData []byte
	if *endpointsPath != "" {
		endpointData, err = workspace.EncodeEndpointProjection(catalog, workspace.Identity{
			GenerationID:    manifest.GenerationID,
			PayloadChecksum: manifest.Payload.Checksum,
		})
		if err != nil {
			return err
		}
	}
	if report.Changed {
		data, marshalErr := json.MarshalIndent(manifest, "", "  ")
		if marshalErr != nil {
			return &errors.ValidationError{Field: "bootstrap_manifest", Message: marshalErr.Error()}
		}
		data = append(data, '\n')
		if err := writeAtomic(*manifestPath, data); err != nil {
			return err
		}
	}
	if *endpointsPath != "" {
		currentEndpoints, readErr := os.ReadFile(*endpointsPath) //nolint:gosec // Explicit repository tooling path.
		if readErr != nil && !os.IsNotExist(readErr) {
			return errors.WrapIO("read", *endpointsPath, readErr)
		}
		if !bytes.Equal(currentEndpoints, endpointData) {
			if err := writeAtomic(*endpointsPath, endpointData); err != nil {
				return err
			}
		}
	}
	return json.NewEncoder(output).Encode(report)
}

func readCurrentManifest(path string) (*catalogs.BootstrapManifest, error) {
	data, err := os.ReadFile(path) //nolint:gosec // Explicit repository tooling path.
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.WrapIO("read", path, err)
	}
	manifest, err := catalogs.ParseBootstrapManifestEnvelopeJSON(data)
	if err != nil {
		return nil, err
	}
	return &manifest, nil
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), constants.DirPermissions); err != nil {
		return errors.WrapIO("create", filepath.Dir(path), err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".generation.json.*")
	if err != nil {
		return errors.WrapIO("create", path, err)
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	if err := file.Chmod(constants.FilePermissions); err != nil {
		_ = file.Close()
		return errors.WrapIO("chmod", temporary, err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return errors.WrapIO("write", temporary, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return errors.WrapIO("sync", temporary, err)
	}
	if err := file.Close(); err != nil {
		return errors.WrapIO("close", temporary, err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return errors.WrapIO("promote", path, err)
	}
	return nil
}
