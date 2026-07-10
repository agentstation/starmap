// Command starmap-embedded-budget emits and enforces checked-in catalog
// freshness, size, and coverage measurements for CI.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/internal/embeddedbudget"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	envMaxAge          = "STARMAP_EMBEDDED_BUDGET_MAX_AGE"
	envMaxUncompressed = "STARMAP_EMBEDDED_BUDGET_MAX_UNCOMPRESSED_BYTES"
	envMaxCompressed   = "STARMAP_EMBEDDED_BUDGET_MAX_COMPRESSED_BYTES"
	envMinProviders    = "STARMAP_EMBEDDED_BUDGET_MIN_PROVIDERS"
	envMinModels       = "STARMAP_EMBEDDED_BUDGET_MIN_MODELS"
	envOverrideReason  = "STARMAP_EMBEDDED_BUDGET_OVERRIDE_REASON"
)

type getenvFunc func(string) string

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Getenv, time.Now().UTC()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer, getenv getenvFunc, now time.Time) error {
	if len(args) != 0 {
		return &errors.ValidationError{Field: "arguments", Value: args, Message: "positional arguments are not supported"}
	}
	limits, reason, err := limitsFromEnvironment(getenv)
	if err != nil {
		return err
	}
	generation, err := bootstrap.Generation()
	if err != nil {
		return err
	}
	report, checkErr := embeddedbudget.Check(generation, now, limits, reason)
	if err := json.NewEncoder(output).Encode(report); err != nil {
		return errors.WrapIO("write", "embedded catalog budget report", err)
	}
	return checkErr
}

func limitsFromEnvironment(getenv getenvFunc) (embeddedbudget.Limits, string, error) {
	limits := embeddedbudget.DefaultLimits()
	overridden := false
	if value := strings.TrimSpace(getenv(envMaxAge)); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return embeddedbudget.Limits{}, "", &errors.ValidationError{Field: envMaxAge, Value: value, Message: err.Error()}
		}
		limits.MaxAge, overridden = parsed, true
	}
	integerOverrides := []struct {
		name string
		set  func(int64)
	}{
		{name: envMaxUncompressed, set: func(value int64) { limits.MaxUncompressedBytes = value }},
		{name: envMaxCompressed, set: func(value int64) { limits.MaxCompressedBytes = value }},
		{name: envMinProviders, set: func(value int64) { limits.MinProviders = int(value) }},
		{name: envMinModels, set: func(value int64) { limits.MinModels = int(value) }},
	}
	for _, override := range integerOverrides {
		value := strings.TrimSpace(getenv(override.name))
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed <= 0 {
			return embeddedbudget.Limits{}, "", &errors.ValidationError{Field: override.name, Value: value, Message: "must be a positive base-10 integer"}
		}
		override.set(parsed)
		overridden = true
	}
	if err := limits.Validate(); err != nil {
		return embeddedbudget.Limits{}, "", err
	}
	reason := strings.TrimSpace(getenv(envOverrideReason))
	if overridden && reason == "" {
		return embeddedbudget.Limits{}, "", &errors.ValidationError{Field: envOverrideReason, Message: "is required whenever a checked-in threshold is overridden"}
	}
	return limits, reason, nil
}
