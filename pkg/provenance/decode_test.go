package provenance

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func wideProvenance(entries int) string {
	var data strings.Builder
	data.WriteString("provenance:\n")
	for index := range entries {
		fmt.Fprintf(&data, "  model:m%d:Value:\n  - value: {date: 2026-07-28, scientific: 1e-7, decimal: 0.0000001, quoted: \"true\", nested: [null, yes, 18446744073709551615]}\n    previousvalue: |\n      preserved: text\n    timestamp: 2026-07-28T18:00:00Z\n", index)
	}
	return data.String()
}

func TestDecodeYAMLPreservesWholeFileSemantics(t *testing.T) {
	wide := wideProvenance(2*provenanceBatchEntries + 1)
	for _, test := range []struct{ name, data string }{
		{"generated block map", wide},
		{"unicode key", strings.ReplaceAll(wide, "model:m", "model:模型")},
		{"comments", strings.ReplaceAll(wide, "  model:", "# boundary comment\n  model:")},
		{"duplicate across batches", wide + "  model:m0:Value: []\n"},
		{"malformed last entry", wide + "  model:broken:Value: [\n"},
		{"cross-batch alias", strings.Replace(wide, "value: {date:", "value: &shared {date:", 1) + "  model:last:Value:\n  - value: *shared\n"},
		{"additional root field", wide + "other: ignored\n"},
		{"explicit document", "---\n" + wide + "...\n"},
		{"empty", "provenance: {}\n"},
		{"null", "provenance: null\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := []byte(test.data)
			want, wantErr := decodeWholeFile(data)
			got, err := DecodeYAML(data)
			if (err == nil) != (wantErr == nil) {
				t.Fatalf("decode errors differ: got %v; want %v", err, wantErr)
			}
			if err == nil && !reflect.DeepEqual(got, want) {
				t.Fatal("batch decoding changed provenance values")
			}
		})
	}
}

func TestProvenanceBatchesUseGeneratedEntryBoundaries(t *testing.T) {
	data := []byte(wideProvenance(2*provenanceBatchEntries + 1))
	if got := len(provenanceBatchBoundaries(data)); got != 3 {
		t.Fatalf("batch count = %d, want 3", got)
	}
	anchored := strings.Replace(string(data), "value: {", "value: &shared {", 1)
	if !hasDependentYAML([]byte(anchored)) {
		t.Fatal("anchor dependency was not detected")
	}
}

func BenchmarkDecodeWideProvenance(b *testing.B) {
	data := []byte(wideProvenance(1024))
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		if _, err := DecodeYAML(data); err != nil {
			b.Fatal(err)
		}
	}
}

func FuzzDecodeYAMLMatchesWholeFile(f *testing.F) {
	f.Add(wideProvenance(provenanceBatchEntries + 1))
	f.Add("provenance: null\n")
	f.Fuzz(func(t *testing.T, input string) {
		want, wantErr := decodeWholeFile([]byte(input))
		got, err := DecodeYAML([]byte(input))
		if (err == nil) != (wantErr == nil) || (err == nil && !reflect.DeepEqual(got, want)) {
			t.Fatalf("batch decoding differs from whole-file decoding: %v / %v", err, wantErr)
		}
	})
}
