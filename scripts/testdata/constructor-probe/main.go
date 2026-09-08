// Command constructor-probe checks passive construction in a restricted process.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 || runtime.GOOS != "linux" {
		return &errors.ValidationError{Field: "probe mode", Message: "select read-only or network-attempt on Linux"}
	}
	if os.Args[1] == "network-attempt" {
		if err := json.NewEncoder(os.Stdout).Encode(map[string]string{"phase": "network-attempt"}); err != nil {
			return err
		}
		connection, err := net.DialTimeout("tcp", "192.0.2.1:9", time.Second)
		if connection != nil {
			_ = connection.Close()
		}
		return &errors.ValidationError{Field: "network control", Message: fmt.Sprintf("socket attempt returned instead of terminating the process: %v", err)}
	}
	if os.Args[1] != "read-only" {
		return &errors.ValidationError{Field: "probe mode", Message: "unknown mode"}
	}
	store, err := storage.NewFilesystem(filepath.Join("/work", "absent-store"))
	if err != nil {
		return err
	}
	constructors := []struct {
		name string
		open func() (*starmap.Client, error)
	}{
		{"New", func() (*starmap.Client, error) { return starmap.New() }},
		{"NewContext", func() (*starmap.Client, error) { return starmap.NewContext(context.Background()) }},
		{"NewContextMemory", func() (*starmap.Client, error) {
			return starmap.NewContext(context.Background(), starmap.WithCatalogStore(storage.NewMemory()))
		}},
		{"NewContextFilesystem", func() (*starmap.Client, error) {
			return starmap.NewContext(context.Background(), starmap.WithCatalogStore(store))
		}},
	}
	var names []string
	var baseline starmap.CatalogState
	for _, constructor := range constructors {
		client, err := constructor.open()
		if err != nil {
			return errors.WrapResource("construct", "catalog client", constructor.name, err)
		}
		current := client.CurrentCatalogState()
		if current.Catalog == nil || current.GenerationID == "" || current.PayloadChecksum == "" || current.Catalog.Providers().Len() == 0 {
			return &errors.ValidationError{Field: constructor.name, Message: "returned an incomplete catalog"}
		}
		if baseline.Catalog != nil && (current.GenerationID != baseline.GenerationID || current.PayloadChecksum != baseline.PayloadChecksum) {
			return &errors.ValidationError{Field: constructor.name, Message: "changed the embedded baseline"}
		}
		baseline = current
		names = append(names, constructor.name)
	}
	entries, err := os.ReadDir("/work")
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return &errors.ValidationError{Field: "home files", Message: "constructors created files"}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"constructors": names, "generation_id": baseline.GenerationID,
		"payload_checksum": baseline.PayloadChecksum, "files_after": len(entries),
		"os": runtime.GOOS, "arch": runtime.GOARCH,
	})
}
