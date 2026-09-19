package filepublish

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestIdentityWindowsMatchesNativeFileID(t *testing.T) {
	if unsafe.Sizeof(windowsFileIDBuffer{}) != nativeFileIDBytes || unsafe.Alignof(windowsFileIDBuffer{}) != 8 {
		t.Fatal("FILE_ID_INFO requires 24 bytes with eight-byte alignment")
	}
	root := publicationRoot(t)
	if err := root.WriteFile("record", []byte("retained"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".", "record"} {
		t.Run(name, func(t *testing.T) {
			file, err := root.Open(name)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = file.Close() }()
			var native struct {
				volume uint64
				id     [16]byte
			}
			if err := windows.GetFileInformationByHandleEx(windows.Handle(file.Fd()), windows.FileIdInfo,
				(*byte)(unsafe.Pointer(&native)), uint32(unsafe.Sizeof(native))); err != nil {
				t.Fatal(err)
			}
			var raw [nativeFileIDBytes]byte
			binary.LittleEndian.PutUint64(raw[:8], native.volume)
			copy(raw[8:], native.id[:])
			want := "windows:" + hex.EncodeToString(raw[:])
			got, err := Identity(file)
			if err != nil || got != want {
				t.Fatalf("native identity = %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestIdentityWindowsReportsNativeFailure(t *testing.T) {
	root := publicationRoot(t)
	file, err := root.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := Identity(file)
	pathErr, ok := errors.AsType[*os.PathError](err)
	if got != "" || !ok || pathErr.Op != "query native file identity" || pathErr.Path != file.Name() {
		t.Fatalf("closed handle identity = %q, %v", got, err)
	}
}
