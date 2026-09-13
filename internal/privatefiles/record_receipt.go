package privatefiles

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"

	"github.com/agentstation/starmap/internal/filepublish"
)

const publicationRecordMaxBytes = 64 << 20

type publicationEntry struct {
	Identity string `json:"identity"`
	Access   string `json:"access"`
}

type publicationRecord struct {
	Entry    publicationEntry `json:"entry"`
	Mode     uint32           `json:"mode"`
	Size     int64            `json:"size"`
	Modified int64            `json:"modified"`
	Digest   string           `json:"digest"`
}

func publicationEntryOf(root *os.Root, name string, directory bool) (publicationEntry, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return publicationEntry{}, err
	}
	if info.IsDir() != directory || (!directory && !info.Mode().IsRegular()) || info.Mode()&fs.ModeSymlink != 0 {
		return publicationEntry{}, changed(name)
	}
	if err := ValidateMetadata(info, "private.publication"); err != nil {
		return publicationEntry{}, err
	}
	if err := ValidateACL(root, name, info, "private.publication"); err != nil {
		return publicationEntry{}, err
	}
	file, err := openPublicationEntry(root, name, info)
	if err != nil {
		return publicationEntry{}, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return publicationEntry{}, err
	}
	if !os.SameFile(info, opened) {
		return publicationEntry{}, changed(name)
	}
	identity, err := filepublish.Identity(file)
	if err != nil {
		return publicationEntry{}, err
	}
	access, err := publicationAccess(file)
	if err != nil {
		return publicationEntry{}, err
	}
	after, err := root.Lstat(name)
	if err != nil {
		return publicationEntry{}, err
	}
	if !os.SameFile(info, after) || info.Mode() != after.Mode() {
		return publicationEntry{}, changed(name)
	}
	return publicationEntry{Identity: identity, Access: access}, nil
}

func publicationRecordOf(root *os.Root, name string, limit int64) (publicationRecord, error) {
	entry, err := publicationEntryOf(root, name, false)
	if err != nil {
		return publicationRecord{}, err
	}
	before, err := recordInfo(root, name)
	if err != nil {
		return publicationRecord{}, err
	}
	data, err := ReadFile(root, name, limit)
	if err != nil {
		return publicationRecord{}, err
	}
	after, err := recordInfo(root, name)
	if err != nil {
		return publicationRecord{}, err
	}
	current, err := publicationEntryOf(root, name, false)
	if err != nil {
		return publicationRecord{}, err
	}
	if !sameRecord(before, after) || current != entry {
		return publicationRecord{}, changed(name)
	}
	return publicationRecord{Entry: entry, Mode: uint32(after.Mode()), Size: int64(len(data)), Modified: after.ModTime().UnixNano(), Digest: publicationDigest(data)}, nil
}

func publicationDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func checkPublicationEntry(root *os.Root, name string, directory bool, want publicationEntry) error {
	current, err := publicationEntryOf(root, name, directory)
	if err != nil {
		return err
	}
	if current != want {
		return changed(name)
	}
	return nil
}

func checkPublicationRecord(root *os.Root, name string, want publicationRecord) error {
	current, err := publicationRecordOf(root, name, publicationRecordMaxBytes)
	if err != nil {
		return err
	}
	if current != want {
		return changed(name)
	}
	return nil
}
