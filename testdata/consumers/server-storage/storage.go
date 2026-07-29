// Package consumer is an external server deployment that explicitly selects
// filesystem or S3-compatible catalog storage.
package consumer

import (
	"strings"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/agentstation/starmap/pkg/catalogstore"
	s3store "github.com/agentstation/starmap/pkg/catalogstore/s3"
	"github.com/agentstation/starmap/pkg/errors"
)

// StorageMode selects exactly one durable generation-store implementation.
type StorageMode string

const (
	// StorageFilesystem selects Starmap's standalone filesystem store.
	StorageFilesystem StorageMode = "filesystem"
	// StorageObject selects Starmap's S3-compatible object backend.
	StorageObject StorageMode = "object"
)

// StorageConfig is deployment-owned configuration. Starmap does not discover
// credentials, construct an S3 client, or own either backend's lifecycle.
type StorageConfig struct {
	Mode StorageMode

	FilesystemPath string

	S3Client     *awss3.Client
	S3Bucket     string
	ObjectPrefix string
}

// Open validates the selected mode and constructs an inert store. Filesystem
// directories and object-network requests begin only when the returned store is
// used.
func (c StorageConfig) Open() (catalogstore.Store, error) {
	switch c.Mode {
	case StorageFilesystem:
		if c.S3Client != nil || strings.TrimSpace(c.S3Bucket) != "" ||
			strings.TrimSpace(c.ObjectPrefix) != "" {
			return nil, &errors.ConfigError{
				Component: "server catalog storage",
				Message:   "object fields are invalid in filesystem mode",
			}
		}
		return catalogstore.NewFilesystem(c.FilesystemPath)
	case StorageObject:
		if strings.TrimSpace(c.FilesystemPath) != "" {
			return nil, &errors.ConfigError{
				Component: "server catalog storage",
				Message:   "filesystem path is invalid in object mode",
			}
		}
		backend, err := s3store.New(c.S3Client, s3store.Config{Bucket: c.S3Bucket})
		if err != nil {
			return nil, err
		}
		return catalogstore.NewObject(backend, c.ObjectPrefix)
	default:
		return nil, &errors.ConfigError{
			Component: "server catalog storage",
			Message:   "mode must be filesystem or object",
		}
	}
}
