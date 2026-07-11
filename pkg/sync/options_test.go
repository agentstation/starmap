package sync

import (
	"context"
	stderrors "errors"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestOptionsValidateRejectsUnknownSource(t *testing.T) {
	opts := Defaults().Apply(WithSources("unknown"))

	err := opts.Validate(catalogs.NewProviders())
	if err == nil {
		t.Fatal("Validate accepted an unknown source")
	}
	var validationErr *errors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("Validate error = %T, want *errors.ValidationError", err)
	}
	if validationErr.Field != "Sources" {
		t.Fatalf("Validation field = %q, want Sources", validationErr.Field)
	}
}

func TestOptionsValidateRejectsConcurrentModelsDevTransports(t *testing.T) {
	opts := Defaults().Apply(WithSources(sources.ModelsDevHTTPID, sources.ModelsDevGitID))
	err := opts.Validate(catalogs.NewProviders())
	if err == nil {
		t.Fatal("Validate accepted simultaneous models.dev HTTP and Git transports")
	}
	var validationErr *errors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("Validate error = %T, want *errors.ValidationError", err)
	}
	if validationErr.Field != "Sources" {
		t.Fatalf("Validation field = %q, want Sources", validationErr.Field)
	}
}

func TestOptionsValidateRequiresPinnedModelsDevGitCommit(t *testing.T) {
	validCommit := strings.Repeat("a", 40)
	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{name: "missing pin", opts: []Option{WithSources(sources.ModelsDevGitID)}, wantErr: true},
		{name: "floating branch", opts: []Option{WithSources(sources.ModelsDevGitID), WithModelsDevGitCommit("dev")}, wantErr: true},
		{name: "exact commit", opts: []Option{WithSources(sources.ModelsDevGitID), WithModelsDevGitCommit(validCommit)}},
		{name: "ignored pin", opts: []Option{WithSources(sources.ModelsDevHTTPID), WithModelsDevGitCommit(validCommit)}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Defaults().Apply(test.opts...).Validate(catalogs.NewProviders())
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestWithSourcesCopiesSelection(t *testing.T) {
	selected := []sources.ID{sources.ProvidersID}
	opts := Defaults().Apply(WithSources(selected...))
	selected[0] = sources.ModelsDevHTTPID

	if got := opts.Sources[0]; got != sources.ProvidersID {
		t.Fatalf("Configured source changed through caller slice: got %q", got)
	}
}

func TestOptionsValidateRejectsConflictingDependencyPolicies(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{
			name: "auto install and skip",
			opts: []Option{WithAutoInstallDeps(true), WithSkipDepPrompts(true)},
		},
		{
			name: "interactive and auto install",
			opts: []Option{
				WithAutoInstallDeps(true),
				WithDependencyDecisionHandler(func(context.Context, sources.ID, sources.Dependency, bool) (DependencyDecision, error) {
					return DependencyDecisionInstall, nil
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Defaults().Apply(tt.opts...).Validate(catalogs.NewProviders())
			if err == nil {
				t.Fatal("Validate accepted conflicting dependency policies")
			}
			var validationErr *errors.ValidationError
			if !stderrors.As(err, &validationErr) {
				t.Fatalf("Validate error = %T, want *errors.ValidationError", err)
			}
			if validationErr.Field != "DependencyPolicy" {
				t.Fatalf("Validation field = %q, want DependencyPolicy", validationErr.Field)
			}
		})
	}
}
