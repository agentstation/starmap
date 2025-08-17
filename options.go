package starmap

import "fmt"

// Configuration and options for each catalog type

// Embedded catalog options
type embeddedConfig struct {
	AutoLoad bool
}

type EmbeddedOption func(*embeddedConfig) error

func WithEmbeddedAutoLoad(enabled bool) EmbeddedOption {
	return func(cfg *embeddedConfig) error {
		cfg.AutoLoad = enabled
		return nil
	}
}

func WithEmbeddedNoAutoLoad() EmbeddedOption {
	return WithEmbeddedAutoLoad(false)
}

// Files catalog options
type filesConfig struct {
	Path     string
	AutoLoad bool
	ReadOnly bool
}

type FilesOption func(*filesConfig) error

func WithFilesAutoLoad(enabled bool) FilesOption {
	return func(cfg *filesConfig) error {
		cfg.AutoLoad = enabled
		return nil
	}
}

func WithFilesReadOnly(readOnly bool) FilesOption {
	return func(cfg *filesConfig) error {
		cfg.ReadOnly = readOnly
		return nil
	}
}

func WithFilesNoAutoLoad() FilesOption {
	return WithFilesAutoLoad(false)
}

// Memory catalog options
type memoryConfig struct {
	ReadOnly    bool
	PreloadData []byte
}

type MemoryOption func(*memoryConfig) error

func WithMemoryReadOnly(readOnly bool) MemoryOption {
	return func(cfg *memoryConfig) error {
		cfg.ReadOnly = readOnly
		return nil
	}
}

func WithMemoryPreload(data []byte) MemoryOption {
	return func(cfg *memoryConfig) error {
		if len(data) == 0 {
			return fmt.Errorf("preload data cannot be empty")
		}
		cfg.PreloadData = make([]byte, len(data))
		copy(cfg.PreloadData, data)
		return nil
	}
}
