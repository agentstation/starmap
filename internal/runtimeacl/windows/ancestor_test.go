package windows

import "testing"

func TestWindowsAncestorGrantPolicy(t *testing.T) {
	const account = "S-1-5-21-1-2-3-1001"
	const installer = "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
	for _, test := range []struct {
		name, owner string
		rights      uint32
		flags       uint8
		invalid     bool
	}{
		{name: "read and traverse", rights: 0x1200a9},
		{name: "generic read execute", rights: 0xa0000000},
		{name: "create subdirectory", rights: 0x4},
		{name: "inherit-only grant", rights: 0x1f01ff, flags: 8},
		{name: "system owner", owner: windowsSystemSID},
		{name: "administrator owner", owner: windowsAdministratorsSID},
		{name: "installer owner", owner: installer},
		{name: "inherited read", rights: 0x1200a9, flags: 16},
		{name: "create file or set reparse", rights: 0x2, invalid: true},
		{name: "write attributes or set reparse", rights: 0x100, invalid: true},
		{name: "write extended attributes", rights: 0x10, invalid: true},
		{name: "delete child", rights: 0x40, invalid: true},
		{name: "delete ancestor", rights: 0x10000, invalid: true},
		{name: "change DACL", rights: 0x40000, invalid: true},
		{name: "change owner", rights: 0x80000, invalid: true},
		{name: "generic write", rights: 0x40000000, invalid: true},
		{name: "generic all", rights: 0x10000000, invalid: true},
		{name: "unknown right", rights: 0x02000000, invalid: true},
		{name: "unknown flag", flags: 0x20, invalid: true},
		{name: "inherited mutation", rights: 0x40, flags: 16, invalid: true},
		{name: "foreign owner", owner: "S-1-5-21-1-2-3-1002", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			owner := test.owner
			if owner == "" {
				owner = account
			}
			entries := []Entry{{Kind: windowsACLAllow, Principal: "S-1-1-0", Rights: test.rights, Flags: test.flags}}
			err := ValidateAncestor(account, owner, true, false, entries, "private.ancestor")
			if (err != nil) != test.invalid {
				t.Fatalf("error = %v, want invalid=%v", err, test.invalid)
			}
		})
	}
	for _, test := range []struct {
		name          string
		present, null bool
		entries       []Entry
		invalid       bool
	}{
		{name: "absent", invalid: true},
		{name: "null", present: true, null: true, invalid: true},
		{name: "empty", present: true},
		{name: "unsupported callback", present: true, entries: []Entry{{Kind: 9, Principal: account}}, invalid: true},
		{name: "deny", present: true, entries: []Entry{{Kind: windowsACLDeny, Principal: "S-1-1-0", Rights: 0x1f01ff}}},
		{name: "deny cannot excuse allow", present: true, entries: []Entry{{Kind: windowsACLDeny, Principal: "S-1-1-0", Rights: 0x40}, {Kind: windowsACLAllow, Principal: "S-1-1-0", Rights: 0x40}}, invalid: true},
		{name: "owner mutation", present: true, entries: []Entry{{Kind: windowsACLAllow, Principal: account, Rights: 0x1f01ff}}},
		{name: "host administrators", present: true, entries: []Entry{{Kind: windowsACLAllow, Principal: windowsSystemSID, Rights: 0x1f01ff}, {Kind: windowsACLAllow, Principal: windowsAdministratorsSID, Rights: 0x1f01ff}, {Kind: windowsACLAllow, Principal: installer, Rights: 0x1f01ff}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateAncestor(account, account, test.present, test.null, test.entries, "private.ancestor")
			if (err != nil) != test.invalid {
				t.Fatalf("error = %v, want invalid=%v", err, test.invalid)
			}
		})
	}
}
