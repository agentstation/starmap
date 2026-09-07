package filepublish

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSyncDirectoryWindowsRequiresWritableHandle(t *testing.T) {
	root := publicationRoot(t)
	readOnly, err := root.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = readOnly.Close() }()
	if err := windows.FlushFileBuffers(windows.Handle(readOnly.Fd())); !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("read-only directory flush = %v; want access denied", err)
	}
	if err := SyncDirectory(root); err != nil {
		t.Fatal(err)
	}
}
