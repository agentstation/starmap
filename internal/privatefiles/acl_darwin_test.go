package privatefiles

import (
	"encoding/binary"
	"slices"
	"testing"
)

func TestDarwinACLParser(t *testing.T) {
	var principal [16]byte
	principal[15] = 1
	fixture := func(count int) []byte {
		buffer := make([]byte, 12+44+24*count)
		binary.LittleEndian.PutUint32(buffer[:4], uint32(len(buffer)))
		binary.LittleEndian.PutUint32(buffer[4:8], 8)
		binary.LittleEndian.PutUint32(buffer[8:12], uint32(len(buffer)-12))
		binary.LittleEndian.PutUint32(buffer[12:16], darwinFileSecurityMagic)
		binary.LittleEndian.PutUint32(buffer[48:52], uint32(count))
		for index := range count {
			entry := buffer[56+index*24:]
			copy(entry[:16], principal[:])
			binary.LittleEndian.PutUint32(entry[16:20], 1)
			binary.LittleEndian.PutUint32(entry[20:24], 2)
		}
		return buffer
	}
	for _, test := range []struct {
		name    string
		count   int
		mutate  func([]byte) []byte
		want    int
		invalid bool
	}{
		{name: "owner grant", count: 1, want: 1},
		{name: "maximum entries", count: 128, want: 128},
		{name: "empty ACL"},
		{name: "no ACL marker", mutate: func(b []byte) []byte { binary.LittleEndian.PutUint32(b[48:52], 0xffffffff); return b }},
		{name: "absent security", mutate: func(b []byte) []byte {
			binary.LittleEndian.PutUint32(b[:4], 12)
			binary.LittleEndian.PutUint32(b[8:12], 0)
			return b[:12]
		}},
		{name: "deny", count: 1, mutate: func(b []byte) []byte { binary.LittleEndian.PutUint32(b[72:76], 2); return b }},
		{name: "zero rights", count: 1, mutate: func(b []byte) []byte { binary.LittleEndian.PutUint32(b[76:80], 0); return b }},
		{name: "inherited grant", count: 1, want: 1, mutate: func(b []byte) []byte { binary.LittleEndian.PutUint32(b[72:76], 1|1<<4|1<<8); return b }},
		{name: "short header", invalid: true, mutate: func(b []byte) []byte { return b[:11] }},
		{name: "truncated entry", count: 1, invalid: true, mutate: func(b []byte) []byte { return b[:len(b)-1] }},
		{name: "negative offset", invalid: true, mutate: func(b []byte) []byte { binary.LittleEndian.PutUint32(b[4:8], 0xffffffff); return b }},
		{name: "header overlap", invalid: true, mutate: func(b []byte) []byte { binary.LittleEndian.PutUint32(b[4:8], 0); return b }},
		{name: "oversized payload", invalid: true, mutate: func(b []byte) []byte { binary.LittleEndian.PutUint32(b[8:12], 0xffffffff); return b }},
		{name: "wrong magic", invalid: true, mutate: func(b []byte) []byte { b[12] = 0; return b }},
		{name: "extra entries", count: 129, invalid: true},
		{name: "wrong entry count", count: 1, invalid: true, mutate: func(b []byte) []byte { binary.LittleEndian.PutUint32(b[48:52], 0); return b }},
		{name: "unsupported entry", count: 1, invalid: true, mutate: func(b []byte) []byte { binary.LittleEndian.PutUint32(b[72:76], 3); return b }},
	} {
		t.Run(test.name, func(t *testing.T) {
			buffer := fixture(test.count)
			if test.mutate != nil {
				buffer = test.mutate(buffer)
			}
			before := slices.Clone(buffer)
			grants, err := darwinACLGrants(buffer)
			if (err != nil) != test.invalid {
				t.Fatalf("error = %v; invalid = %v", err, test.invalid)
			}
			if !slices.Equal(before, buffer) {
				t.Fatal("parser changed native metadata")
			}
			if len(grants) != test.want {
				t.Fatalf("got %d grants, want %d", len(grants), test.want)
			}
			for _, grant := range grants {
				if grant != principal {
					t.Fatal("principal changed")
				}
			}
		})
	}
}
