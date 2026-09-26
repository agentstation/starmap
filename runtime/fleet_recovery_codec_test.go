package runtime

import (
	"bytes"
	"context"
	"testing"
)

func TestFleetRecoveryCodecBoundsAndIntegrity(t *testing.T) {
	data := bytes.Repeat([]byte("retained baseline and acquisition evidence\n"), 4096)
	encoded, err := compressFleetRecovery(data)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := compressFleetRecovery(data)
	if err != nil || !bytes.Equal(encoded, repeated) {
		t.Fatal("encoding is not deterministic", err)
	}
	decoded, err := decompressFleetRecovery(t.Context(), encoded, int64(len(data)))
	if err != nil || !bytes.Equal(data, decoded) {
		t.Fatal("codec changed recovery bytes", err)
	}
	damaged := bytes.Clone(encoded)
	damaged[len(damaged)-1] ^= 1
	for _, scenario := range []struct {
		name    string
		encoded []byte
		limit   int64
	}{
		{"expansion", encoded, int64(len(data) - 1)},
		{"truncated", encoded[:len(encoded)-1], int64(len(data))},
		{"checksum", damaged, int64(len(data))},
		{"extra-member", append(bytes.Clone(encoded), encoded...), int64(len(data))},
		{"trailing", append(bytes.Clone(encoded), 'x'), int64(len(data))},
		{"plaintext", data, int64(len(data))},
		{"empty", nil, int64(len(data))},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if _, err := decompressFleetRecovery(t.Context(), scenario.encoded, scenario.limit); err == nil {
				t.Fatal("accepted invalid recovery encoding")
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := decompressFleetRecovery(ctx, encoded, int64(len(data))); err != context.Canceled {
		t.Fatal("ignored cancellation", err)
	}
}
