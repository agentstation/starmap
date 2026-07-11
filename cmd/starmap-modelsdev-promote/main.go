// Command starmap-modelsdev-promote validates and atomically promotes one
// downloaded models.dev payload for the embedded catalog generation workflow.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/pkg/errors"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("starmap-modelsdev-promote", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	input := flags.String("input", "", "downloaded models.dev api.json candidate")
	destination := flags.String("output", "", "embedded models.dev api.json destination")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return &errors.ValidationError{Field: "arguments", Value: flags.Args(), Message: "positional arguments are not supported"}
	}
	promotion, err := modelsdev.PromoteAPIFile(*input, *destination)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(promotion)
}
