package productfiles

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCompletePublicationChecksHostBeforeExactWrite(t *testing.T) {
	d, err := NewDirectory(filepath.Join(t.TempDir(), "records"))
	if err != nil {
		t.Fatal(err)
	}
	refusal := errors.New("original receipt unavailable")
	if err := d.CompletePublication(t.Context(), "phase", nil, []byte("original"), func(context.Context) error { return refusal }); !errors.Is(err, refusal) {
		t.Fatalf("host refusal: %v", err)
	}
	if _, err := d.ReadFile("phase", 128); !os.IsNotExist(err) {
		t.Fatalf("refusal wrote destination: %v", err)
	}
	if err := d.CompletePublication(t.Context(), "phase", nil, []byte("original"), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := d.CompletePublication(t.Context(), "phase", nil, []byte("other"), func(context.Context) error { return nil }); err == nil {
		t.Fatal("changed value accepted")
	}
	if err := d.CompletePublication(t.Context(), "phase", nil, []byte("original"), nil); err == nil {
		t.Fatal("missing host check accepted")
	}
	if err := d.CompletePublication(nil, "phase", nil, []byte("original"), func(context.Context) error { return nil }); err == nil {
		t.Fatal("nil context accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := d.CompletePublication(ctx, "phase", nil, []byte("original"), func(context.Context) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	var zero Directory
	if err := zero.CompletePublication(t.Context(), "phase", nil, []byte("original"), func(context.Context) error { return nil }); err == nil {
		t.Fatal("zero directory accepted")
	}
}

func TestInspectPublicationIsPassiveWithoutWriterMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records")
	d, err := NewDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	if stages, err := d.InspectPublication(t.Context(), "phase", nil, []byte("original")); err != nil || len(stages) != 0 {
		t.Fatalf("passive absent: %v %v", stages, err)
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		t.Fatalf("inspection created metadata: %v %v", entries, err)
	}
	if _, err := d.InspectPublication(nil, "phase", nil, []byte("original")); err == nil {
		t.Fatal("nil context accepted")
	}
	if _, err := d.InspectPublication(t.Context(), "../phase", nil, []byte("original")); err == nil {
		t.Fatal("foreign path accepted")
	}
}

func TestCompletePublicationDetachesHostCallbackBytes(t *testing.T) {
	d, err := NewDirectory(filepath.Join(t.TempDir(), "records"))
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("original")
	if err := d.CompletePublication(t.Context(), "phase", nil, body, func(context.Context) error { copy(body, []byte("changed!")); return nil }); err != nil {
		t.Fatal(err)
	}
	got, err := d.ReadFile("phase", 128)
	if err != nil || string(got) != "original" {
		t.Fatalf("callback changed exact phase: %q %v", got, err)
	}
}
