package config_test

import (
	"github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/sources"
	"strings"
	"testing"
)

func TestCanonicalGitAcquisitionCommitSetting(t *testing.T) {
	for _, test := range []struct {
		name, value string
		valid       bool
	}{
		{"sha1", strings.Repeat("a", 40), true},
		{"sha256", strings.Repeat("B", 64), true},
		{"clear", "", true},
		{"short", "abcdef", false},
		{"branch", "main", false},
		{"invalid_hex", strings.Repeat("g", 40), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := config.Parse(map[string]string{config.AcquisitionSources: string(sources.ModelsDevGitID), config.ModelsDevGitCommit: test.value})
			if (err == nil) != test.valid {
				t.Fatalf("valid=%t: %v", test.valid, err)
			}
			if err != nil {
				return
			}
			if value, present := parsed.Value(config.ModelsDevGitCommit); !present || value != test.value {
				t.Fatal("lost configured Git commit")
			}
		})
	}
}
