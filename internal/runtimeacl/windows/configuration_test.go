package windows

import "testing"

func TestConfigurationTrustedOwnershipAndReadOnlyGrants(t *testing.T) {
	const account = "S-1-5-21-1-2-3-1001"
	for _, owner := range []string{account, windowsSystemSID, windowsAdministratorsSID, windowsInstallerSID, "S-1-1-0"} {
		for _, right := range []uint32{configurationReadRights, 0x2, 0x4, 0x10000, 0x40000, 0x80000, 0x40000000} {
			descriptor := Descriptor{Owner: owner, Present: true, Entries: []Entry{{Kind: windowsACLAllow, Principal: "S-1-1-0", Rights: right}}}
			allowed := owner != "S-1-1-0" && right == configurationReadRights
			if err := ValidateConfiguration(account, descriptor, "test"); (err == nil) != allowed {
				t.Errorf("owner=%s rights=%x error=%v", owner, right, err)
			}
		}
	}
	for _, descriptor := range []Descriptor{
		{Owner: account}, {Owner: account, Present: true, Null: true},
		{Owner: account, Present: true, Entries: []Entry{{Kind: 5}}},
		{Owner: account, Present: true, Entries: []Entry{{Kind: windowsACLAllow, Flags: 0x80}}},
	} {
		if err := ValidateConfiguration(account, descriptor, "test"); err == nil {
			t.Errorf("accepted unsupported descriptor: %+v", descriptor)
		}
	}
}
