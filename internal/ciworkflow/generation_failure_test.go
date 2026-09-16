package ciworkflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAPIGenerationRejectsPartialOutputAfterToolFailure(t *testing.T) {
	makefile, err := filepath.Abs("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"openapi", "openapi-check"} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			output := filepath.Join(root, "internal", "embedded", "openapi")
			if err := os.MkdirAll(output, 0o700); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"openapi.json", "openapi.yaml"} {
				if err := os.WriteFile(filepath.Join(output, name), []byte("fixture\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			generator := filepath.Join(root, "generator")
			if err := os.WriteFile(generator, []byte(`#!/bin/sh
while [ "$#" -gt 0 ]; do
  if [ "$1" = -o ]; then shift; output="$1"; break; fi
  shift
done
mkdir -p "$output"
printf 'fixture\n' > "$output/swagger.json"
printf 'fixture\n' > "$output/swagger.yaml"
printf 'fixture generator failed after writing output\n'
exit 42
`), 0o700); err != nil {
				t.Fatal(err)
			}
			presence := filepath.Join(root, "presence")
			if err := os.WriteFile(presence, []byte("#!/bin/sh\nprintf 'ran\\n' > presence-ran\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), "make", "-f", makefile, "HAS_DEVBOX=", "SWAG_RUN="+generator, "GOCMD="+presence, target)
			command.Dir = root
			result, err := command.CombinedOutput()
			if err == nil {
				t.Fatalf("%s accepted a failed generator with plausible output:\n%s", target, result)
			}
			if !strings.Contains(string(result), "fixture generator failed after writing output") {
				t.Fatalf("%s lost the generator diagnostic:\n%s", target, result)
			}
			if _, err := os.Stat(filepath.Join(root, "presence-ran")); !os.IsNotExist(err) {
				t.Fatalf("%s continued after generation failed: %v\n%s", target, err, result)
			}
		})
	}
}
