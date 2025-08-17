package embedded

import (
	"embed"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/internal/catalogs/base"
	embeddedCatalog "github.com/agentstation/starmap/internal/embedded"
)

// Load loads the catalog from YAML files in the embedded catalog directory structure
func (c *catalog) Load() error {
	// Create a custom reader that uses the embedded FS with "catalog" prefix
	embeddedReader := &embeddedFSReader{fs: embeddedCatalog.FS, prefix: "catalog"}
	loader := base.NewLoader(embeddedReader, &c.BaseCatalog)
	return loader.Load()
}

// embeddedFSReader wraps the embedded FS with a path prefix
type embeddedFSReader struct {
	fs     embed.FS
	prefix string
}

func (e *embeddedFSReader) ReadFile(path string) ([]byte, error) {
	fullPath := filepath.Join(e.prefix, path)
	return e.fs.ReadFile(fullPath)
}

func (e *embeddedFSReader) WalkDir(root string, fn base.WalkFunc) error {
	fullRoot := filepath.Join(e.prefix, root)
	return fs.WalkDir(e.fs, fullRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Check if it's a "file does not exist" error and handle gracefully
			if strings.Contains(err.Error(), "file does not exist") {
				return fn(path, nil, err)
			}
			return err
		}

		// Skip directories and non-YAML files
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		// Remove prefix from path for consistency
		relPath := strings.TrimPrefix(path, e.prefix+"/")

		data, err := e.fs.ReadFile(path)
		return fn(relPath, data, err)
	})
}

// LoadFromPath loads the catalog from YAML files in the given filesystem path
func (c *catalog) LoadFromPath(basePath string) error {
	reader := &base.FilesystemFileReader{BasePath: basePath}
	loader := base.NewLoader(reader, &c.BaseCatalog)
	return loader.Load()
}
