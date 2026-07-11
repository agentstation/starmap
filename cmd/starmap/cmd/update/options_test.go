package update

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/pkg/sync"
)

func TestBuildUpdateOptionsMapsSourceFlag(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []sources.ID
	}{
		{
			name:   "provider api",
			source: "provider-api",
			want:   []sources.ID{sources.ProvidersID},
		},
		{
			name:   "models.dev http only",
			source: "models.dev",
			want:   []sources.ID{sources.ModelsDevHTTPID},
		},
		{
			name:   "models.dev git only",
			source: "models.dev-git",
			want:   []sources.ID{sources.ModelsDevGitID},
		},
		{
			name:   "all leaves sources unset",
			source: "all",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := BuildUpdateOptions("", tt.source, "", false, false, false, false, "", "", false, false, false)
			if err != nil {
				t.Fatalf("BuildUpdateOptions returned error: %v", err)
			}

			options := sync.Defaults().Apply(opts...)
			if !slices.Equal(options.Sources, tt.want) {
				t.Fatalf("sources = %#v, want %#v", options.Sources, tt.want)
			}
		})
	}
}

func TestPromptForMissingDependencyIsCLIAdapter(t *testing.T) {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("Create prompt pipe: %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = readEnd
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = readEnd.Close()
		_ = writeEnd.Close()
	})
	if _, err := writeEnd.WriteString("n\n"); err != nil {
		t.Fatalf("Write prompt response: %v", err)
	}

	decision, err := promptForMissingDependency(
		context.Background(),
		sources.ModelsDevGitID,
		sources.Dependency{Name: "bun", DisplayName: "Bun"},
		true,
	)
	if err != nil {
		t.Fatalf("Prompt adapter: %v", err)
	}
	if decision != sync.DependencyDecisionSkip {
		t.Fatalf("Prompt decision = %v, want skip", decision)
	}
}

func TestBuildUpdateOptionsRejectsUnknownSourceFlag(t *testing.T) {
	_, err := BuildUpdateOptions("", "unknown", "", false, false, false, false, "", "", false, false, false)
	if err == nil {
		t.Fatal("BuildUpdateOptions returned nil error")
	}
}

func TestBuildUpdateOptionsOwnsInteractiveDependencyDecision(t *testing.T) {
	tests := []struct {
		name              string
		autoInstall       bool
		skipPrompts       bool
		wantDecisionOwner bool
	}{
		{name: "interactive CLI", wantDecisionOwner: true},
		{name: "automatic install", autoInstall: true},
		{name: "explicit skip", skipPrompts: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := BuildUpdateOptions(
				"", "", "", false, false, false, false, "", "",
				tt.autoInstall, tt.skipPrompts, false,
			)
			if err != nil {
				t.Fatalf("BuildUpdateOptions: %v", err)
			}
			configured := sync.Defaults().Apply(opts...)
			gotDecisionOwner := configured.DependencyDecisionHandler != nil
			if gotDecisionOwner != tt.wantDecisionOwner {
				t.Fatalf("Dependency decision owner configured = %t, want %t", gotDecisionOwner, tt.wantDecisionOwner)
			}
		})
	}
}
