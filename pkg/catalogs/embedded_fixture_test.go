package catalogs

import (
	"os"
)

func newEmbeddedTestBuilder() (*Builder, error) {
	return New(WithFS(os.DirFS("../../internal/embedded/catalog")))
}
