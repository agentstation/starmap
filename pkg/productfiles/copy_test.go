package productfiles_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/productfiles"
)

func TestPrivateFileCopyStreamsVerifiedBytes(t *testing.T) {
	directory, err := productfiles.NewDirectory(filepath.Join(t.TempDir(), "private"))
	if err != nil {
		t.Fatal(err)
	}
	expected := bytes.Repeat([]byte("bounded streaming content\n"), 10000)
	if err := directory.CompareAndPublish(t.Context(), "模型.json", nil, expected); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	n, err := directory.CopyFile(t.Context(), "模型.json", &output, int64(len(expected)))
	if err != nil || n != int64(len(expected)) || !bytes.Equal(output.Bytes(), expected) {
		t.Fatalf("copy: %d, %v", n, err)
	}
}

type privateCopyWriter func([]byte) (int, error)

func (write privateCopyWriter) Write(body []byte) (int, error) { return write(body) }

func TestPrivateFileCopyBoundsAndFailures(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	directory := newDirectory(t, root)
	body := bytes.Repeat([]byte("x"), 128<<10)
	if err := directory.CompareAndPublish(t.Context(), "record", nil, body); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../record", "nested/record", "", ".", "record."} {
		if _, err := directory.CopyFile(t.Context(), name, io.Discard, int64(len(body))); err == nil {
			t.Fatalf("accepted name %q", name)
		}
	}
	for _, limit := range []int64{-1, 0, int64(len(body) - 1), math.MaxInt64} {
		writes := 0
		_, err := directory.CopyFile(t.Context(), "record", privateCopyWriter(func(p []byte) (int, error) { writes++; return len(p), nil }), limit)
		if err == nil || writes != 0 {
			t.Fatalf("bound %d: writes=%d, err=%v", limit, writes, err)
		}
	}
	if _, err := directory.CopyFile(t.Context(), "record", nil, int64(len(body))); err == nil {
		t.Fatal("accepted nil writer")
	}
	if _, err := directory.CopyFile(nil, "record", io.Discard, int64(len(body))); err == nil {
		t.Fatal("accepted nil context")
	}
	var zero productfiles.Directory
	if _, err := zero.CopyFile(t.Context(), "record", io.Discard, int64(len(body))); err == nil {
		t.Fatal("accepted zero directory")
	}
	expected := errors.New("writer stopped")
	n, err := directory.CopyFile(t.Context(), "record", privateCopyWriter(func([]byte) (int, error) { return 7, expected }), int64(len(body)))
	if n != 7 || !errors.Is(err, expected) {
		t.Fatalf("partial writer: %d, %v", n, err)
	}
	if err := directory.CompareAndPublish(t.Context(), "empty", nil, []byte{}); err != nil {
		t.Fatal(err)
	}
	writes := 0
	n, err = directory.CopyFile(t.Context(), "empty", privateCopyWriter(func(p []byte) (int, error) { writes++; return len(p), nil }), 0)
	if err != nil || n != 0 || writes != 0 {
		t.Fatalf("empty: %d, %v, %d writes", n, err, writes)
	}
}

func TestPrivateFileCopyCancellationAndChanges(t *testing.T) {
	for _, mode := range []string{"canceled", "cancel-after-write", "file-size"} {
		t.Run(mode, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "private")
			directory := newDirectory(t, root)
			body := bytes.Repeat([]byte("x"), 128<<10)
			if err := directory.CompareAndPublish(t.Context(), "record", nil, body); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			writes := 0
			n, err := directory.CopyFile(ctx, "record", privateCopyWriter(func(p []byte) (int, error) {
				writes++
				if writes == 1 {
					switch mode {
					case "cancel-after-write":
						cancel()
					case "file-size":
						if err := os.WriteFile(filepath.Join(root, "record"), []byte("changed"), 0o600); err != nil {
							t.Fatal(err)
						}
					}
				}
				return len(p), nil
			}), int64(len(body)))
			if err == nil {
				t.Fatal("accepted changed or canceled copy")
			}
			if mode == "canceled" && (n != 0 || writes != 0) {
				t.Fatal("canceled copy wrote output")
			}
			if strings.HasPrefix(mode, "cancel") && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation: %v", err)
			}
		})
	}
}

func TestPrivateFileCopyUsesBoundedChunks(t *testing.T) {
	directory := newDirectory(t, filepath.Join(t.TempDir(), "private"))
	body := bytes.Repeat([]byte("x"), 1<<20)
	if err := directory.CompareAndPublish(t.Context(), "record", nil, body); err != nil {
		t.Fatal(err)
	}
	writes, maximum := 0, 0
	n, err := directory.CopyFile(t.Context(), "record", privateCopyWriter(func(p []byte) (int, error) { writes++; maximum = max(maximum, len(p)); return len(p), nil }), int64(len(body)))
	if err != nil || n != int64(len(body)) || writes < 2 || maximum > 32<<10 {
		t.Fatalf("stream: bytes=%d writes=%d chunk=%d err=%v", n, writes, maximum, err)
	}
}
