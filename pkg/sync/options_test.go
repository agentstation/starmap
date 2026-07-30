package sync

import (
	"context"
	stderrors "errors"
	"path/filepath"
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

func TestOptionsValidateRejectsSourceStateOverlappingHumanWorkspace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	exactCommit := strings.Repeat("a", 40)

	tests := []struct {
		name string
		opts []Option
	}{
		{
			name: "default HTTP cache contains workspace",
			opts: []Option{
				WithCatalogPath(filepath.Join(home, ".starmap", "cache", "human")),
			},
		},
		{
			name: "default Git checkout contains workspace",
			opts: []Option{
				WithCatalogPath(filepath.Join(home, ".starmap", "sources", "human")),
				WithSources(sources.ModelsDevGitID),
				WithModelsDevGitCommit(exactCommit),
			},
		},
		{
			name: "explicit source state equals workspace",
			opts: []Option{
				WithCatalogPath(filepath.Join(home, "catalog")),
				WithSourcesDir(filepath.Join(home, "catalog")),
			},
		},
		{
			name: "explicit source state beneath workspace",
			opts: []Option{
				WithCatalogPath(filepath.Join(home, "catalog")),
				WithSourcesDir(filepath.Join(home, "catalog", "cache")),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Defaults().Apply(test.opts...).Validate(catalogs.NewProviders())
			var configErr *errors.ConfigError
			if !stderrors.As(err, &configErr) {
				t.Fatalf("Validate() error = %T %v, want *errors.ConfigError", err, err)
			}
			if configErr.Component != "catalog filesystem layout" {
				t.Fatalf("component = %q", configErr.Component)
			}
		})
	}
}

func TestOptionsValidateAcceptsSeparatedHumanAndMachineRoots(t *testing.T) {
	root := t.TempDir()
	opts := Defaults().Apply(
		WithCatalogPath(filepath.Join(root, "catalog")),
		WithSourcesDir(filepath.Join(root, "source-state")),
	)
	if err := opts.Validate(catalogs.NewProviders()); err != nil {
		t.Fatalf("Validate separated roots: %v", err)
	}
}

func TestOptionsValidateAcceptsFirstRunWorkspaceWithMissingParent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "new", ".starmap", "catalog")
	opts := Defaults().Apply(
		WithCatalogPath(path),
		WithSources(sources.ProvidersID),
	)
	if err := opts.Validate(catalogs.NewProviders()); err != nil {
		t.Fatalf("Validate first-run workspace: %v", err)
	}
}
