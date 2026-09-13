// Package filemutation verifies filesystem mutation attempts in contract tests.
package filemutation

import (
	"os"
	"testing"
)

// Rename moves a test entry or verifies that the native filesystem prevents the move.
// A false result requires unchanged source identity, metadata, and an absent destination.
// Unexpected failures stop the test.
func Rename(t *testing.T, source, destination string) bool {
	t.Helper()
	before, err := os.Lstat(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("mutation destination must be absent: %v", err)
	}
	err = os.Rename(source, destination)
	if err == nil {
		return true
	}
	if !nativeRenameRefusal(err) {
		t.Fatalf("rename mutation failed: %v", err)
	}
	after, statErr := os.Lstat(source)
	if statErr != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("refused rename changed its source: rename=%v, stat=%v", err, statErr)
	}
	if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("refused rename created its destination: %v", statErr)
	}
	t.Logf("native filesystem prevented replacement: %v", err)
	return false
}
