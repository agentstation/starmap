package workspace

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func finishInstalledReplacement(ctx context.Context, root *os.Root, record replacementRecord, hooks replacementHooks) error {
	if err := validateReplacementCatalog(ctx, root, filepath.Base(record.Target), record); err != nil {
		return err
	}
	data, err := json.Marshal(record.Marker)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	markerName := filepath.Base(projectionMarkerPath(record.Target))
	current, err := readReplacementBytes(root, markerName, replacementJournalMax)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !bytes.Equal(data, current) {
		backup, err := optionalTree(ctx, root, record.Backup)
		if err != nil {
			return err
		}
		if backup.ID != "" && !sameTree(backup, record.Old) {
			return replacementConflict(record.Target, "backup changed before the projection receipt")
		}
		if hooks.beforeMarker != nil {
			if err := hooks.beforeMarker(); err != nil {
				return err
			}
		}
		temporary := "." + markerName + "." + rand.Text()
		if err := writeReplacementBytes(root, temporary, data); err != nil {
			return err
		}
		defer func() { _ = root.Remove(temporary) }()
		if err := root.Rename(temporary, markerName); err != nil {
			return err
		}
		if err := filepublish.SyncDirectory(root); err != nil {
			return err
		}
	}
	if err := hooks.reached(replacementMarkerSaved); err != nil {
		return err
	}
	if err := cleanupReplacementBackup(ctx, root, record, hooks); err != nil {
		return err
	}
	if err := hooks.reached(replacementBackupRemoved); err != nil {
		return err
	}
	return finishReplacementRecord(root, record)
}

func validateReplacementCatalog(ctx context.Context, root *os.Root, name string, record replacementRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	child, err := root.OpenRoot(name)
	if err != nil {
		return err
	}
	defer func() { _ = child.Close() }()
	file, err := child.Open(".")
	if err != nil {
		return err
	}
	id, identityErr := entryIdentity(file)
	_ = file.Close()
	if identityErr != nil {
		return identityErr
	}
	if id != record.New.ID {
		return replacementConflict(record.Target, "installed directory identity changed")
	}
	builder, err := catalogs.New(catalogs.WithFS(child.FS()))
	if err != nil {
		return err
	}
	if err := builder.LoadReport().Err(); err != nil {
		return err
	}
	catalog, err := builder.Build()
	if err != nil {
		return err
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		return err
	}
	if catalogs.DescribeCatalogPayload(payload).Checksum != record.Marker.WorkspaceChecksum {
		return replacementConflict(record.Target, "installed catalog does not match its receipt")
	}
	endpoints, err := readReplacementBytes(child, endpointProjectionFilename, replacementMaxBytes)
	if err != nil {
		return err
	}
	if endpointProjectionChecksum(endpoints) != record.Marker.EndpointChecksum {
		return replacementConflict(record.Target, "installed endpoints do not match their receipt")
	}
	var header struct {
		SchemaVersion int    `yaml:"schema_version"`
		GenerationID  string `yaml:"generation_id"`
		CatalogDigest string `yaml:"catalog_digest"`
	}
	if err := yaml.Unmarshal(endpoints, &header); err != nil {
		return err
	}
	if header.SchemaVersion != endpointProjectionSchemaVersion || header.GenerationID != record.Marker.GenerationID || header.CatalogDigest != record.Marker.PayloadChecksum {
		return replacementConflict(record.Target, "endpoint identity does not match the journal")
	}
	return ctx.Err()
}

func readReplacementBytes(root *os.Root, name string, limit int64) ([]byte, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, invalidReplacement("file")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) {
		return nil, replacementConflict(name, "file changed before the read")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, replacementLimit("file")
	}
	return data, nil
}

func finishReplacementRecord(root *os.Root, record replacementRecord) error {
	current, err := readReplacementRecord(root, record.Target)
	if err != nil {
		return err
	}
	want, err := json.Marshal(record)
	if err != nil {
		return err
	}
	actual, err := json.Marshal(current)
	if err != nil {
		return err
	}
	if !bytes.Equal(want, actual) {
		return replacementConflict(record.Target, "journal changed before completion")
	}
	if err := root.Remove(filepath.Base(replacementJournalPath(record.Target))); err != nil {
		return err
	}
	return filepublish.SyncDirectory(root)
}
