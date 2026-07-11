// Package embed provides utilities for working with the embedded filesystem.
package embed

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/embedded"
)

// GetEmbeddedFS returns the embedded filesystem.
func GetEmbeddedFS() fs.FS {
	return embedded.FS
}

// NormalizePath normalizes an embedded filesystem path.
func NormalizePath(p string) string {
	if p == "" {
		return "."
	}

	// Remove leading slash - embedded paths are relative
	p = strings.TrimPrefix(p, "/")

	// Convert to forward slashes (embedded uses path, not filepath)
	p = path.Clean(p)

	// Empty or just "." means root
	if p == "" || p == "." {
		return "."
	}

	return p
}

// FormatBytes formats byte count as human-readable size.
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB",
		float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatTime formats time in a standard way.
func FormatTime(t time.Time) string {
	return t.Format("Jan 2 15:04")
}

// FileInfo extracts info from fs.DirEntry and fs.FileInfo.
type FileInfo struct {
	Name    string
	Size    int64
	Mode    fs.FileMode
	ModTime time.Time
	IsDir   bool
	Path    string
}

// GetFileInfoFromEntry gets file info from a directory entry.
func GetFileInfoFromEntry(entry fs.DirEntry, fullPath string, _ fs.FS) (*FileInfo, error) {
	info, err := entry.Info()
	if err != nil {
		// Fallback for basic info
		return &FileInfo{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
			Path:  fullPath,
		}, nil
	}

	return &FileInfo{
		Name:    entry.Name(),
		Size:    info.Size(),
		Mode:    info.Mode(),
		ModTime: info.ModTime(),
		IsDir:   entry.IsDir(),
		Path:    fullPath,
	}, nil
}

// GetFileInfoFromPath gets file info from a path.
func GetFileInfoFromPath(fsys fs.FS, fullPath string) (*FileInfo, error) {
	info, err := fs.Stat(fsys, fullPath)
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		Name:    path.Base(fullPath),
		Size:    info.Size(),
		Mode:    info.Mode(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
		Path:    fullPath,
	}, nil
}

// FormatMode formats file mode similar to ls -l.
func FormatMode(mode fs.FileMode) string {
	var buf [10]byte

	// File type
	switch {
	case mode.IsDir():
		buf[0] = 'd'
	case mode.IsRegular():
		buf[0] = '-'
	default:
		buf[0] = '?'
	}

	// Owner permissions
	buf[1] = 'r' // Embedded files are always readable
	buf[2] = '-' // Not writable
	buf[3] = '-' // Not executable

	// Group permissions
	buf[4] = 'r' // Readable
	buf[5] = '-' // Not writable
	buf[6] = '-' // Not executable

	// Other permissions
	buf[7] = 'r' // Readable
	buf[8] = '-' // Not writable
	buf[9] = '-' // Not executable

	return string(buf[:])
}

// IsHidden checks if a file is hidden (starts with .).
func IsHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}
