// Command starmap-embedded-budget emits the versioned embedded catalog
// release-policy report and enforces hard correctness gates.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/internal/bootstrap/budget"
	"github.com/agentstation/starmap/pkg/errors"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, time.Now().UTC()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer, now time.Time) error {
	if len(args) != 0 {
		return &errors.ValidationError{Field: "arguments", Value: args, Message: "positional arguments are not supported"}
	}
	generation, err := bootstrap.Generation()
	if err != nil {
		return err
	}
	report, checkErr := budget.Check(generation, now, budget.DefaultPolicy())
	if err := json.NewEncoder(output).Encode(report); err != nil {
		return errors.WrapIO("write", "embedded catalog budget report", err)
	}
	return checkErr
}
