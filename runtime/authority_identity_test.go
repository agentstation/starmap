package runtime

import (
	stderrors "errors"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestSourcePolicyAuthorityIdentityContract(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		identity string
		wantErr  bool
	}{
		{"ordinary identity", "enterprise", false},
		{"unicode identity", "équipe", false},
		{"exact byte limit", strings.Repeat("a", 256), false},
		{"empty", "", true},
		{"over byte limit", strings.Repeat("a", 257), true},
		{"unicode over byte limit", strings.Repeat("é", 129), true},
		{"leading whitespace", " enterprise", true},
		{"trailing whitespace", "enterprise\t", true},
		{"interior control", "enter\x00prise", true},
		{"invalid UTF-8", "enter\xffprise", true},
	} {
		for _, field := range []string{"authority", "policy"} {
			t.Run(scenario.name+"/"+field, func(t *testing.T) {
				policy := DefaultSourcePolicy()
				policy.Kind, policy.URL = SourceStarmap, "https://authority.example"
				policy.StartupPolicy = StartupRequireAuthority
				policy.AuthorityID, policy.PolicyID = "enterprise", "production"
				if field == "authority" {
					policy.AuthorityID = scenario.identity
				} else {
					policy.PolicyID = scenario.identity
				}
				err := policy.Validate()
				if !scenario.wantErr {
					if err != nil {
						t.Fatal(err)
					}
					return
				}
				var invalid *errors.ValidationError
				if !stderrors.As(err, &invalid) || invalid.Field != "source_policy.authority" {
					t.Fatalf("error=%v", err)
				}
			})
		}
	}
}
