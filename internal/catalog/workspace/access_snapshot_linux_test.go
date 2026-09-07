package workspace

import (
	"encoding/binary"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestTreeSnapshotDetectsNativeACLChange(t *testing.T) {
	for _, attribute := range []string{"system.posix_acl_access", "system.posix_acl_default"} {
		t.Run(attribute, func(t *testing.T) {
			path := t.TempDir()
			if err := os.Chmod(path, 0o700); err != nil {
				t.Fatal(err)
			}
			before, err := snapshotTree(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			// Version 2 uses eight-byte entries: tag, permissions, and qualifier.
			data := make([]byte, 4+5*8)
			binary.LittleEndian.PutUint32(data, 2)
			for index, entry := range [][3]uint32{{1, 7, 0xffffffff}, {2, 4, 65534}, {4, 0, 0xffffffff}, {16, 0, 0xffffffff}, {32, 0, 0xffffffff}} {
				offset := 4 + index*8
				binary.LittleEndian.PutUint16(data[offset:], uint16(entry[0]))
				binary.LittleEndian.PutUint16(data[offset+2:], uint16(entry[1]))
				binary.LittleEndian.PutUint32(data[offset+4:], entry[2])
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = file.Close() }()
			if err := unix.Fsetxattr(int(file.Fd()), attribute, data, 0); err != nil {
				t.Fatal("set native ACL", err)
			}
			after, err := snapshotTree(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			if before.Entries[0].Mode != after.Entries[0].Mode {
				t.Fatal("fixture changed mode bits")
			}
			if sameTree(before, after) {
				t.Fatal("snapshot omitted a native ACL change")
			}
		})
	}
}
