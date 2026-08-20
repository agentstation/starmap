package catalogs

import (
	"testing"
	"testing/fstest"
)

func TestWithFSDoesNotSelectAWritePath(t *testing.T) {
	builder, err := New(WithFS(fstest.MapFS{}))
	if err != nil {
		t.Fatalf("New WithFS: %v", err)
	}
	if got := builder.config.resolveWritePath(""); got != "" {
		t.Fatalf("WithFS write path = %q, want empty", got)
	}
}
