// Command starmap-bootstrap-manifest atomically refreshes embedded generation
// metadata only when canonical catalog bytes changed.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/agentstation/starmap/internal/bootstrapmanifest"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
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
	catalog, err := builder.Build()
	if err != nil {
		return err
	}
	current, err := readCurrentManifest(*manifestPath)
	if err != nil {
		return err
	}
	manifest, report, err := bootstrapmanifest.Derive(catalog, current, now)
	if err != nil {
		return err
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
	manifest, err := catalogs.ParseBootstrapManifestJSON(data)
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
