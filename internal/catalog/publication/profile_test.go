package publication

import (
	"strings"
	"testing"
	"time"
)

func TestPublicationProfileRequiresExplicitPolicy(t *testing.T) {
	valid := `policy_version: fixture-v1
scopes:
  - source: embedded_catalog
    required: true
    enabled: true
    allow_missing: false
    max_retained_age: 4h
    disabled_action: preserve
`
	profile, err := ParseProfile([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if !profile.Scopes[0].Required || profile.Scopes[0].MaxRetainedAge != 4*time.Hour {
		t.Fatal("profile changed policy")
	}
	for _, input := range []string{
		strings.Replace(valid, "    required: true\n", "", 1),
		strings.Replace(valid, "    enabled: true\n", "    enabled: true\n    enabled: false\n", 1),
		strings.Replace(valid, "    enabled: true", "    Enabled: true", 1),
		strings.Replace(valid, "    max_retained_age: 4h", "    max_retained_age: -1h", 1),
		valid + "---\npolicy_version: ignored\n",
		valid + "unknown: true\n",
	} {
		if _, err := ParseProfile([]byte(input)); err == nil {
			t.Fatal("invalid profile was accepted", input)
		}
	}
}
