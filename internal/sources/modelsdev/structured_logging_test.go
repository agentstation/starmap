package modelsdev

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/logging"
)

func TestHTTPClientUsesContextStructuredLoggingWithoutDirectOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(largeMockAPIJSON()))
	}))
	defer server.Close()
	client := &HTTPClient{CacheDir: filepath.Join(t.TempDir(), "models.dev"), APIURL: server.URL, Client: server.Client()}
	testLogger := logging.NewTestLogger(t)
	ctx := logging.WithLogger(context.Background(), testLogger.Logger)
	ctx = logging.WithRunID(ctx, "source-run-123")

	stdout, stderr := captureProcessOutput(t, func() {
		if _, err := client.AcquireAPI(ctx); err != nil {
			t.Fatalf("AcquireAPI: %v", err)
		}
	})
	if stdout != "" || stderr != "" {
		t.Fatalf("direct process output = stdout %q stderr %q", stdout, stderr)
	}
	if len(testLogger.Lines()) == 0 {
		t.Fatal("structured logger captured no source events")
	}
	for _, line := range testLogger.Lines() {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("structured log is not JSON: %v: %s", err, line)
		}
		if event["source"] != "models_dev_http" || event["run_id"] != "source-run-123" {
			t.Fatalf("log fields = %#v, want source and run_id", event)
		}
	}
}

func TestSourceAndProviderLibrariesContainNoDirectProcessWrites(t *testing.T) {
	directories := []string{"..", "../../providers", "../../../pkg/sources"}
	for _, directory := range directories {
		err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				packageName, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				if isDirectOutputSelector(packageName.Name, selector.Sel.Name) {
					t.Errorf("direct process output call remains in %s: %s.%s", path, packageName.Name, selector.Sel.Name)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("WalkDir(%s): %v", directory, err)
		}
	}
}

func isDirectOutputSelector(packageName, selector string) bool {
	switch packageName {
	case "fmt":
		return strings.HasPrefix(selector, "Print") || strings.HasPrefix(selector, "Fprint")
	case "log":
		return strings.HasPrefix(selector, "Print")
	case "os":
		return selector == "Stdout" || selector == "Stderr"
	default:
		return false
	}
}

func captureProcessOutput(t *testing.T, run func()) (string, string) {
	t.Helper()
	oldStdout, oldStderr := os.Stdout, os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	defer func() {
		os.Stdout, os.Stderr = oldStdout, oldStderr
	}()

	run()
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	stdout, stdoutErr := io.ReadAll(stdoutReader)
	stderr, stderrErr := io.ReadAll(stderrReader)
	_ = stdoutReader.Close()
	_ = stderrReader.Close()
	if stdoutErr != nil || stderrErr != nil {
		t.Fatalf("read captured output: stdout=%v stderr=%v", stdoutErr, stderrErr)
	}
	return string(stdout), string(stderr)
}
