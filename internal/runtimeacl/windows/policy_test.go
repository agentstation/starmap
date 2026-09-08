package windows

import (
	"errors"
	"testing"

	starmaperrors "github.com/agentstation/starmap/pkg/errors"
)

func TestWindowsACLGrantPolicy(t *testing.T) {
	const account = "S-1-5-21-1-2-3-1001"
	for _, test := range []struct {
		name         string
		owner        string
		absent, null bool
		entries      []Entry
		invalid      bool
	}{
		{name: "owner", owner: account, entries: []Entry{{Kind: windowsACLAllow, Principal: account, Rights: 1}}},
		{name: "host administrators", owner: account, entries: []Entry{{Kind: windowsACLAllow, Principal: windowsSystemSID, Rights: 1}, {Kind: windowsACLAllow, Principal: windowsAdministratorsSID, Rights: 1}}},
		{name: "deny only", owner: account, entries: []Entry{{Kind: windowsACLDeny, Principal: "S-1-1-0", Rights: 1}}},
		{name: "empty restrictive ACL", owner: account},
		{name: "no rights", owner: account, entries: []Entry{{Kind: windowsACLAllow, Principal: "S-1-1-0"}}},
		{name: "everyone", owner: account, entries: []Entry{{Kind: windowsACLAllow, Principal: "S-1-1-0", Rights: 1}}, invalid: true},
		{name: "authenticated users", owner: account, entries: []Entry{{Kind: windowsACLAllow, Principal: "S-1-5-11", Rights: 1}}, invalid: true},
		{name: "another account", owner: account, entries: []Entry{{Kind: windowsACLAllow, Principal: "S-1-5-21-1-2-3-1002", Rights: 1}}, invalid: true},
		{name: "deny does not excuse a grant", owner: account, entries: []Entry{{Kind: windowsACLDeny, Principal: "S-1-1-0", Rights: 1}, {Kind: windowsACLAllow, Principal: "S-1-1-0", Rights: 1}}, invalid: true},
		{name: "missing owner", invalid: true},
		{name: "wrong owner", owner: windowsAdministratorsSID, invalid: true},
		{name: "absent DACL", owner: account, absent: true, invalid: true},
		{name: "null DACL", owner: account, null: true, invalid: true},
		{name: "unsupported callback", owner: account, entries: []Entry{{Kind: 9, Principal: account, Rights: 1}}, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := Validate(account, test.owner, !test.absent, test.null, test.entries, "runtime.directory")
			if (err != nil) != test.invalid {
				t.Fatalf("error = %v, invalid = %v", err, test.invalid)
			}
			if err != nil {
				var invalid *starmaperrors.ValidationError
				if !errors.As(err, &invalid) || invalid.Field != "runtime.directory" {
					t.Fatalf("wrong diagnostic: %v", err)
				}
			}
		})
	}
}
