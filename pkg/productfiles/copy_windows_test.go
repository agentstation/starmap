package productfiles_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestPrivateFileCopyLocksDirectoryUntilComplete(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	directory := newDirectory(t, root)
	body := bytes.Repeat([]byte("x"), 128<<10)
	if err := directory.CompareAndPublish(t.Context(), "record", nil, body); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	var renameErr error
	writes := 0
	n, err := directory.CopyFile(t.Context(), "record", privateCopyWriter(func(p []byte) (int, error) {
		writes++
		if writes == 1 {
			renameErr = os.Rename(root, root+"-old")
		}
		return output.Write(p)
	}), int64(len(body)))
	if !errors.Is(renameErr, windows.ERROR_SHARING_VIOLATION) {
		t.Fatalf("open directory rename: %v", renameErr)
	}
	if err != nil || n != int64(len(body)) || !bytes.Equal(output.Bytes(), body) {
		t.Fatalf("protected copy: bytes=%d err=%v", n, err)
	}
	if err := os.Rename(root, root+"-old"); err != nil {
		t.Fatal(err)
	}
	replacement := newDirectory(t, root)
	if err := replacement.CompareAndPublish(t.Context(), "record", nil, body); err != nil {
		t.Fatal(err)
	}
	if n, err := directory.CopyFile(t.Context(), "record", io.Discard, int64(len(body))); err == nil || n != 0 {
		t.Fatalf("replaced directory: bytes=%d err=%v", n, err)
	}
}
