package privatefiles

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"testing"
)

func TestDarwinAncestorACLSeparatesReadAndMutationGrants(t *testing.T) {
	owner, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		entry string
		deny  bool
	}{
		{"everyone allow read,readattr,readextattr,readsecurity,search", false},
		{"everyone deny delete", false},
		{"user:" + owner.Username + " allow add_file,add_subdirectory,delete_child", false},
		{"everyone allow add_file", true},
		{"everyone allow delete_child", true},
		{"everyone allow writesecurity", true},
		{"everyone allow add_file,directory_inherit,only_inherit", true},
	} {
		t.Run(test.entry, func(t *testing.T) {
			parent := t.TempDir()
			if output, err := exec.Command("/bin/chmod", "+a", test.entry, parent).CombinedOutput(); err != nil {
				t.Fatalf("set ancestor ACL: %v: %s", err, output)
			}
			t.Cleanup(func() {
				if output, err := exec.Command("/bin/chmod", "-N", parent).CombinedOutput(); err != nil {
					t.Errorf("remove ancestor ACL: %v: %s", err, output)
				}
			})
			before, err := os.Stat(parent)
			if err != nil {
				t.Fatal(err)
			}
			err = ValidateAncestors(filepath.Join(parent, "missing"))
			if (err != nil) != test.deny {
				t.Fatalf("ancestor ACL error = %v, want refusal = %v", err, test.deny)
			}
			after, err := os.Stat(parent)
			if err != nil || before.Mode() != after.Mode() {
				t.Fatal("ancestor inspection changed mode bits", err)
			}
			if _, err := os.Stat(filepath.Join(parent, "missing")); !os.IsNotExist(err) {
				t.Fatal("ancestor inspection created a path")
			}
		})
	}
}
