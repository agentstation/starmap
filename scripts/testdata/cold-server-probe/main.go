// Command cold-server-probe observes an offline Starmap server and its saved baseline.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 3 {
		return &errors.ValidationError{Field: "observation", Message: "select http, file, or network and a target"}
	}
	result := map[string]any{}
	switch os.Args[1] {
	case "http":
		client := http.Client{Timeout: 5 * time.Second}
		response, err := client.Get(os.Args[2])
		if err != nil {
			return err
		}
		defer response.Body.Close()
		data, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
		if err != nil {
			return err
		}
		result["status_code"] = response.StatusCode
		result["checksum"] = fmt.Sprintf("sha256:%x", sha256.Sum256(data))
		result["bytes"] = len(data)
		result["generation_header"] = response.Header.Get("X-Starmap-Generation-ID")
		if !strings.HasSuffix(os.Args[2], "/payload") {
			result["body"] = json.RawMessage(data)
		}
		if strings.HasSuffix(os.Args[2], "/manifest") {
			if _, err := catalogs.ParseGenerationManifestJSON(data); err != nil {
				return err
			}
			result["manifest_validated"] = true
		}
	case "file":
		data, err := os.ReadFile(os.Args[2])
		if err != nil {
			return err
		}
		info, err := os.Stat(os.Args[2])
		if err != nil {
			return err
		}
		result["mode"] = fmt.Sprintf("%04o", info.Mode().Perm())
		result["bytes"] = len(data)
		result["checksum"] = fmt.Sprintf("sha256:%x", sha256.Sum256(data))
		if strings.HasSuffix(os.Args[2], "manifest.json") {
			if _, err := catalogs.ParseGenerationManifestJSON(data); err != nil {
				return err
			}
			result["manifest_validated"] = true
			result["body"] = json.RawMessage(data)
		}
	case "network":
		connection, err := net.DialTimeout("tcp", os.Args[2], time.Second)
		if connection != nil {
			_ = connection.Close()
			return &errors.ValidationError{Field: "network control", Message: "unexpected external connection"}
		}
		if err == nil {
			return &errors.ValidationError{Field: "network control", Message: "network control had no error"}
		}
		result["denied"] = true
		result["error"] = err.Error()
	default:
		return &errors.ValidationError{Field: "observation", Message: "unknown observation"}
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
