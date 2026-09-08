package windows

import "testing"

func TestDescriptorAssessmentUsesPrivatePolicy(t *testing.T) {
	const owner = "S-1-5-21-1-2-3-1001"
	for _, test := range []struct {
		name, dacl, status string
		descriptor         Descriptor
	}{
		{"private", "present", "compatible", Descriptor{Owner: owner, Present: true, Entries: []Entry{{Principal: owner, Rights: 1}}}},
		{"foreign-grant", "present", "conflict", Descriptor{Owner: owner, Present: true, Entries: []Entry{{Principal: "S-1-1-0", Rights: 1}}}},
		{"foreign-owner", "empty", "conflict", Descriptor{Owner: "S-1-5-18", Present: true}},
		{"absent", "absent", "conflict", Descriptor{Owner: owner}},
		{"null", "null", "conflict", Descriptor{Owner: owner, Present: true, Null: true}},
		{"empty", "empty", "compatible", Descriptor{Owner: owner, Present: true}},
		{"deny-data", "present", "compatible", Descriptor{Owner: owner, Present: true, Entries: []Entry{{Kind: 1, Principal: owner, Rights: 1}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			observation := Assess(owner, test.descriptor)
			if observation.DACLState != test.dacl || observation.Status != test.status || observation.EntryCount != len(test.descriptor.Entries) {
				t.Fatalf("unexpected native observation: %+v", observation)
			}
			if test.status == "compatible" && observation.Reason != "" {
				t.Fatal("compatible policy reported a conflict")
			}
		})
	}
}
